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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/auth"
)

// bucket tracks a specific quota instance
type bucket struct {
	manager      *Manager
	quotaURL     string
	prototype    *Request
	requests     []*Request
	result       *Result
	created      time.Time
	lock         sync.RWMutex
	now          func() time.Time
	synced       time.Time     // last sync
	checked      time.Time     // last apply
	refreshAfter time.Duration // after synced
	deleteAfter  time.Duration // after checked
}

func newBucket(req *Request, m *Manager, auth *auth.Context) *bucket {
	org := auth.Context.Organization()
	env := auth.Context.Environment()
	quotaURL := *m.baseURL
	quotaURL.Path = path.Join(quotaURL.Path, fmt.Sprintf(quotaPath, org, env))
	return &bucket{
		prototype:    req,
		manager:      m,
		quotaURL:     quotaURL.String(),
		requests:     nil,
		result:       nil,
		created:      m.now(),
		lock:         sync.RWMutex{},
		now:          m.now,
		deleteAfter:  defaultDeleteAfter,
		refreshAfter: defaultRefreshAfter,
	}
}

// apply a quota request to the local quota bucket and schedule for sync
func (b *bucket) apply(m *Manager, req *Request) (*Result, error) {
	if !b.isCompatible(req) {
		return nil, fmt.Errorf("incompatible quota buckets")
	}

	b.lock.Lock()
	defer b.lock.Unlock()
	b.checked = b.now()
	res := &Result{
		Allowed:    req.Allow,
		ExpiryTime: b.checked.Unix(),
		Timestamp:  b.checked.Unix(),
	}
	if b.result != nil {
		res.Used = b.result.Used // start from last result
		res.Used += b.result.Exceeded
	}
	var dupRequest bool
	for _, r := range b.requests {
		res.Used += r.Weight
		if req.DeduplicationID != "" && r.DeduplicationID == req.DeduplicationID {
			dupRequest = true
		}
	}
	if !dupRequest {
		res.Used += req.Weight
		b.requests = append(b.requests, req)
	}
	if res.Used > res.Allowed {
		res.Exceeded = res.Used - res.Allowed
		res.Used = res.Allowed
	}
	return res, nil
}

func (b *bucket) isCompatible(r *Request) bool {
	return b.prototype.Interval == r.Interval &&
		b.prototype.Allow == r.Allow &&
		b.prototype.TimeUnit == r.TimeUnit &&
		b.prototype.Identifier == r.Identifier
}

// sync local quota bucket with server
func (b *bucket) sync(m *Manager) {
	b.lock.Lock()
	requests := b.requests
	b.requests = nil
	b.lock.Unlock()

	var weight int64
	for _, r := range requests {
		weight += r.Weight
	}

	r := *b.prototype // make copy
	r.Weight = weight

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(r)

	req, err := http.NewRequest(http.MethodPost, b.quotaURL, body)
	if err != nil {
		m.log.Errorf("unable to create quota sync request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	m.log.Debugf("Sending to %s: %s", b.quotaURL, body)

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		m.log.Errorf("unable to sync quota: %v", err)
		return
	}
	defer resp.Body.Close()

	buf := bytes.NewBuffer(make([]byte, 0, resp.ContentLength))
	_, err = buf.ReadFrom(resp.Body)
	respBody := buf.Bytes()

	switch resp.StatusCode {
	case 200:
		var quotaResult Result
		if err = json.Unmarshal(respBody, &quotaResult); err != nil {
			m.log.Errorf("Error unmarshalling: %s", string(respBody))
			return
		}

		m.log.Debugf("quota result: %#v", quotaResult)
		b.lock.Lock()
		b.synced = b.now()
		b.result = &quotaResult
		b.lock.Unlock()

	default:
		m.log.Errorf("quota sync failed. result: %s", string(respBody))
	}
}

func (b *bucket) needToDelete() bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.requests == nil && b.now().After(b.checked.Add(b.deleteAfter))
}

func (b *bucket) needToSync() bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.requests != nil || b.now().After(b.synced.Add(b.refreshAfter))
}
