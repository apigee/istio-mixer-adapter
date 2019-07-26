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
	request      *Request // accumulated for sync
	result       *Result
	created      time.Time
	lock         sync.RWMutex
	synced       time.Time     // last sync time
	checked      time.Time     // last apply time
	refreshAfter time.Duration // duration after synced
	deleteAfter  time.Duration // duration after checked
	invalidAfter time.Time     // result window is no longer valid after this
	syncError    error
}

func newBucket(req Request, m *Manager, auth *auth.Context) *bucket {
	org := auth.Context.Organization()
	env := auth.Context.Environment()
	quotaURL := *m.baseURL
	quotaURL.Path = path.Join(quotaURL.Path, fmt.Sprintf(quotaPath, org, env))
	return &bucket{
		request:      &req,
		manager:      m,
		quotaURL:     quotaURL.String(),
		result:       nil,
		created:      m.now(),
		checked:      m.now(),
		lock:         sync.RWMutex{},
		deleteAfter:  defaultDeleteAfter,
		refreshAfter: defaultRefreshAfter,
	}
}

func (b *bucket) now() time.Time {
	return b.manager.now()
}

// apply a quota request to the local quota bucket and schedule for sync
func (b *bucket) apply(req *Request) (*Result, error) {

	if !b.compatible(req) {
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

	b.request.Weight += req.Weight
	res.Used += b.request.Weight

	if res.Used > res.Allowed {
		res.Exceeded = res.Used - res.Allowed
		res.Used = res.Allowed
	}

	return res, b.syncError
}

func (b *bucket) compatible(r *Request) bool {
	return b.request.Interval == r.Interval &&
		b.request.Allow == r.Allow &&
		b.request.TimeUnit == r.TimeUnit &&
		b.request.Identifier == r.Identifier
}

// sync local quota bucket with server
// single-threaded call - managed by manager
func (b *bucket) sync() error {

	log := b.manager.log
	log.Debugf("syncing quota %s", b.request.Identifier)

	revert := func(err error) error {
		err = log.Errorf("unable to sync quota %s: %v", b.request.Identifier, err)
		b.lock.Lock()
		b.syncError = err
		b.lock.Unlock()
		return err
	}

	b.lock.Lock()
	r := *b.request // make copy

	if b.windowExpired() {
		r.Weight = 0 // if expired, don't send Weight
	}
	b.lock.Unlock()

	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(r)
	if err != nil {
		return revert(fmt.Errorf("encode: %v", err))
	}

	req, err := http.NewRequest(http.MethodPost, b.quotaURL, body)
	if err != nil {
		return revert(fmt.Errorf("new request: %v", err))
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	log.Debugf("sending quota: %s", body)

	resp, err := b.manager.client.Do(req)
	if err != nil {
		return revert(fmt.Errorf("do request: %v", err))
	}
	defer resp.Body.Close()

	buf := bytes.NewBuffer(make([]byte, 0, resp.ContentLength))
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return revert(fmt.Errorf("read body: %v", err))
	}
	respBody := buf.Bytes()

	switch resp.StatusCode {
	case 200:
		var quotaResult Result
		if err = json.Unmarshal(respBody, &quotaResult); err != nil {
			return revert(fmt.Errorf("bad response: %s", string(respBody)))
		}

		log.Debugf("quota synced: %#v", quotaResult)
		b.lock.Lock()
		b.synced = b.now()
		if b.result != nil && b.result.ExpiryTime != quotaResult.ExpiryTime {
			b.request.Weight = 0
		} else {
			b.request.Weight -= r.Weight // same window, keep accumulated Weight
		}
		b.result = &quotaResult
		b.syncError = nil
		b.lock.Unlock()
		return nil

	default:
		return revert(fmt.Errorf("bad response (%d): %s", resp.StatusCode, string(respBody)))
	}
}

func (b *bucket) needToDelete() bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.request.Weight == 0 && b.now().After(b.checked.Add(b.deleteAfter))
}

func (b *bucket) needToSync() bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.request.Weight > 0 || b.now().After(b.synced.Add(b.refreshAfter))
}

func (b *bucket) windowExpired() bool {
	if b.result != nil {
		return b.now().After(time.Unix(b.result.ExpiryTime, 0))
	}
	return false
}
