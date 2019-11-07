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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"github.com/apigee/istio-mixer-adapter/adapter/util"
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

	// limited to 2 for now to limit upload stress
	numUploaders = 2
)

// Start starts the manager.
func (m *manager) Start(env adapter.Env) {
	m.env = env
	m.log = env.Logger()
	m.log.Infof("starting analytics manager: %s", m.tempDir)

	// start upload channel and workers
	errNoRetry := fmt.Errorf("analytics closed, no retry on upload")
	errHandler := func(err error) error {
		if m.closed {
			return errNoRetry
		}
		env.Logger().Errorf("analytics upload: %v", err)
		return nil
	}
	m.startUploader(env, errHandler)

	// handle anything hanging around in temp or staging
	if err := m.crashRecovery(); err != nil {
		m.log.Errorf("Error(s) recovering crashed data: %s", err)
	}

	m.startStagingSweeper(env)

	m.log.Infof("started analytics manager: %s", m.tempDir)
}

func (m *manager) startStagingSweeper(env adapter.Env) {
	env.ScheduleDaemon(func() {
		m.stagingLoop()
	})
}

func (m *manager) startUploader(env adapter.Env, errHandler util.ErrorFunc) {
	m.uploadersWait = sync.WaitGroup{}
	ctx := context.Background()

	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	limit := m.stagingFileLimit - numUploaders
	send, receive, overflow := util.NewReservoir(env, limit)
	m.uploadChan = send

	// handle uploads
	for i := 0; i < numUploaders; i++ {
		l := util.Looper{
			Env:     env,
			Backoff: util.DefaultExponentialBackoff(),
		}
		env.ScheduleDaemon(func() {
			m.uploadersWait.Add(1)
			defer m.uploadersWait.Done()

			for work := range receive {
				l.Run(ctx, work.(util.WorkFunc), errHandler)
			}
		})
	}

	// handle overflow
	env.ScheduleDaemon(func() {
		m.uploadersWait.Add(1)
		defer m.uploadersWait.Done()

		for dropped := range overflow {
			env.ScheduleWork(func() {
				work := dropped.(util.WorkFunc)
				work(canceledCtx)
			})
		}
	})
}

// Close shuts down the manager
func (m *manager) Close() {
	if m == nil {
		return
	}
	m.log.Infof("closing analytics manager: %s", m.tempDir)

	m.bucketsLock.Lock()
	m.closed = true
	m.bucketsLock.Unlock()

	m.closeStaging <- true

	// force stage and upload
	m.stageAllBucketsWait()
	close(m.uploadChan)
	m.uploadersWait.Wait()

	m.log.Infof("closed analytics manager")
}

// stagingLoop periodically closes and sweeps open buckets to staging
func (m *manager) stagingLoop() {
	t := time.NewTicker(m.collectionInterval)
	for {
		select {
		case <-t.C:
			m.stageAllBucketsWait()

		case <-m.closeStaging:
			m.log.Debugf("analytics staging loop closed: %s", m.tempDir)
			return
		}
	}
}

// SendRecords is called by Mixer, spools records for sending
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

	return m.writeToBucket(ctx, records)
}

func (m *manager) writeToBucket(ctx *auth.Context, records []Record) error {
	tenant := fmt.Sprintf("%s~%s", ctx.Organization(), ctx.Environment())

	m.bucketsLock.RLock()
	if bucket, ok := m.buckets[tenant]; ok {
		bucket.write(records)
		m.bucketsLock.RUnlock()
		return nil
	}

	// no bucket, we'll have to work harder
	m.bucketsLock.RUnlock()
	m.bucketsLock.Lock()
	defer m.bucketsLock.Unlock()

	bucket, ok := m.buckets[tenant]
	if !ok {
		if err := m.prepTenant(tenant); err != nil {
			return err
		}

		var err error
		bucket, err = newBucket(m, tenant, m.getTempDir(tenant))
		if err != nil {
			return err
		}
		m.buckets[tenant] = bucket
	}
	bucket.write(records)
	return nil
}

// ensures tenant temp and staging dirs are created
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
