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
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/authtest"
	"github.com/apigee/istio-mixer-adapter/apigee/log"
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

		recs, err := ReadRecords(gz, fs.ignoreBadRecs)
		if err != nil {
			t.Fatalf("Error decoding JSON sent to signed URL: %s", err)
		}

		tenant := r.FormValue("tenant")
		fp := r.FormValue("relative_file_path")
		fs.records[tenant] = append(fs.records[tenant], testRecordPush{
			records: recs,
			dir:     filepath.Dir(fp),
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

func ReadRecords(gz io.Reader, ignoreBadRecs bool) ([]Record, error) {
	var recs []Record
	bio := bufio.NewReader(gz)
	for {
		line, isPrefix, err := bio.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if isPrefix {
			return nil, fmt.Errorf("isPrefix: %v", err)
		}
		r := bytes.NewReader(line)
		var rec Record
		if err := json.NewDecoder(r).Decode(&rec); err != nil {
			if !ignoreBadRecs {
				return nil, fmt.Errorf("Error decoding JSON sent to signed URL: %s", err)
			}
		}
		recs = append(recs, rec)
	}

	return recs, nil
}

func (fs *fakeServer) close() { fs.srv.Close() }
func (fs *fakeServer) pushes() map[string][]testRecordPush {
	fs.lock.RLock()
	defer fs.lock.RUnlock()

	// make copy
	targetMap := make(map[string][]testRecordPush)
	for key, value := range fs.records {
		targetMap[key] = value
	}
	return targetMap
}
func (fs *fakeServer) URL() string { return fs.srv.URL }

func (fs *fakeServer) pushesForTenant(tenant string) []testRecordPush {
	return fs.pushes()[tenant]
}

func (fs *fakeServer) uploadedRecords(tenant string) []Record {
	fs.lock.RLock()
	defer fs.lock.RUnlock()

	var recs []Record
	for _, push := range fs.pushesForTenant(tenant) {
		recs = append(recs, push.records...)
	}
	return recs
}

func TestPushAnalytics(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	defer fs.close()

	t1 := getTenantName("hi", "test")
	t2 := getTenantName("otherorg", "test")
	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.
	now := func() time.Time { return time.Unix(ts, 0) }

	testDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(testDir)

	baseURL, _ := url.Parse(fs.URL())

	uploader := &saasUploader{
		client:  http.DefaultClient,
		baseURL: baseURL,
		key:     "key",
		secret:  "secret",
		now:     now,
	}

	m, err := newManager(uploader, Options{
		BufferPath:         testDir,
		StagingFileLimit:   10,
		now:                now,
		CollectionInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}
	uploadDir := fmt.Sprintf("date=%s/time=%s", now().Format("2006-01-02"), now().Format("15-04-00"))

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

	m.Start()
	defer m.Close()

	tc := authtest.NewContext(fs.URL())
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
	tc = authtest.NewContext(fs.URL())
	tc.SetOrganization("otherorg")
	tc.SetEnvironment("test")
	ctx = &auth.Context{Context: tc}
	if err := m.SendRecords(ctx, sendRecords[t2][0].records); err != nil {
		t.Errorf("Error on SendRecords(): %s", err)
	}

	// Records are sent async, so we should not have sent any yet.
	if len(fs.pushes()) > 0 {
		t.Errorf("Got %d records sent, want 0: %v", len(fs.pushes()), fs.pushes())
	}

	// should upload async without prodding, give it a moment
	time.Sleep(150 * time.Millisecond)

	// Should have sent things out by now, check it out.
	fs.lock.RLock()
	checkAndClearGatewayFlowIDs(fs, t)
	if !reflect.DeepEqual(fs.pushes(), wantRecords) {
		t.Errorf("got records %#v, want records %#v", fs.pushes(), wantRecords)
	}
	fs.lock.RUnlock()

	// Should have deleted everything.
	for _, p := range []string{
		m.getTempDir(t1),
		m.getTempDir(t2),
		m.getStagingDir(t1),
		m.getStagingDir(t2),
	} {
		files, err := ioutil.ReadDir(p)
		if err != nil {
			t.Errorf("ioutil.ReadDir(%s): %s", p, err)
		} else if len(files) > 0 {
			t.Errorf("got %d records on disk, want 0", len(files))
			for _, f := range files {
				t.Log(filepath.Join(testDir, f.Name()))
			}
		}
	}
}

func TestPushAnalyticsMultipleRecords(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	defer fs.close()

	t1 := getTenantName("hi", "test")
	t2 := getTenantName("hi", "test~2")
	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.
	now := func() time.Time { return time.Unix(ts, 0) }

	testDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(testDir)

	baseURL, _ := url.Parse(fs.URL())

	uploader := &saasUploader{
		client:  http.DefaultClient,
		baseURL: baseURL,
		key:     "key",
		secret:  "secret",
		now:     now,
	}

	m, err := newManager(uploader, Options{
		BufferPath:         testDir,
		StagingFileLimit:   10,
		now:                now,
		CollectionInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}
	uploadDir := fmt.Sprintf("date=%s/time=%s", now().Format("2006-01-02"), now().Format("15-04-00"))

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

	m.Start()

	tc := authtest.NewContext(fs.URL())
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
	if len(fs.pushes()) > 0 {
		t.Errorf("Got %d records sent, want 0: %v", len(fs.pushes()), fs.pushes())
	}

	m.Close()

	// Should have sent things out by now, check it out.
	fs.lock.RLock()
	checkAndClearGatewayFlowIDs(fs, t)
	if !reflect.DeepEqual(fs.pushes(), wantRecords) {
		t.Errorf("got records %v, want records %v", fs.pushes(), wantRecords)
	}
	fs.lock.RUnlock()

	// Should have deleted everything.
	for _, p := range []string{
		m.getTempDir(t1),
		m.getStagingDir(t1),
	} {
		files, err := ioutil.ReadDir(p)
		if err != nil {
			t.Errorf("ioutil.ReadDir(%s): %s", p, err)
		} else if len(files) > 0 {
			t.Errorf("got %d records on disk, want 0", len(files))
			for _, f := range files {
				t.Log(filepath.Join(testDir, f.Name()))
			}
		}
	}
}

func TestLoad(t *testing.T) {
	t.Parallel()

	const SendRecs = 100

	fs := newFakeServer(t)
	defer fs.close()

	t1 := "load~test"
	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.
	now := func() time.Time { return time.Unix(ts, 0) }

	testDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(testDir)

	baseURL, _ := url.Parse(fs.URL())

	uploader := &saasUploader{
		client:  http.DefaultClient,
		baseURL: baseURL,
		key:     "key",
		secret:  "secret",
		now:     now,
	}

	m, err := newManager(uploader, Options{
		BufferPath:         testDir,
		StagingFileLimit:   10,
		now:                now,
		CollectionInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}
	uploadDir := fmt.Sprintf("date=%s/time=%s", now().Format("2006-01-02"), now().Format("15-04-00"))

	record := Record{
		Organization:                 "load",
		Environment:                  "test",
		ClientReceivedStartTimestamp: ts * 1000,
		ClientReceivedEndTimestamp:   ts * 1000,
		APIProxy:                     "proxy",
	}

	sendRecords := map[string][]testRecordPush{
		t1: {{
			records: []Record{record},
			dir:     uploadDir,
		}},
	}

	m.Start()

	tc := authtest.NewContext(fs.URL())
	tc.SetOrganization("load")
	tc.SetEnvironment("test")
	ctx := &auth.Context{Context: tc}

	for i := 0; i < SendRecs; i++ {
		recs := sendRecords[t1][0].records
		recs[0].APIProxyRevision = i
		if err := m.SendRecords(ctx, sendRecords[t1][0].records); err != nil {
			t.Fatalf("SendRecords(): %s", err)
		}
		if i%50 == 0 {
			t.Log("stageAllBucketsWait")
			m.stageAllBucketsWait()
		}
	}

	m.Close()

	// Should have sent things out by now, check it out.
	fs.lock.RLock()
	checkAndClearGatewayFlowIDs(fs, t)
	pushes := fs.pushes()["load~test"]
	receivedRecs := []Record{}
	for _, push := range pushes {
		receivedRecs = append(receivedRecs, push.records...)
	}
	if len(receivedRecs) != SendRecs {
		t.Errorf("got %d records, want %d records", len(receivedRecs), SendRecs)
		for _, r := range receivedRecs {
			t.Errorf("record: %v", r)
			// r.APIProxyRevision
		}
		// t.Errorf("records: %v", receivedRecs)
	}
	fs.lock.RUnlock()

	// Should have deleted everything.
	for _, p := range []string{
		m.getTempDir(t1),
		m.getStagingDir(t1),
	} {
		files, err := ioutil.ReadDir(p)
		if err != nil {
			t.Errorf("ioutil.ReadDir(%s): %s", p, err)
		} else if len(files) > 0 {
			t.Errorf("got %d records on disk, want 0", len(files))
			for _, f := range files {
				t.Log(filepath.Join(testDir, f.Name()))
			}
		}
	}
}

func checkAndClearGatewayFlowIDs(fs *fakeServer, t *testing.T) {
	for tid, recs := range fs.pushes() {
		for i, trp := range recs {
			for j, rec := range trp.records {
				if rec.GatewayFlowID == "" {
					t.Errorf("gateway_flow_id not set on record %#v", rec)
				}
				fs.pushes()[tid][i].records[j].GatewayFlowID = ""
				rec.GatewayFlowID = "" // clear for DeepEqual check
			}
		}
	}
}

func TestUploadFailure(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	fs.failUpload = http.StatusInternalServerError
	defer fs.close()

	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(d)

	baseURL, _ := url.Parse(fs.URL())
	now := func() time.Time { return time.Unix(ts, 0) }

	uploader := &saasUploader{
		client:  http.DefaultClient,
		baseURL: baseURL,
		key:     "key",
		secret:  "secret",
		now:     now,
	}

	m, err := newManager(uploader, Options{
		BufferPath:         d,
		StagingFileLimit:   10,
		SendChannelSize:    0,
		now:                now,
		CollectionInterval: time.Minute,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}

	records := []Record{
		{
			Organization:                 "hi",
			Environment:                  "test",
			ClientReceivedStartTimestamp: ts * 1000,
			ClientReceivedEndTimestamp:   ts * 1000,
			APIProxy:                     "proxy",
		},
		{
			Organization:                 "hi",
			Environment:                  "test",
			ClientReceivedStartTimestamp: ts * 1000,
			ClientReceivedEndTimestamp:   ts * 1000,
			APIProduct:                   "product",
		},
	}

	t1 := "hi~test"
	tc := authtest.NewContext(fs.URL())
	tc.SetOrganization("hi")
	tc.SetEnvironment("test")
	authCtx := &auth.Context{Context: tc}

	// since we're using a custom errorHandler we can't call m.Start() and need to do this setup
	var uploadError error
	errH := func(err error) error {
		log.Errorf("errH: %v", err)
		uploadError = err
		return err
	}
	m.startUploader(errH)
	go m.stagingLoop()

	if err := m.SendRecords(authCtx, records); err != nil {
		t.Errorf("Error on SendRecords(): %s", err)
	}

	// Records are sent async, so we should not have sent any yet.
	if len(fs.pushes()) > 0 {
		t.Errorf("Got %d records sent, want 0: %v", len(fs.pushes()), fs.pushes())
	}

	m.Close()

	if uploadError != nil {
		if !strings.Contains(uploadError.Error(), "500 Internal Server Error") {
			t.Errorf("unexpected err on upload(): %s", err)
		}
	} else {
		t.Errorf("expected 500 error on upload()")
	}

	// Should have triggered the process by now, but we don't want any records sent.
	if len(fs.pushes()) > 0 {
		t.Errorf("Got %d records sent, want 0: %v", len(fs.pushes()), fs.pushes())
	}
	if fs.failedCalls == 0 {
		t.Errorf("Should have hit signedURL endpoint at least once.")
	}

	// All the files should still be there.
	for p, wantCount := range map[string]int{
		m.getTempDir(t1):    0,
		m.getStagingDir(t1): 1,
	} {
		files, err := ioutil.ReadDir(p)
		if err != nil {
			t.Errorf("ioutil.ReadDir(%s): %s", p, err)
		} else if len(files) != wantCount {
			t.Errorf("got %d records on disk, want %d", len(files), wantCount)
		}
	}
}
