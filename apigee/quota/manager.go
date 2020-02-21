// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package quota

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/log"
	"github.com/apigee/istio-mixer-adapter/apigee/product"
	"istio.io/istio/mixer/pkg/adapter"
)

const (
	quotaPath             = "/quotas"
	defaultSyncRate       = time.Second
	defaultNumSyncWorkers = 10
	defaultRefreshAfter   = 1 * time.Minute
	defaultDeleteAfter    = 10 * time.Minute
	syncQueueSize         = 100
	resultCacheBufferSize = 30
)

// A Manager tracks multiple Apigee quotas
type Manager struct {
	baseURL            *url.URL
	close              chan bool
	closed             chan bool
	client             *http.Client
	now                func() time.Time
	syncRate           time.Duration
	bucketsLock        sync.RWMutex
	buckets            map[string]*bucket // Map from ID -> bucket
	syncQueue          chan *bucket
	numSyncWorkers     int
	dupCache           ResultCache
	syncingBuckets     map[*bucket]struct{}
	syncingBucketsLock sync.Mutex
	key                string
	secret             string
}

// NewManager constructs and starts a new Manager. Call Close when done.
func NewManager(options Options) (*Manager, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}
	m := newManager(options.BaseURL, options.Client, options.Key, options.Secret)
	m.Start()
	return m, nil
}

// newManager constructs a new Manager
func newManager(baseURL *url.URL, client *http.Client, key, secret string) *Manager {
	return &Manager{
		close:          make(chan bool),
		closed:         make(chan bool),
		client:         client,
		now:            time.Now,
		syncRate:       defaultSyncRate,
		buckets:        map[string]*bucket{},
		syncQueue:      make(chan *bucket, syncQueueSize),
		baseURL:        baseURL,
		numSyncWorkers: defaultNumSyncWorkers,
		dupCache:       ResultCache{size: resultCacheBufferSize},
		syncingBuckets: map[*bucket]struct{}{},
		key:            key,
		secret:         secret,
	}
}

// Start starts the manager.
func (m *Manager) Start() {
	log.Infof("starting quota manager")

	go m.syncLoop()

	for i := 0; i < m.numSyncWorkers; i++ {
		go m.syncBucketWorker()
	}
	log.Infof("started quota manager with %d workers", m.numSyncWorkers)
}

// Close shuts down the manager.
func (m *Manager) Close() {
	if m == nil {
		return
	}
	log.Infof("closing quota manager")
	m.close <- true
	close(m.syncQueue)
	for i := 0; i <= m.numSyncWorkers; i++ {
		<-m.closed
	}
	log.Infof("closed quota manager")
}

func getQuotaID(auth *auth.Context, p *product.APIProduct) string {
	return fmt.Sprintf("%s-%s", auth.Application, p.Name)
}

// Apply a quota request to the local quota bucket and schedule for sync
func (m *Manager) Apply(auth *auth.Context, p *product.APIProduct, args adapter.QuotaArgs) (*Result, error) {

	if result := m.dupCache.Get(args.DeduplicationID); result != nil {
		return result, nil
	}

	quotaID := getQuotaID(auth, p)

	req := &Request{
		Identifier: quotaID,
		Interval:   p.QuotaIntervalInt,
		Allow:      p.QuotaLimitInt,
		TimeUnit:   p.QuotaTimeUnit,
	}

	// a new bucket is created if missing or if product is no longer compatible
	var result *Result
	var err error
	m.bucketsLock.RLock()
	b, ok := m.buckets[quotaID]
	m.bucketsLock.RUnlock()
	if !ok || !b.compatible(req) {
		m.bucketsLock.Lock()
		b, ok = m.buckets[quotaID]
		if !ok || !b.compatible(req) {
			b = newBucket(*req, m)
			m.buckets[quotaID] = b
			log.Debugf("new quota bucket: %s", quotaID)
		}
		m.bucketsLock.Unlock()
	}

	req.Weight = args.QuotaAmount
	result, err = b.apply(req)

	if result != nil && err == nil && args.DeduplicationID != "" {
		m.dupCache.Add(args.DeduplicationID, result)
	}

	return result, err
}

// loop to sync active buckets and deletes old buckets
func (m *Manager) syncLoop() {
	t := time.NewTicker(m.syncRate)
	for {
		select {
		case <-t.C:
			var deleteIDs []string
			m.bucketsLock.RLock()
			for id, b := range m.buckets {
				if b.needToDelete() {
					deleteIDs = append(deleteIDs, id)
				} else if b.needToSync() {
					bucket := b
					m.syncQueue <- bucket
				}
			}
			m.bucketsLock.RUnlock()
			if deleteIDs != nil {
				log.Debugf("deleting quota buckets: %v", deleteIDs)
				m.bucketsLock.Lock()
				for _, id := range deleteIDs {
					delete(m.buckets, id)
				}
				m.bucketsLock.Unlock()
			}
		case <-m.close:
			log.Debugf("closing quota sync loop")
			t.Stop()
			m.closed <- true
			return
		}
	}
}

// worker routine for syncing a bucket with the server
func (m *Manager) syncBucketWorker() {
	for {
		bucket, ok := <-m.syncQueue
		if ok {
			m.syncingBucketsLock.Lock()
			if _, ok := m.syncingBuckets[bucket]; !ok {
				m.syncingBuckets[bucket] = struct{}{}
				m.syncingBucketsLock.Unlock()
				bucket.sync()
				m.syncingBucketsLock.Lock()
				delete(m.syncingBuckets, bucket)
			}
			m.syncingBucketsLock.Unlock()
		} else {
			log.Debugf("closing quota sync worker")
			m.closed <- true
			return
		}
	}
}

// Options allows us to specify options for how this auth manager will run
type Options struct {
	// Client is a configured HTTPClient
	Client *http.Client
	// BaseURL of the Apigee internal proxy
	BaseURL *url.URL
	// Key is provisioning key
	Key string
	// Secret is provisioning secret
	Secret string
}

func (o *Options) validate() error {
	if o.Client == nil ||
		o.BaseURL == nil ||
		o.Key == "" ||
		o.Secret == "" {
		return fmt.Errorf("all quota options are required")
	}
	return nil
}
