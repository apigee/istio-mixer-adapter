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
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"github.com/apigee/istio-mixer-adapter/adapter/authtest"
	adaptertest "istio.io/istio/mixer/pkg/adapter/test"
)

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

	baseURL, _ := url.Parse(fs.URL())
	m, err := newManager(Options{
		BufferPath: d,
		BufferSize: 10,
		BaseURL:    *baseURL,
		Key:        "key",
		Secret:     "secret",
		Client:     http.DefaultClient,
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
	m.env = env
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

	time.Sleep(time.Millisecond) // give time for file creation
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

	baseURL, _ := url.Parse(fs.URL())
	m, err := newManager(Options{
		BufferPath: d,
		BufferSize: 10,
		BaseURL:    *baseURL,
		Key:        "key",
		Secret:     "secret",
		Client:     http.DefaultClient,
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
	m.env = env
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

	time.Sleep(time.Millisecond) // give time for file creation
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

func TestShortCircuit(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	defer fs.Close()

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(d)

	baseURL, _ := url.Parse(fs.URL())
	m, err := newManager(Options{
		BufferPath: d,
		BufferSize: 10,
		BaseURL:    *baseURL,
		Key:        "key",
		Secret:     "secret",
		Client:     http.DefaultClient,
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

	time.Sleep(time.Millisecond) // give time for file creation
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

func TestNoUploadBadFiles(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	defer fs.Close()
	fs.ignoreBadRecs = true

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(d)
	baseURL, _ := url.Parse(fs.URL())
	m, err := newManager(Options{
		BufferPath: d,
		BufferSize: 10,
		BaseURL:    *baseURL,
		Key:        "key",
		Secret:     "secret",
		Client:     http.DefaultClient,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}
	m.env = adaptertest.NewEnv(t)
	m.log = m.env.Logger()

	// Put two files into the stage dir:
	// - a good gzip file
	// - an unrecoverable file
	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.
	rec := Record{
		Organization:                 "hi",
		Environment:                  "test",
		ClientReceivedStartTimestamp: ts * 1000,
		ClientReceivedEndTimestamp:   ts * 1000,
	}

	stageDir := path.Join(m.stagingDir, "hi~test")
	if err := os.MkdirAll(stageDir, 0700); err != nil {
		t.Fatal(err)
	}

	goodFile := path.Join(stageDir, "good.json.gz")
	brokeFile := path.Join(stageDir, "broke.json.gz")

	f, err := os.Create(goodFile)
	if err != nil {
		t.Fatalf("error creating good file: %s", err)
	}
	gz := gzip.NewWriter(f)
	json.NewEncoder(gz).Encode(&rec)
	gz.Close()
	f.Close()

	f, _ = os.Create(brokeFile)
	f.WriteString("this is not a json record")
	f.Close()

	err = m.uploadAll()
	if err == nil {
		t.Errorf("got nil error, want one")
	}

	if len(fs.Records()) != 1 {
		t.Errorf("Got %d records sent, want 1: %v", len(fs.Records()), fs.Records())
	}
	if fs.failedCalls != 0 {
		t.Errorf("Should not fail, failed: %d", fs.failedCalls)
	}

	files, err := ioutil.ReadDir(stageDir)
	if err != nil {
		t.Fatalf("ls %s: %s", stageDir, err)
	}

	if len(files) != 0 {
		t.Errorf("got %d files in staging, want 1:", len(files))
		for _, fi := range files {
			t.Log(fi.Name())
		}
	}
}
