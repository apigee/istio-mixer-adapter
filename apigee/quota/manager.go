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
	"github.com/apigee/istio-mixer-adapter/apigee/product"
	"istio.io/istio/mixer/pkg/adapter"
)

// todo: support args.DeduplicationID

const (
	quotaPath             = "/quotas/organization/%s/environment/%s"
	defaultPollPeriod     = time.Second
	defaultNumSyncWorkers = 10
	defaultDeleteAfter    = 10 * time.Minute
	httpTimeout           = 15 * time.Second
	syncQueueSize         = 100
)

// A Manager tracks multiple Apigee quotas
type Manager struct {
	baseURL           url.URL
	close             chan bool
	client            *http.Client
	now               func() time.Time
	log               adapter.Logger
	syncLoopPollEvery time.Duration
	bucketsLock       sync.RWMutex
	buckets           map[string]*bucket // Map from ID -> bucket
	syncQueue         chan *bucket
	numSyncWorkers    int
}

// NewManager constructs and starts a new Manager. Call Close when done.
func NewManager(baseURL url.URL, env adapter.Env) *Manager {
	m := newManager(baseURL)
	m.Start(env)
	return m
}

// newManager constructs a new Manager
func newManager(baseURL url.URL) *Manager {
	return &Manager{
		close: make(chan bool),
		client: &http.Client{
			Timeout: httpTimeout,
		},
		now:               time.Now,
		syncLoopPollEvery: defaultPollPeriod,
		buckets:           map[string]*bucket{},
		syncQueue:         make(chan *bucket, syncQueueSize),
		baseURL:           baseURL,
		numSyncWorkers:    defaultNumSyncWorkers,
	}
}

// Start starts the manager.
func (m *Manager) Start(env adapter.Env) {
	m.log = env.Logger()
	m.log.Infof("starting quota manager")

	env.ScheduleDaemon(func() {
		m.idleSyncLoop()
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
}

// Apply a quota request to the local quota bucket and schedule for sync
func (m *Manager) Apply(auth auth.Context, p product.APIProduct, args adapter.QuotaArgs) Result {
	quotaID := fmt.Sprintf("%s-%s", auth.Application, p.Name)

	req := request{
		Identifier: quotaID,
		Weight:     args.QuotaAmount,
		Interval:   p.QuotaIntervalInt,
		Allow:      p.QuotaLimitInt,
		TimeUnit:   p.QuotaTimeUnit,
	}

	m.bucketsLock.Lock()
	b := m.buckets[quotaID]
	if b == nil {
		b = &bucket{
			org:         auth.Context.Organization(),
			env:         auth.Context.Environment(),
			id:          quotaID,
			requests:    []*request{},
			result:      nil,
			created:     m.now(),
			lock:        sync.RWMutex{},
			now:         m.now,
			deleteAfter: defaultDeleteAfter,
		}
		m.buckets[quotaID] = b
	}
	m.bucketsLock.Unlock()

	return b.apply(m, &req)
}

// loop that deletes old buckets and syncs active buckets
func (m *Manager) idleSyncLoop() {
	t := time.NewTicker(m.syncLoopPollEvery)
	for {
		select {
		case <-t.C:
			m.bucketsLock.Lock()
			for id, b := range m.buckets {
				if b.okToDelete() {
					delete(m.buckets, id)
				} else if b.needsUpdate() {
					m.syncQueue <- b
				}
			}
			m.bucketsLock.Unlock()
		case <-m.close:
			m.log.Infof("closing quota sync loop")
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
			// todo: debounce?
			bucket.sync(m)
		case <-m.close:
			m.log.Infof("closing quota sync worker")
			return
		}
	}
}
