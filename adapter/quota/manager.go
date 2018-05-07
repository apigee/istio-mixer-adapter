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
	quotaPath             = "/quotas/organization/%s/environment/%s"
	defaultSyncRate       = time.Second
	defaultNumSyncWorkers = 10
	defaultRefreshAfter   = 1 * time.Minute
	defaultDeleteAfter    = 10 * time.Minute
	httpTimeout           = 15 * time.Second
	syncQueueSize         = 100
)

// A Manager tracks multiple Apigee quotas
type Manager struct {
	baseURL        *url.URL
	close          chan bool
	client         *http.Client
	now            func() time.Time
	log            adapter.Logger
	syncRate       time.Duration
	bucketsLock    sync.RWMutex
	buckets        map[string]*bucket // Map from ID -> bucket
	syncQueue      chan *bucket
	numSyncWorkers int
}

// NewManager constructs and starts a new Manager. Call Close when done.
func NewManager(baseURL *url.URL, env adapter.Env) *Manager {
	m := newManager(baseURL)
	m.Start(env)
	return m
}

// newManager constructs a new Manager
func newManager(baseURL *url.URL) *Manager {
	return &Manager{
		close: make(chan bool),
		client: &http.Client{
			Timeout: httpTimeout,
		},
		now:            time.Now,
		syncRate:       defaultSyncRate,
		buckets:        map[string]*bucket{},
		syncQueue:      make(chan *bucket, syncQueueSize),
		baseURL:        baseURL,
		numSyncWorkers: defaultNumSyncWorkers,
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
	for i := 0; i < m.numSyncWorkers; i++ {
		m.close <- true
	}
	m.log.Infof("closed quota manager")
}

// Apply a quota request to the local quota bucket and schedule for sync
func (m *Manager) Apply(auth *auth.Context, p *product.APIProduct, args adapter.QuotaArgs) (*Result, error) {
	quotaID := fmt.Sprintf("%s-%s", auth.Application, p.Name)

	req := &Request{
		Identifier:      quotaID,
		Weight:          args.QuotaAmount,
		Interval:        p.QuotaIntervalInt,
		Allow:           p.QuotaLimitInt,
		TimeUnit:        p.QuotaTimeUnit,
		DeduplicationID: args.DeduplicationID,
	}

	m.bucketsLock.RLock()
	b, existingBucket := m.buckets[quotaID]
	if !existingBucket {
		b = newBucket(req, m, auth)
		m.buckets[quotaID] = b
	}
	m.bucketsLock.RUnlock()
	if !existingBucket {
		b.sync(m) // force sync for new bucket
	}

	return b.apply(m, req)
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
			return
		}
	}
}

// worker routine for syncing a bucket with the server
func (m *Manager) syncBucketWorker() {
	for {
		select {
		case bucket := <-m.syncQueue:
			bucket.sync(m)
		case <-m.close:
			m.log.Debugf("closing quota sync worker")
			return
		}
	}
}
