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
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"github.com/apigee/istio-mixer-adapter/adapter/authtest"
	"github.com/apigee/istio-mixer-adapter/adapter/product"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/pkg/adapter/test"
)

func TestQuota(t *testing.T) {

	var m *Manager
	m.Close() // just to verify it doesn't die here

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := Result{}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer ts.Close()

	env := test.NewEnv(t)
	context := authtest.NewContext(ts.URL, env)
	authContext := &auth.Context{
		Context: context,
	}

	p := &product.APIProduct{
		QuotaLimitInt:    1,
		QuotaIntervalInt: 1,
		QuotaTimeUnit:    "second",
	}

	args := adapter.QuotaArgs{
		QuotaAmount: 1,
		BestEffort:  true,
	}

	var err error
	m, err = NewManager(env, Options{
		BaseURL: context.ApigeeBase(),
		Client:  http.DefaultClient,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	cases := []struct {
		test            string
		deduplicationID string
		want            Result
	}{
		{
			test:            "first",
			deduplicationID: "X",
			want: Result{
				Used:     1,
				Exceeded: 0,
			},
		},
		{
			test:            "duplicate",
			deduplicationID: "X",
			want: Result{
				Used:     1,
				Exceeded: 0,
			},
		},
		{
			test:            "second",
			deduplicationID: "Y",
			want: Result{
				Used:     1,
				Exceeded: 1,
			},
		},
	}

	for _, c := range cases {
		t.Logf("** Executing test case '%s' **", c.test)

		args.DeduplicationID = c.deduplicationID
		result, err := m.Apply(authContext, p, args)
		if err != nil {
			t.Errorf("should not get error: %v", err)
		}
		if result.Used != c.want.Used {
			t.Errorf("used got: %v, want: %v", result.Used, c.want.Used)
		}
		if result.Exceeded != c.want.Exceeded {
			t.Errorf("exceeded got: %v, want: %v", result.Exceeded, c.want.Exceeded)
		}
	}
}

// not fully determinate, uses delays and background threads
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

	quotaID := "id"
	requests := []*Request{
		{
			Identifier: quotaID,
			Weight:     2,
		},
		{
			Weight: 1,
		},
	}
	result := &Result{
		Used: 1,
	}

	m := &Manager{
		close:          make(chan bool),
		closed:         make(chan bool),
		client:         http.DefaultClient,
		now:            now,
		syncRate:       2 * time.Millisecond,
		syncQueue:      make(chan *bucket, 10),
		baseURL:        context.ApigeeBase(),
		numSyncWorkers: 1,
	}
	m.Start(env)
	defer m.Close()

	b := newBucket(requests[0], m, authContext)
	b.lock.Lock()
	b.created = now()
	b.now = now
	b.requests = requests
	b.result = result
	m.bucketsLock.Lock()
	m.buckets = map[string]*bucket{quotaID: b}
	m.bucketsLock.Unlock()
	b.refreshAfter = time.Millisecond
	b.lock.Unlock()

	time.Sleep(15 * time.Millisecond) // allow idle sync
	b.lock.RLock()
	if len(b.requests) != 0 {
		t.Errorf("pending requests got: %d, want: %d", len(b.requests), 0)
	}
	if !reflect.DeepEqual(*b.result, serverResult) {
		t.Errorf("result got: %#v, want: %#v", *b.result, serverResult)
	}
	if b.synced != m.now() {
		t.Errorf("synced got: %#v, want: %#v", b.synced, m.now())
	}
	b.lock.RUnlock()

	// do interactive sync
	req := &Request{
		Allow:  3,
		Weight: 2,
	}
	b.apply(m, req)
	b.sync(m)

	b.lock.Lock()
	if len(b.requests) != 0 {
		t.Errorf("pending requests got: %d, want: %d", len(b.requests), 0)
	}
	if !reflect.DeepEqual(*b.result, serverResult) {
		t.Errorf("result got: %#v, want: %#v", *b.result, serverResult)
	}
	if b.synced != m.now() {
		t.Errorf("synced got: %#v, want: %#v", b.synced, m.now())
	}

	b.deleteAfter = time.Millisecond
	b.lock.Unlock()
	time.Sleep(10 * time.Millisecond) // allow background delete
	m.bucketsLock.RLock()
	defer m.bucketsLock.RUnlock()
	if m.buckets[quotaID] != nil {
		t.Errorf("old bucket should have been deleted")
	}

	b.refreshAfter = time.Hour
}
