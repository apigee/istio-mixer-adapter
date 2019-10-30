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
	"path/filepath"
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

	// This is a list of errors that the signedURL endpoint will return.
	errUnauth     = "401 Unauthorized" // Auth credentials are wrong.
	errNotFound   = "404 Not Found"    // Base URL is wrong.
	errApigeeDown = "code 50"          // Internal Apigee issue.

	// collection interval is not configurable at the moment because UAP can
	// become unstable if all the Istio adapters are spamming it faster than
	// that. Hard code for now.
	defaultCollectionInterval = 1 * time.Minute
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
	stageLock          sync.Mutex
	closed             bool
}

// Start starts the manager.
func (m *manager) Start(env adapter.Env) {
	m.env = env
	m.log = env.Logger()
	m.log.Infof("starting analytics manager: %s", m.tempDir)

	if err := m.crashRecovery(); err != nil {
		m.log.Errorf("Error(s) recovering crashed data: %s", err)
	}

	env.ScheduleDaemon(func() {
		m.stagingLoop()
	})

	m.log.Infof("started analytics manager: %s", m.tempDir)
}

// Close shuts down the manager
func (m *manager) Close() {
	if m == nil {
		return
	}
	m.log.Infof("closing analytics manager: %s", m.tempDir)

	m.close <- true

	// close buckets
	m.bucketsLock.Lock()
	m.closed = true
	m.bucketsLock.Unlock()

	// stage and upload everything
	m.stageAllBucketsWait()
	if err := m.uploadAll(); err != nil {
		m.log.Errorf("Error pushing analytics: %s", err)
	}

	m.log.Infof("closed analytics manager")
}

// stagingLoop periodically close all buckets for upload
func (m *manager) stagingLoop() {
	t := time.NewTicker(m.collectionInterval)
	for {
		select {

		// stage and upload everything
		case <-t.C:
			m.stageAllBucketsWait()
			if err := m.uploadAll(); err != nil {
				m.log.Errorf("Error pushing analytics: %s", err)
			}

		// close loop
		case <-m.close:
			m.log.Debugf("analytics staging loop closed: %s", m.tempDir)
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
	if m == nil || len(incoming) == 0 {
		return nil
	}

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

// lazy creates bucket, returns nil and no error if manager is closed
func (m *manager) getBucket(ctx *auth.Context) (*bucket, error) {
	tenant := fmt.Sprintf("%s~%s", ctx.Organization(), ctx.Environment())
	m.bucketsLock.RLock()
	if _, ok := m.buckets[tenant]; ok {
		bucket := m.buckets[tenant]
		m.bucketsLock.RUnlock()
		return bucket, nil
	}
	m.bucketsLock.RUnlock()

	return m.createBucket(ctx, tenant)
}

func (m *manager) createBucket(ctx *auth.Context, tenant string) (*bucket, error) {
	m.bucketsLock.Lock()
	defer m.bucketsLock.Unlock()

	if m.closed {
		return nil, nil
	}

	if bucket, ok := m.buckets[tenant]; ok {
		return bucket, nil
	}

	if err := m.prepTenant(tenant); err != nil {
		return nil, err
	}

	bucket := newBucket(m, m.getTempDir(tenant))
	m.buckets[tenant] = bucket
	return bucket, nil
}

func (m *manager) prepTenant(tenant string) error {
	bufferMode := os.FileMode(0700)

	dir := m.getTempDir(tenant)
	if err := os.MkdirAll(dir, bufferMode); err != nil {
		return fmt.Errorf("mkdir %s: %s", dir, err)
	}

	dir = m.getStagingDir(tenant)
	if err := os.MkdirAll(dir, bufferMode); err != nil {
		return fmt.Errorf("mkdir %s: %s", dir, err)
	}

	return nil
}

func (m *manager) getTempDir(tenant string) string {
	return filepath.Join(m.tempDir, tenant)
}

func (m *manager) getStagingDir(tenant string) string {
	return filepath.Join(m.stagingDir, tenant)
}

func getTenantName(org, env string) string {
	return fmt.Sprintf("%s~%s", org, env)
}
