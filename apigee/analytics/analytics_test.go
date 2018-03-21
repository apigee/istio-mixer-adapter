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
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/authtest"
	adaptertest "istio.io/istio/mixer/pkg/adapter/test"
)

// a testRecordPush represents a single push of analytics to a given directory.
type testRecordPush struct {
	records []Record
	dir     string
}

// A fakeServer wraps around an httptest.Server and tracks the things that have
// been sent to it.
type fakeServer struct {
	records     map[string][]testRecordPush
	srv         *httptest.Server
	failAuth    bool
	failedCalls int
}

func newFakeServer(t *testing.T) *fakeServer {
	fs := &fakeServer{
		records: map[string][]testRecordPush{},
	}
	fs.srv = httptest.NewServer(fs.handler(t))
	return fs
}

func (fs *fakeServer) handler(t *testing.T) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/analytics", func(w http.ResponseWriter, r *http.Request) {
		if fs.failAuth {
			// UAP gives a 404 response when we don't auth properly.
			fs.failedCalls++
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// Give them a signed URL. Include the file path they picked so that we can
		// confirm they are sending the right one.
		url := "%s/signed-url-1234?relative_file_path=%s&tenant=%s"
		json.NewEncoder(w).Encode(map[string]interface{}{
			"url": fmt.Sprintf(url, fs.srv.URL, r.FormValue("relative_file_path"), r.FormValue("tenant")),
		})
	})
	m.HandleFunc("/signed-url-1234", func(w http.ResponseWriter, r *http.Request) {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatalf("Error on gzip.NewReader: %s", err)
		}
		defer gz.Close()
		defer r.Body.Close()

		rec := []Record{}
		if err := json.NewDecoder(gz).Decode(&rec); err != nil {
			t.Fatalf("Error decoding JSON sent to signed URL: %s", err)
		}
		tenant := r.FormValue("tenant")
		fp := r.FormValue("relative_file_path")
		fs.records[tenant] = append(fs.records[tenant], testRecordPush{
			records: rec,
			dir:     path.Dir(fp),
		})

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// This matches every other route - we should not hit this one.
		t.Fatalf("Unknown route %s hit", r.URL.Path)
	})
	return m
}

func (fs *fakeServer) Close()                               { fs.srv.Close() }
func (fs *fakeServer) Records() map[string][]testRecordPush { return fs.records }
func (fs *fakeServer) URL() string                          { return fs.srv.URL }

func TestPushAnalytics(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	defer fs.Close()

	t1 := "hi~test"
	t2 := "otherorg~test"
	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.

	m := newManager()
	m.now = func() time.Time { return time.Unix(ts, 0) }
	m.collectionInterval = 50 * time.Millisecond

	wantRecords := map[string][]testRecordPush{
		t1: {
			{
				records: []Record{{APIProxy: "proxy"}, {APIProduct: "product"}},
				dir:     fmt.Sprintf("date=2018-03-16/time=%d-%d", ts, ts),
			},
		},
		t2: {
			{
				records: []Record{{RequestURI: "request URI"}},
				dir:     fmt.Sprintf("date=2018-03-16/time=%d-%d", ts, ts),
			},
		},
	}

	env := adaptertest.NewEnv(t)
	m.Start(env)
	defer m.Close()

	tc := authtest.NewContext(fs.URL(), env)
	tc.SetOrganization("hi")
	tc.SetEnvironment("test")
	ctx := &auth.Context{Context: tc}

	// Send them in batches to ensure we group them all together.
	if err := m.SendRecords(ctx, wantRecords[t1][0].records[:1]); err != nil {
		t.Errorf("Error on SendRecords(): %s", err)
	}
	if err := m.SendRecords(ctx, wantRecords[t1][0].records[1:]); err != nil {
		t.Errorf("Error on SendRecords(): %s", err)
	}

	// Send one more with a different org.
	tc = authtest.NewContext(fs.URL(), env)
	tc.SetOrganization("otherorg")
	tc.SetEnvironment("test")
	ctx = &auth.Context{Context: tc}
	if err := m.SendRecords(ctx, wantRecords[t2][0].records); err != nil {
		t.Errorf("Error on SendRecords(): %s", err)
	}

	// Records are sent async, so we should not have sent any yet.
	if len(fs.Records()) > 0 {
		t.Errorf("Got %d records sent, want 0: %v", len(fs.Records()), fs.Records())
	}

	time.Sleep(100 * time.Millisecond)

	// Should have sent things out by now, check it out.
	if !reflect.DeepEqual(fs.Records(), wantRecords) {
		t.Errorf("got records %v, want records %v", fs.Records(), wantRecords)
	}
}

func TestAuthFailure(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	fs.failAuth = true
	defer fs.Close()

	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.
	m := newManager()
	m.now = func() time.Time { return time.Unix(ts, 0) }
	m.collectionInterval = 50 * time.Millisecond

	records := []Record{{APIProxy: "proxy"}, {APIProduct: "product"}}

	env := adaptertest.NewEnv(t)
	m.log = env.Logger()

	tc := authtest.NewContext(fs.URL(), env)
	tc.SetOrganization("hi")
	tc.SetEnvironment("test")
	ctx := &auth.Context{Context: tc}

	if err := m.SendRecords(ctx, records); err != nil {
		t.Errorf("Error on SendRecords(): %s", err)
	}

	// Records are sent async, so we should not have sent any yet.
	if len(fs.Records()) > 0 {
		t.Errorf("Got %d records sent, want 0: %v", len(fs.Records()), fs.Records())
	}

	if err := m.flush(); err != nil {
		if !strings.Contains(err.Error(), "non-200 status") {
			t.Errorf("unexpected err on flush(): %s", err)
		}
	} else {
		t.Errorf("expected 404 error on flush()")
	}

	// Should have triggered the process by now, but we don't want any records sent.
	if len(fs.Records()) > 0 {
		t.Errorf("Got %d records sent, want 0: %v", len(fs.Records()), fs.Records())
	}
	if fs.failedCalls == 0 {
		t.Errorf("Should have hit signedURL endpoint at least once.")
	}
}
