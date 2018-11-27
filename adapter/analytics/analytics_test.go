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
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"github.com/apigee/istio-mixer-adapter/adapter/authtest"
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
	records       map[string][]testRecordPush
	srv           *httptest.Server
	failAuth      func() int
	failUpload    int
	failedCalls   int
	lock          sync.RWMutex
	ignoreBadRecs bool
}

func newFakeServer(t *testing.T) *fakeServer {
	fs := &fakeServer{
		records: map[string][]testRecordPush{},
	}
	fs.failAuth = func() int { return 0 }
	fs.srv = httptest.NewServer(fs.handler(t))
	return fs
}

func (fs *fakeServer) handler(t *testing.T) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/analytics/", func(w http.ResponseWriter, r *http.Request) {
		fs.lock.Lock()
		defer fs.lock.Unlock()
		if c := fs.failAuth(); c != 0 {
			fs.failedCalls++
			w.WriteHeader(c)
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
		fs.lock.Lock()
		defer fs.lock.Unlock()
		if fs.failUpload != 0 {
			fs.failedCalls++
			w.WriteHeader(fs.failUpload)
			return
		}
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatalf("Error on gzip.NewReader: %s", err)
		}
		defer gz.Close()
		defer r.Body.Close()

		var recs []Record
		bio := bufio.NewReader(gz)
		for {
			line, isPrefix, err := bio.ReadLine()
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("ReadLine: %v", err)
			}
			if isPrefix {
				t.Fatalf("isPrefix: %v", err)
			}
			r := bytes.NewReader(line)
			var rec Record
			if err := json.NewDecoder(r).Decode(&rec); err != nil {
				if !fs.ignoreBadRecs {
					t.Fatalf("Error decoding JSON sent to signed URL: %s", err)
					continue
				}
			}
			recs = append(recs, rec)
		}

		tenant := r.FormValue("tenant")
		fp := r.FormValue("relative_file_path")
		fs.records[tenant] = append(fs.records[tenant], testRecordPush{
			records: recs,
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

func (fs *fakeServer) Close() { fs.srv.Close() }
func (fs *fakeServer) Records() map[string][]testRecordPush {
	fs.lock.RLock()
	defer fs.lock.RUnlock()

	// copy
	targetMap := make(map[string][]testRecordPush)
	for key, value := range fs.records {
		targetMap[key] = value
	}
	return targetMap
}
func (fs *fakeServer) URL() string { return fs.srv.URL }

func TestPushAnalytics(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	defer fs.Close()

	t1 := "hi~test"
	t2 := "otherorg~test"
	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(d)

	// Use a subdirectory to ensure that we can set up the directory properly.
	bufferPath := path.Join(d, "subdir")

	baseURL, _ := url.Parse(fs.URL())
	m, err := newManager(Options{
		BufferPath:       bufferPath,
		StagingFileLimit: 10,
		BaseURL:          *baseURL,
		Key:              "key",
		Secret:           "secret",
		Client:           http.DefaultClient,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}
	m.now = func() time.Time { return time.Unix(ts, 0) }
	m.collectionInterval = 100 * time.Millisecond
	uploadDir := fmt.Sprintf("date=%s/time=%s", m.now().Format("2006-01-02"), m.now().Format("15:04:00"))

	sendRecords := map[string][]testRecordPush{
		t1: {
			{
				records: []Record{
					{
						Organization:                 "hi",
						Environment:                  "test",
						ClientReceivedStartTimestamp: ts * 1000,
						ClientReceivedEndTimestamp:   ts * 1000,
						APIProxy:                     "proxy",
					},
					{
						ClientReceivedStartTimestamp: ts * 1000,
						ClientReceivedEndTimestamp:   ts * 1000,
						APIProduct:                   "product",
					},
				},
				dir: uploadDir,
			},
		},
		t2: {
			{
				records: []Record{
					{
						Organization:                 "otherorg",
						Environment:                  "test",
						ClientReceivedStartTimestamp: ts * 1000,
						ClientReceivedEndTimestamp:   ts * 1000,
						RequestURI:                   "request URI",
					},
				},
				dir: uploadDir,
			},
		},
	}

	wantRecords := map[string][]testRecordPush{
		t1: {
			{
				records: []Record{
					{
						RecordType:                   "APIAnalytics",
						Organization:                 "hi",
						Environment:                  "test",
						ClientReceivedStartTimestamp: ts * 1000,
						ClientReceivedEndTimestamp:   ts * 1000,
						APIProxy:                     "proxy",
					},
					{
						RecordType:                   "APIAnalytics",
						Organization:                 "hi",
						Environment:                  "test",
						ClientReceivedStartTimestamp: ts * 1000,
						ClientReceivedEndTimestamp:   ts * 1000,
						APIProduct:                   "product",
					},
				},
				dir: uploadDir,
			},
		},
		t2: {
			{
				records: []Record{
					{
						RecordType:                   "APIAnalytics",
						Organization:                 "otherorg",
						Environment:                  "test",
						ClientReceivedStartTimestamp: ts * 1000,
						ClientReceivedEndTimestamp:   ts * 1000,
						RequestURI:                   "request URI",
					},
				},
				dir: uploadDir,
			},
		},
	}

	env := adaptertest.NewEnv(t)
	m.Start(env)

	tc := authtest.NewContext(fs.URL(), env)
	tc.SetOrganization("hi")
	tc.SetEnvironment("test")
	ctx := &auth.Context{Context: tc}

	if err := m.SendRecords(ctx, sendRecords[t1][0].records); err != nil {
		t.Errorf("Error on SendRecords(): %s", err)
	}

	// Send an invalid record
	if err := m.SendRecords(ctx, []Record{{}}); err != nil {
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

	m.Close()

	// Should have sent things out by now, check it out.
	fs.lock.RLock()
	if !reflect.DeepEqual(fs.Records(), wantRecords) {
		t.Errorf("got records %#v, want records %#v", fs.Records(), wantRecords)
	}
	fs.lock.RUnlock()

	// Should have deleted everything.
	for _, p := range []string{
		"/temp/hi~test/",
		"/temp/otherorg~test/",
		"/staging/hi~test/",
		"/staging/otherorg~test/",
	} {
		files, err := ioutil.ReadDir(bufferPath + p)
		if err != nil {
			t.Errorf("ioutil.ReadDir(%s): %s", p, err)
		} else if len(files) > 0 {
			t.Errorf("got %d records on disk, want 0", len(files))
			for _, f := range files {
				t.Log(path.Join(bufferPath, f.Name()))
			}
		}
	}
}

func TestPushAnalyticsMultipleRecords(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	defer fs.Close()

	t1 := "hi~test"
	t2 := "hi~test~2"
	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(d)

	// Use a subdirectory to ensure that we can set up the directory properly.
	bufferPath := path.Join(d, "subdir")

	baseURL, _ := url.Parse(fs.URL())
	m, err := newManager(Options{
		BufferPath:       bufferPath,
		StagingFileLimit: 10,
		BaseURL:          *baseURL,
		Key:              "key",
		Secret:           "secret",
		Client:           http.DefaultClient,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}
	m.now = func() time.Time { return time.Unix(ts, 0) }
	m.collectionInterval = 100 * time.Millisecond
	uploadDir := fmt.Sprintf("date=%s/time=%s", m.now().Format("2006-01-02"), m.now().Format("15:04:00"))

	sendRecords := map[string][]testRecordPush{
		t1: {{
			records: []Record{
				{
					Organization:                 "hi",
					Environment:                  "test",
					ClientReceivedStartTimestamp: ts * 1000,
					ClientReceivedEndTimestamp:   ts * 1000,
					APIProxy:                     "proxy",
				},
				{
					ClientReceivedStartTimestamp: ts * 1000,
					ClientReceivedEndTimestamp:   ts * 1000,
					APIProduct:                   "product",
				},
			},
			dir: uploadDir,
		}},
		t2: {{
			records: []Record{
				{
					Organization:                 "hi",
					Environment:                  "test",
					ClientReceivedStartTimestamp: ts * 1000,
					ClientReceivedEndTimestamp:   ts * 1000,
					RequestURI:                   "request URI",
				},
			},
			dir: uploadDir,
		}},
	}

	wantRecords := map[string][]testRecordPush{
		t1: {{
			records: []Record{
				{
					RecordType:                   "APIAnalytics",
					Organization:                 "hi",
					Environment:                  "test",
					ClientReceivedStartTimestamp: ts * 1000,
					ClientReceivedEndTimestamp:   ts * 1000,
					APIProxy:                     "proxy",
				},
				{
					RecordType:                   "APIAnalytics",
					Organization:                 "hi",
					Environment:                  "test",
					ClientReceivedStartTimestamp: ts * 1000,
					ClientReceivedEndTimestamp:   ts * 1000,
					APIProduct:                   "product",
				},
				{
					RecordType:                   "APIAnalytics",
					Organization:                 "hi",
					Environment:                  "test",
					ClientReceivedStartTimestamp: ts * 1000,
					ClientReceivedEndTimestamp:   ts * 1000,
					RequestURI:                   "request URI",
				},
			},
			dir: uploadDir,
		}},
	}

	env := adaptertest.NewEnv(t)
	m.Start(env)

	tc := authtest.NewContext(fs.URL(), env)
	tc.SetOrganization("hi")
	tc.SetEnvironment("test")
	ctx := &auth.Context{Context: tc}

	if err := m.SendRecords(ctx, sendRecords[t1][0].records); err != nil {
		t.Errorf("Error on SendRecords(): %s", err)
	}

	// Send one more with same org
	if err := m.SendRecords(ctx, sendRecords[t2][0].records); err != nil {
		t.Errorf("Error on SendRecords(): %s", err)
	}

	// Records are sent async, so we should not have sent any yet.
	if len(fs.Records()) > 0 {
		t.Errorf("Got %d records sent, want 0: %v", len(fs.Records()), fs.Records())
	}

	m.Close()

	// Should have sent things out by now, check it out.
	fs.lock.RLock()
	if !reflect.DeepEqual(fs.Records(), wantRecords) {
		t.Errorf("got records %v, want records %v", fs.Records(), wantRecords)
	}
	fs.lock.RUnlock()

	// Should have deleted everything.
	for _, p := range []string{
		"/temp/hi~test/",
		"/staging/hi~test/",
	} {
		files, err := ioutil.ReadDir(bufferPath + p)
		if err != nil {
			t.Errorf("ioutil.ReadDir(%s): %s", p, err)
		} else if len(files) > 0 {
			t.Errorf("got %d records on disk, want 0", len(files))
			for _, f := range files {
				t.Log(path.Join(bufferPath, f.Name()))
			}
		}
	}
}
