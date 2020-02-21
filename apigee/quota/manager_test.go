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
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/authtest"
	"github.com/apigee/istio-mixer-adapter/apigee/product"
	"istio.io/istio/mixer/pkg/adapter"
)

func TestQuota(t *testing.T) {

	type testcase struct {
		name    string
		dedupID string
		want    Result
	}

	var m *Manager
	m.Close() // just to verify it doesn't die here

	serverResult := Result{}
	ts := testServer(&serverResult, time.Now, nil)

	context := authtest.NewContext(ts.URL)
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
	m, err = NewManager(Options{
		BaseURL: context.ApigeeBase(),
		Client:  http.DefaultClient,
		Key:     "key",
		Secret:  "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	cases := []testcase{
		{
			name:    "first",
			dedupID: "X",
			want: Result{
				Used:     1,
				Exceeded: 0,
			},
		},
		{
			name:    "duplicate",
			dedupID: "X",
			want: Result{
				Used:     1,
				Exceeded: 0,
			},
		},
		{
			name:    "second",
			dedupID: "Y",
			want: Result{
				Used:     1,
				Exceeded: 1,
			},
		},
	}

	for _, c := range cases {
		t.Logf("** Executing test case '%s' **", c.name)

		args.DeduplicationID = c.dedupID
		result, err := m.Apply(authContext, p, args)
		if err != nil {
			t.Fatalf("should not get error: %v", err)
		}
		if result.Used != c.want.Used {
			t.Errorf("used got: %v, want: %v", result.Used, c.want.Used)
		}
		if result.Exceeded != c.want.Exceeded {
			t.Errorf("exceeded got: %v, want: %v", result.Exceeded, c.want.Exceeded)
		}
	}

	// test incompatible product (replaces bucket)
	p2 := &product.APIProduct{
		QuotaLimitInt:    1,
		QuotaIntervalInt: 2,
		QuotaTimeUnit:    "second",
	}
	c := testcase{
		name:    "incompatible",
		dedupID: "Z",
		want: Result{
			Used:     1,
			Exceeded: 0,
		},
	}

	t.Logf("** Executing test case '%s' **", c.name)
	args.DeduplicationID = c.dedupID
	result, err := m.Apply(authContext, p2, args)
	if err != nil {
		t.Fatalf("should not get error: %v", err)
	}
	if result.Used != c.want.Used {
		t.Errorf("used got: %v, want: %v", result.Used, c.want.Used)
	}
	if result.Exceeded != c.want.Exceeded {
		t.Errorf("exceeded got: %v, want: %v", result.Exceeded, c.want.Exceeded)
	}
}

// not fully determinate, uses delays and background threads
func TestSync(t *testing.T) {

	fakeTime := int64(1521221450)
	now := func() time.Time { return time.Unix(fakeTime, 0) }
	serverResult := Result{}
	ts := testServer(&serverResult, now, nil)
	defer ts.Close()

	context := authtest.NewContext(ts.URL)

	quotaID := "id"
	request := &Request{
		Identifier: quotaID,
		Interval:   1,
		TimeUnit:   "seconds",
		Allow:      1,
		Weight:     3,
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
		syncingBuckets: map[*bucket]struct{}{},
		key:            "key",
		secret:         "secret",
	}

	b := newBucket(*request, m)
	b.checked = now()
	b.result = result
	m.buckets = map[string]*bucket{quotaID: b}
	b.refreshAfter = time.Millisecond

	m.Start()
	defer m.Close()

	fakeTime = fakeTime + 10
	time.Sleep(10 * time.Millisecond) // allow idle sync
	b.refreshAfter = time.Hour

	b.lock.RLock()
	if b.request.Weight != 0 {
		t.Errorf("pending request weight got: %d, want: %d", b.request.Weight, 0)
	}
	if !reflect.DeepEqual(*b.result, serverResult) {
		t.Errorf("result got: %#v, want: %#v", *b.result, serverResult)
	}
	if b.synced != m.now() {
		t.Errorf("synced got: %#v, want: %#v", b.synced, m.now())
	}
	if m.buckets[quotaID] == nil {
		t.Errorf("old bucket should not have been deleted")
	}
	b.lock.RUnlock()

	// do interactive sync
	req := &Request{
		Identifier: quotaID,
		Interval:   1,
		TimeUnit:   "seconds",
		Allow:      1,
		Weight:     2,
	}
	_, err := b.apply(req)
	if err != nil {
		t.Errorf("should not have received error on apply: %v", err)
	}
	fakeTime = fakeTime + 10
	err = b.sync()
	if err != nil {
		t.Errorf("should not have received error on sync: %v", err)
	}

	b.lock.Lock()
	if b.request.Weight != 0 {
		t.Errorf("pending request weight got: %d, want: %d", b.request.Weight, 0)
	}
	if !reflect.DeepEqual(*b.result, serverResult) {
		t.Errorf("result got: %#v, want: %#v", *b.result, serverResult)
	}
	if b.synced != m.now() {
		t.Errorf("synced got: %#v, want: %#v", b.synced, m.now())
	}

	fakeTime = fakeTime + 10*60
	b.lock.Unlock()
	time.Sleep(10 * time.Millisecond) // allow background delete
	m.bucketsLock.RLock()
	defer m.bucketsLock.RUnlock()
	if m.buckets[quotaID] != nil {
		t.Errorf("old bucket should have been deleted")
	}
}

func TestDisconnected(t *testing.T) {
	now := func() time.Time { return time.Unix(1521221450, 0) }

	errC := &errControl{
		send: 404,
	}
	serverResult := Result{}
	ts := testServer(&serverResult, now, errC)
	defer ts.Close()

	context := authtest.NewContext(ts.URL)
	context.SetOrganization("org")
	context.SetEnvironment("env")
	authContext := &auth.Context{
		Context:        context,
		DeveloperEmail: "email",
		Application:    "app",
		AccessToken:    "token",
		ClientID:       "clientId",
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
		buckets:        map[string]*bucket{},
		syncingBuckets: map[*bucket]struct{}{},
		key:            "key",
		secret:         "secret",
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

	_, err := m.Apply(authContext, p, args)
	if err != nil {
		t.Errorf("shouln't get error: %v", err)
	}

	// force sync error
	err = m.forceSync(getQuotaID(authContext, p))
	if err == nil {
		t.Fatalf("should have received error: %s", err)
	}

	_, err = m.Apply(authContext, p, args)
	if err != nil {
		t.Errorf("shouln't get error: %v", err)
	}

	errC.send = 200
	m.forceSync(getQuotaID(authContext, p))

	res, err := m.Apply(authContext, p, args)
	if err != nil {
		t.Fatalf("got error: %s", err)
	}
	wantResult := Result{
		Allowed:    1,
		Used:       1,
		Exceeded:   2,
		ExpiryTime: now().Unix(),
		Timestamp:  now().Unix(),
	}
	if !reflect.DeepEqual(*res, wantResult) {
		t.Errorf("result got: %#v, want: %#v", *res, wantResult)
	}
}

func TestWindowExpired(t *testing.T) {
	fakeTime := int64(1521221450)
	now := func() time.Time { return time.Unix(fakeTime, 0) }

	errC := &errControl{
		send: 200,
	}
	serverResult := Result{}
	ts := testServer(&serverResult, now, errC)
	defer ts.Close()

	context := authtest.NewContext(ts.URL)
	context.SetOrganization("org")
	context.SetEnvironment("env")
	authContext := &auth.Context{
		Context:        context,
		DeveloperEmail: "email",
		Application:    "app",
		AccessToken:    "token",
		ClientID:       "clientId",
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
		buckets:        map[string]*bucket{},
		syncingBuckets: map[*bucket]struct{}{},
		key:            "key",
		secret:         "secret",
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

	res, err := m.Apply(authContext, p, args)
	m.forceSync(getQuotaID(authContext, p))

	quotaID := fmt.Sprintf("%s-%s", authContext.Application, p.Name)
	bucket := m.buckets[quotaID]

	if bucket.request.Weight != 0 {
		t.Errorf("got: %d, want: %d", bucket.request.Weight, 0)
	}
	if res.Used != 1 {
		t.Errorf("got: %d, want: %d", res.Used, 1)
	}

	fakeTime++
	if !bucket.windowExpired() {
		t.Errorf("should be expired")
	}

	res, err = m.Apply(authContext, p, args)
	if err != nil {
		t.Errorf("got error: %v", err)
	}
	if bucket.request.Weight != 1 {
		t.Errorf("got: %d, want: %d", bucket.request.Weight, 1)
	}

	err = bucket.sync() // after window expiration, should reset
	if err != nil {
		t.Errorf("got error: %v", err)
	}
	if bucket.result.Used != 1 {
		t.Errorf("got: %d, want: %d", bucket.result.Used, 1)
	}
	if bucket.result.Exceeded != 0 {
		t.Errorf("got: %d, want: %d", bucket.result.Exceeded, 0)
	}
}

type errControl struct {
	send int
}

func testServer(serverResult *Result, now func() time.Time, errC *errControl) *httptest.Server {

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if errC != nil && errC.send != 200 {
			w.WriteHeader(errC.send)
			w.Write([]byte("error"))
			return
		}

		username, password, ok := r.BasicAuth()
		if !ok || username != "key" || password != "secret" {
			w.WriteHeader(403)
			w.Write([]byte("invalid basic auth"))
			return
		}

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
}

// ignores if no matching quota bucket
func (m *Manager) forceSync(quotaID string) error {
	m.bucketsLock.RLock()
	b, ok := m.buckets[quotaID]
	if !ok {
		return nil
	}
	m.syncingBucketsLock.Lock()
	m.syncingBuckets[b] = struct{}{}
	m.syncingBucketsLock.Unlock()
	defer func() {
		m.syncingBucketsLock.Lock()
		delete(m.syncingBuckets, b)
		m.syncingBucketsLock.Unlock()
	}()
	return b.sync()
}
