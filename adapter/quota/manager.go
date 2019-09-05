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

	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"github.com/apigee/istio-mixer-adapter/adapter/product"
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
	log                adapter.Logger
	syncRate           time.Duration
	bucketsLock        sync.RWMutex
	buckets            map[string]*bucket // Map from ID -> bucket
	syncQueue          chan *bucket
	numSyncWorkers     int
	dupCache           ResultCache
	syncingBuckets     map[*bucket]struct{}
	syncingBucketsLock sync.Mutex
}

// NewManager constructs and starts a new Manager. Call Close when done.
func NewManager(env adapter.Env, options Options) (*Manager, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}
	m := newManager(options.BaseURL, options.Client)
	m.Start(env)
	return m, nil
}

// newManager constructs a new Manager
func newManager(baseURL *url.URL, client *http.Client) *Manager {
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
	}
}

// Start starts the manager.
func (m *Manager) Start(env adapter.Env) {
	m.log = env.Logger()
	m.log.Infof("starting quota manager")

	env.ScheduleDaemon(func() {
		m.syncLoop()
	})

	for i := 0; i < m.numSyncWorkers; i++ {
		env.ScheduleDaemon(func() {
			m.syncBucketWorker()
		})
	}
	m.log.Infof("started quota manager with %d workers", m.numSyncWorkers)
}

// Close shuts down the manager.
func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.log.Infof("closing quota manager")
	m.close <- true
	close(m.syncQueue)
	for i := 0; i <= m.numSyncWorkers; i++ {
		<-m.closed
	}
	m.log.Infof("closed quota manager")
}

// Apply a quota request to the local quota bucket and schedule for sync
func (m *Manager) Apply(auth *auth.Context, p *product.APIProduct, args adapter.QuotaArgs) (*Result, error) {

	if result := m.dupCache.Get(args.DeduplicationID); result != nil {
		return result, nil
	}

	quotaID := fmt.Sprintf("%s-%s", auth.Application, p.Name)

	req := &Request{
		Identifier: quotaID,
		Weight:     args.QuotaAmount,
		Interval:   p.QuotaIntervalInt,
		Allow:      p.QuotaLimitInt,
		TimeUnit:   p.QuotaTimeUnit,
	}

	// a new bucket is created if missing or if product is no longer compatible
	var result *Result
	var err error
	forceSync := false
	m.bucketsLock.RLock()
	b, ok := m.buckets[quotaID]
	m.bucketsLock.RUnlock()
	if !ok || !b.compatible(req) {
		m.bucketsLock.Lock()
		b, ok = m.buckets[quotaID]
		if !ok || !b.compatible(req) {
			forceSync = true
			b = newBucket(*req, m)
			m.syncingBucketsLock.Lock()
			m.syncingBuckets[b] = struct{}{}
			m.syncingBucketsLock.Unlock()
			defer func() {
				m.syncingBucketsLock.Lock()
				delete(m.syncingBuckets, b)
				m.syncingBucketsLock.Unlock()
			}()
			m.buckets[quotaID] = b
			m.log.Debugf("new quota bucket: %s", quotaID)
		}
		m.bucketsLock.Unlock()
	}

	if forceSync {
		err = b.sync() // force sync for new bucket
		result = b.result
	} else {
		result, err = b.apply(req)
	}

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
				m.log.Debugf("deleting quota buckets: %v", deleteIDs)
				m.bucketsLock.Lock()
				for _, id := range deleteIDs {
					delete(m.buckets, id)
				}
				m.bucketsLock.Unlock()
			}
		case <-m.close:
			m.log.Debugf("closing quota sync loop")
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
			m.log.Debugf("closing quota sync worker")
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
}

func (o *Options) validate() error {
	if o.Client == nil ||
		o.BaseURL == nil {
		return fmt.Errorf("all quota options are required")
	}
	return nil
}
