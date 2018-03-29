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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
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
	failAuth    func() int
	failUpload  int
	failedCalls int
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

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(d)

	// Use a subdirectory to ensure that we can set up the directory properly.
	bufferPath := path.Join(d, "subdir")

	m, err := newManager(Options{
		BufferPath: bufferPath,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}
	m.now = func() time.Time { return time.Unix(ts, 0) }
	m.collectionInterval = 50 * time.Millisecond

	wantRecords := map[string][]testRecordPush{
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
						Organization:                 "hi",
						Environment:                  "test",
						ClientReceivedStartTimestamp: ts * 1000,
						ClientReceivedEndTimestamp:   ts * 1000,
						APIProduct:                   "product",
					},
				},
				dir: fmt.Sprintf("date=2018-03-16/time=%d-%d", ts, ts),
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
				dir: fmt.Sprintf("date=2018-03-16/time=%d-%d", ts, ts),
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

	if err := m.SendRecords(ctx, wantRecords[t1][0].records); err != nil {
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

func TestAuthFailure(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	fs.failAuth = func() int { return http.StatusUnauthorized }
	defer fs.Close()

	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(d)

	m, err := newManager(Options{
		BufferPath: d,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}
	m.now = func() time.Time { return time.Unix(ts, 0) }
	m.collectionInterval = 50 * time.Millisecond

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

	if err := m.uploadAll(); err != nil {
		if !strings.Contains(err.Error(), "code 401") {
			t.Errorf("unexpected err on upload(): %s", err)
		}
	} else {
		t.Errorf("expected 401 error on upload()")
	}

	// Should have triggered the process by now, but we don't want any records sent.
	if len(fs.Records()) > 0 {
		t.Errorf("Got %d records sent, want 0: %v", len(fs.Records()), fs.Records())
	}
	if fs.failedCalls == 0 {
		t.Errorf("Should have hit signedURL endpoint at least once.")
	}

	// All the files should still be there.
	for p, wantCount := range map[string]int{
		"/temp/hi~test/":    0,
		"/staging/hi~test/": 1,
	} {
		files, err := ioutil.ReadDir(d + p)
		if err != nil {
			t.Errorf("ioutil.ReadDir(%s): %s", d, err)
		} else if len(files) != wantCount {
			t.Errorf("got %d records on disk, want %d", len(files), wantCount)
		}
	}
}

func TestUploadFailure(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	fs.failUpload = http.StatusInternalServerError
	defer fs.Close()

	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(d)

	m, err := newManager(Options{
		BufferPath: d,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}
	m.now = func() time.Time { return time.Unix(ts, 0) }
	m.collectionInterval = 50 * time.Millisecond

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

	if err := m.uploadAll(); err != nil {
		if !strings.Contains(err.Error(), "500 Internal Server Error") {
			t.Errorf("unexpected err on upload(): %s", err)
		}
	} else {
		t.Errorf("expected 500 error on upload()")
	}

	// Should have triggered the process by now, but we don't want any records sent.
	if len(fs.Records()) > 0 {
		t.Errorf("Got %d records sent, want 0: %v", len(fs.Records()), fs.Records())
	}
	if fs.failedCalls == 0 {
		t.Errorf("Should have hit signedURL endpoint at least once.")
	}

	// All the files should still be there.
	for p, wantCount := range map[string]int{
		"/temp/hi~test/":    0,
		"/staging/hi~test/": 1,
	} {
		files, err := ioutil.ReadDir(d + p)
		if err != nil {
			t.Errorf("ioutil.ReadDir(%s): %s", d, err)
		} else if len(files) != wantCount {
			t.Errorf("got %d records on disk, want %d", len(files), wantCount)
		}
	}
}

func TestValidationFailure(t *testing.T) {
	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.
	for _, test := range []struct {
		desc      string
		record    Record
		wantError string
	}{
		{"good record", Record{
			Organization:                 "hi",
			Environment:                  "test",
			ClientReceivedStartTimestamp: ts * 1000,
			ClientReceivedEndTimestamp:   ts * 1000,
		}, ""},
		{"missing org", Record{
			Environment:                  "test",
			ClientReceivedStartTimestamp: ts * 1000,
			ClientReceivedEndTimestamp:   ts * 1000,
		}, "missing Organization"},
		{"missing env", Record{
			Organization:                 "hi",
			ClientReceivedStartTimestamp: ts * 1000,
			ClientReceivedEndTimestamp:   ts * 1000,
		}, "missing Environment"},
		{"missing start timestamp", Record{
			Organization:               "hi",
			Environment:                "test",
			ClientReceivedEndTimestamp: ts * 1000,
		}, "missing ClientReceivedStartTimestamp"},
		{"missing end timestamp", Record{
			Organization:                 "hi",
			Environment:                  "test",
			ClientReceivedStartTimestamp: ts * 1000,
		}, "missing ClientReceivedEndTimestamp"},
		{"end < start", Record{
			Organization:                 "hi",
			Environment:                  "test",
			ClientReceivedStartTimestamp: ts * 1000,
			ClientReceivedEndTimestamp:   ts*1000 - 1,
		}, "ClientReceivedStartTimestamp > ClientReceivedEndTimestamp"},
		{"in the future", Record{
			Organization:                 "hi",
			Environment:                  "test",
			ClientReceivedStartTimestamp: (ts + 1) * 1000,
			ClientReceivedEndTimestamp:   (ts + 1) * 1000,
		}, "in the future"},
		{"too old", Record{
			Organization:                 "hi",
			Environment:                  "test",
			ClientReceivedStartTimestamp: (ts - 91*24*3600) * 1000,
			ClientReceivedEndTimestamp:   (ts - 91*24*3600) * 1000,
		}, "more than 90 days old"},
	} {
		t.Log(test.desc)

		m := Manager{}
		m.now = func() time.Time { return time.Unix(ts, 0) }

		gotErr := m.validate(test.record)
		if test.wantError == "" {
			if gotErr != nil {
				t.Errorf("got error %s, want none", gotErr)
			}
			continue
		}
		if gotErr == nil {
			t.Errorf("got nil error, want one containing %s", test.wantError)
			continue
		}

		if !strings.Contains(gotErr.Error(), test.wantError) {
			t.Errorf("error %s should contain '%s'", gotErr, test.wantError)
		}
	}
}

func TestCrashRecovery(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	defer fs.Close()

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(d)

	m, err := newManager(Options{
		BufferPath: d,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}
	// Set logger so we can log things while debugging.
	env := adaptertest.NewEnv(t)
	m.log = env.Logger()

	// Put two files into the temp dir:
	// - a good gzip file
	// - a corrupted gzip file
	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.
	rec := Record{
		Organization:                 "hi",
		Environment:                  "test",
		ClientReceivedStartTimestamp: ts * 1000,
		ClientReceivedEndTimestamp:   ts * 1000,
	}

	bucket := path.Join(m.tempDir, "hi~test")
	targetBucket := path.Join(m.stagingDir, "hi~test")
	if err := os.MkdirAll(bucket, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetBucket, 0700); err != nil {
		t.Fatal(err)
	}

	goodFile := path.Join(bucket, "good.json.gz")
	brokeFile := path.Join(bucket, "broke.json.gz")

	f, err := os.Create(goodFile)
	if err != nil {
		t.Fatalf("error creating good file: %s", err)
	}
	gz := gzip.NewWriter(f)
	json.NewEncoder(gz).Encode(&rec)
	gz.Flush()
	gz.Close()
	f.Close()

	f, _ = os.Create(brokeFile)
	gz = gzip.NewWriter(f)
	json.NewEncoder(gz).Encode(&rec)
	// Close the file, but don't close the gzip to ensure it's broken.
	gz.Flush()
	f.Close()

	// Now attempt recovery.
	if err := m.crashRecovery(); err != nil {
		t.Fatal(err)
	}

	// We should have two proper gzip files in the staging dir.
	files, err := ioutil.ReadDir(targetBucket)
	if err != nil {
		t.Fatalf("ls %s: %s", targetBucket, err)
	}

	if len(files) != 2 {
		t.Errorf("got %d files in staging, want 2:", len(files))
		for _, fi := range files {
			t.Log(fi.Name())
		}
	}
	for _, fi := range files {
		// Confirm that it's a valid gzip file.
		f, err := os.Open(path.Join(targetBucket, fi.Name()))
		if err != nil {
			t.Fatalf("error opening %s: %s", fi.Name(), err)
		}
		gz, err := gzip.NewReader(f)
		if err != nil {
			t.Errorf("gzip error %s: %s", fi.Name(), err)
		}
		var got Record
		if err := json.NewDecoder(gz).Decode(&got); err != nil {
			t.Errorf("json decode error %s: %s", fi.Name(), err)
		}

		if got != rec {
			t.Errorf("file %s: got %v, want %v", fi.Name(), got, rec)
		}
	}
}

func TestShortCircuit(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	defer fs.Close()

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(d)

	m, err := newManager(Options{
		BufferPath: d,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}

	// Test plan: create 10 files containing one record. The first upload attempt
	// will return a non-short-circuit error, all after that will return an auth
	// failure (which short-circuits).
	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.
	rec := Record{
		Organization:                 "hi",
		Environment:                  "test",
		ClientReceivedStartTimestamp: ts * 1000,
		ClientReceivedEndTimestamp:   ts * 1000,
		APIProxy:                     "proxy",
	}
	callCount := 10

	m.log = adaptertest.NewEnv(t).Logger()
	m.creds.Store("hi~test", cred{"key", "secret", fs.URL()})

	p := path.Join(d, "temp/hi~test")
	if err := os.MkdirAll(p, 0777); err != nil {
		t.Fatalf("could not create temp dir: %s", err)
	}

	for i := 0; i < callCount; i++ {
		f, err := os.Create(path.Join(p, fmt.Sprintf("%d.json.gz", i)))
		if err != nil {
			t.Fatalf("unexpected error on create: %s", err)
		}

		gz := gzip.NewWriter(f)
		json.NewEncoder(gz).Encode([]Record{rec})
		gz.Close()
		f.Close()
	}

	fs.failAuth = func() int {
		fs.failAuth = func() int { return http.StatusUnauthorized }
		return http.StatusTeapot
	}
	err = m.uploadAll()
	if err == nil {
		t.Errorf("got nil error, want one")
	} else if !strings.Contains(err.Error(), "code 401") && !strings.Contains(err.Error(), "code 418") {
		t.Errorf("got error %s on upload, want 401/418", err)
	}

	// We should not have sent any records because of auth failures.
	if len(fs.Records()) > 0 {
		t.Errorf("Got %d records sent, want 0: %v", len(fs.Records()), fs.Records())
	}
	if fs.failedCalls != 2 {
		t.Errorf("Should hit signedURL endpoint exactly twice")
	}

	// All the files should be sitting in staging.
	for p, wantCount := range map[string]int{
		"/temp/hi~test/":    0,
		"/staging/hi~test/": callCount,
	} {
		files, err := ioutil.ReadDir(d + p)
		if err != nil {
			t.Errorf("ioutil.ReadDir(%s): %s", d, err)
		} else if len(files) != wantCount {
			t.Errorf("got %d records on disk, want %d", len(files), wantCount)
		}
	}
}
