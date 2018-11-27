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

package analytics

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"istio.io/istio/mixer/pkg/adapter"
)

const (
	analyticsPath = "/analytics/organization/%s/environment/%s"
	axRecordType  = "APIAnalytics"
	pathFmt       = "date=%s/time=%s/"
	bufferMode    = os.FileMode(0700)
	tempDir       = "temp"
	stagingDir    = "staging"

	// This is a list of errors that the signedURL endpoint will return.
	errUnauth     = "401 Unauthorized" // Auth credentials are wrong.
	errNotFound   = "404 Not Found"    // Base URL is wrong.
	errApigeeDown = "code 50"          // Internal Apigee issue.

	// collection interval is not configurable at the moment because UAP can
	// become unstable if all the Istio adapters are spamming it faster than
	// that. Hard code for now.
	defaultCollectionInterval = 1 * time.Minute

	closeChannelSize = 0
)

// A manager is a way for Istio to interact with Apigee's analytics platform.
type manager struct {
	env                adapter.Env
	close              chan bool
	client             *http.Client
	now                func() time.Time
	log                adapter.Logger
	collectionInterval time.Duration
	tempDir            string // open gzip files being written to
	stagingDir         string // gzip files staged for upload
	stagingFileLimit   int
	bucketsLock        sync.RWMutex
	buckets            map[string]*bucket // dir ("org~env") -> bucket
	baseURL            url.URL
	key                string
	secret             string
	sendChannelSize    int
	closeWait          sync.WaitGroup
}

// Start starts the manager.
func (m *manager) Start(env adapter.Env) {
	m.env = env
	m.log = env.Logger()
	m.log.Infof("starting analytics manager")

	if err := m.crashRecovery(); err != nil {
		m.log.Errorf("Error(s) recovering crashed data: %s", err)
	}

	env.ScheduleDaemon(func() {
		m.uploadLoop()
	})
	m.log.Infof("started analytics manager")
}

// Close shuts down the manager.
func (m *manager) Close() {
	if m == nil {
		return
	}
	m.log.Infof("closing analytics manager: %s", m.tempDir)
	m.bucketsLock.Lock()
	m.closeWait.Add(len(m.buckets))
	for _, b := range m.buckets {
		b.stop()
	}
	m.buckets = nil
	m.bucketsLock.Unlock()
	m.closeWait.Wait()
	if err := m.uploadAll(); err != nil {
		m.log.Errorf("Error pushing analytics: %s", err)
	}
	m.log.Infof("closed analytics manager")
}

// uploadLoop periodically uploads everything in the tempDir
func (m *manager) uploadLoop() {
	t := time.NewTicker(m.collectionInterval)
	for {
		select {
		case <-t.C:
			if err := m.uploadAll(); err != nil {
				m.log.Errorf("Error pushing analytics: %s", err)
			}
		case <-m.close:
			m.log.Debugf("analytics upload loop closed")
			return
		}
	}
}

// shortCircuitErr checks if we should bail early on this error (i.e. an error
// that will be the same for all requests, like an auth fail or Apigee is down).
func (m *manager) shortCircuitErr(err error) bool {
	s := err.Error()
	return strings.Contains(s, errUnauth) ||
		strings.Contains(s, errNotFound) ||
		strings.Contains(s, errApigeeDown)
}

// SendRecords sends the records asynchronously to the UAP primary server.
func (m *manager) SendRecords(ctx *auth.Context, incoming []Record) error {
	// Validate the records
	now := m.now()
	records := make([]Record, 0, len(incoming))
	for _, record := range incoming {
		record := record.ensureFields(ctx)
		if err := record.validate(now); err != nil {
			m.log.Errorf("invalid record %v: %s", record, err)
			continue
		}
		records = append(records, record)
	}

	bucket, err := m.getBucket(ctx)
	if err != nil {
		return fmt.Errorf("get bucket: %s", err)
	}

	bucket.write(records)
	return nil
}

func (m *manager) getBucket(ctx *auth.Context) (*bucket, error) {
	tenant := fmt.Sprintf("%s~%s", ctx.Organization(), ctx.Environment())

	m.bucketsLock.RLock()
	if _, ok := m.buckets[tenant]; ok {
		m.bucketsLock.RUnlock()
		return m.buckets[tenant], nil
	}

	m.bucketsLock.RUnlock()
	m.bucketsLock.Lock()
	defer m.bucketsLock.Unlock()

	// double check after lock
	if _, ok := m.buckets[tenant]; ok {
		return m.buckets[tenant], nil
	}

	// ensure directory exists
	dir := path.Join(m.tempDir, tenant)
	if err := os.MkdirAll(dir, bufferMode); err != nil {
		return nil, fmt.Errorf("mkdir %s: %s", dir, err)
	}

	b := &bucket{
		manager:  m,
		log:      m.log,
		dir:      dir,
		tenant:   tenant,
		incoming: make(chan []Record, m.sendChannelSize),
		closer:   make(chan closeReq, closeChannelSize),
	}
	m.env.ScheduleDaemon(b.runLoop)

	m.buckets[tenant] = b
	return b, nil
}
