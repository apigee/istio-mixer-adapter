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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/authtest"
	"github.com/apigee/istio-mixer-adapter/apigee/product"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/pkg/adapter/test"
)

func TestQuota(t *testing.T) {

	var m *Manager
	m.Close() // just to verify it doesn't die here

	env := test.NewEnv(t)
	context := authtest.NewContext("", env)
	authContext := &auth.Context{
		Context: context,
	}

	p := product.APIProduct{
		QuotaLimitInt:    1,
		QuotaIntervalInt: 1,
		QuotaTimeUnit:    "second",
	}

	args := adapter.QuotaArgs{
		DeduplicationID: "X",
		QuotaAmount:     1,
		BestEffort:      true,
	}

	m = NewManager(context.ApigeeBase(), env)
	m.Start(env)
	defer m.Close()

	result := m.Apply(*authContext, p, args)
	if result.Used != 1 {
		t.Errorf("result used should be 1")
	}
	if result.Exceeded != 0 {
		t.Errorf("result exceeded should be 0")
	}

	result = m.Apply(*authContext, p, args)
	if result.Used != 1 {
		t.Errorf("result used should be 1")
	}
	if result.Exceeded != 1 {
		t.Errorf("result exceeded should be 1")
	}
}

// todo: make determinate
func TestSync(t *testing.T) {

	now := func() time.Time { return time.Unix(1521221450, 0) }
	serverResult := Result{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := Request{}
		json.NewDecoder(r.Body).Decode(&req)
		serverResult.Allowed = req.Allow
		serverResult.Used += req.Weight
		if serverResult.Used > serverResult.Allowed {
			serverResult.Exceeded = serverResult.Used - serverResult.Allowed
			serverResult.Used = serverResult.Allowed
		}
		serverResult.Timestamp = now().Unix()
		serverResult.ExpiryTime = now().Unix()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(serverResult)
	}))
	defer ts.Close()

	env := test.NewEnv(t)
	context := authtest.NewContext(ts.URL, env)
	context.SetOrganization("org")
	context.SetEnvironment("env")
	authContext := &auth.Context{
		Context:        context,
		DeveloperEmail: "email",
		Application:    "app",
		AccessToken:    "token",
		ClientID:       "clientId",
	}

	requests := []*Request{
		{
			Weight: 2,
		},
		{
			Weight: 1,
		},
	}
	result := &Result{
		Used: 1,
	}

	b := &bucket{
		org:          authContext.Organization(),
		env:          authContext.Environment(),
		id:           "id",
		requests:     requests,
		result:       result,
		created:      now(),
		lock:         sync.RWMutex{},
		now:          now,
		refreshAfter: time.Millisecond,
	}

	m := &Manager{
		close: make(chan bool),
		client: &http.Client{
			Timeout: httpTimeout,
		},
		now:               now,
		syncLoopPollEvery: 2 * time.Millisecond,
		buckets:           map[string]*bucket{b.id: b},
		syncQueue:         make(chan *bucket, 10),
		baseURL:           context.ApigeeBase(),
		numSyncWorkers:    1,
	}
	m.Start(env)
	defer m.Close()

	time.Sleep(10 * time.Millisecond) // allow idle sync
	if len(b.requests) != 0 {
		t.Errorf("num request got: %d, want: %d", len(b.requests), 0)
	}
	if !reflect.DeepEqual(*b.result, serverResult) {
		t.Errorf("result got: %#v, want: %#v", *b.result, serverResult)
	}
	if b.synced != m.now() {
		t.Errorf("synced got: %#v, want: %#v", b.synced, m.now())
	}

	// do interactive sync
	req := &Request{
		Allow:  3,
		Weight: 2,
	}
	m.now = func() time.Time { return time.Unix(1521221451, 0) }
	b.apply(m, req)
	time.Sleep(10 * time.Millisecond) // allow background sync

	if len(b.requests) != 0 {
		t.Errorf("num request got: %d, want: %d", len(b.requests), 0)
	}
	if !reflect.DeepEqual(*b.result, serverResult) {
		t.Errorf("result got: %#v, want: %#v", *b.result, serverResult)
	}
	if b.synced != m.now() {
		t.Errorf("synced got: %#v, want: %#v", b.synced, m.now())
	}

	b.deleteAfter = time.Millisecond
	time.Sleep(10 * time.Millisecond) // allow background delete
	m.bucketsLock.RLock()
	defer m.bucketsLock.RUnlock()
	if m.buckets[b.id] != nil {
		t.Errorf("old bucket should have been deleted")
	}

	b.refreshAfter = time.Hour
}
