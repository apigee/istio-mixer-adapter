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
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"github.com/apigee/istio-mixer-adapter/adapter/authtest"
	"go.uber.org/multierr"
	adaptertest "istio.io/istio/mixer/pkg/adapter/test"
)

func TestStagingSizeCap(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	defer fs.Close()

	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.

	workDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(workDir)

	baseURL, _ := url.Parse(fs.URL())
	m, err := newManager(Options{
		BufferPath:       workDir,
		StagingFileLimit: 3,
		BaseURL:          *baseURL,
		Key:              "key",
		Secret:           "secret",
		Client:           http.DefaultClient,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}
	m.now = func() time.Time { return time.Unix(ts, 0) }

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

	t1 := "hi~test"
	tc := authtest.NewContext(fs.URL(), env)
	tc.SetOrganization("hi")
	tc.SetEnvironment("test")
	ctx := &auth.Context{Context: tc}

	for i := 1; i < m.stagingFileLimit+3; i++ {
		if err := m.SendRecords(ctx, records); err != nil {
			t.Errorf("Error on SendRecords(): %s", err)
		}
		m.stageAllBucketsWait()

		if len(fs.Records()) > 0 {
			t.Errorf("Got %d tenants sent, want 0: %v", len(fs.Records()), fs.Records())
		}

		limited := math.Min(float64(m.stagingFileLimit), float64(i))
		wantFileCount(t, m.getTempDir(t1), 0)
		wantFileCount(t, m.getStagingDir(t1), int(limited))
	}

	err = m.uploadAll()
	if err != nil {
		t.Errorf("Error on uploadAll(): %s", err)
	}

	// 1 tenant
	if len(fs.Records()) != 1 {
		t.Errorf("Got %d tenants sent, want 1: %v", len(fs.Records()), fs.Records())
	}

	// actual records
	pushes := fs.Records()["hi~test"]
	if len(pushes) != 3 {
		t.Errorf("Got %d files sent, want %d: %v", len(pushes), 3, pushes)
	}
	for _, push := range pushes {
		recs := push.records
		if len(recs) != 2 {
			t.Errorf("Got %d records sent, want %d: %v", len(recs), 2, recs)
		}
	}
}

func wantFileCount(t *testing.T, path string, want int) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		if want != 0 || err.Error() != "no such file or directory" {
			t.Errorf("ioutil.ReadDir(%s): %s", path, err)
		}
	} else if len(files) != want {
		t.Errorf("got %d file, want %d in %s", len(files), want, path)
	}
}

func TestCrashRecoveryGoodFiles(t *testing.T) {
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
		BufferPath:       d,
		StagingFileLimit: 10,
		BaseURL:          *baseURL,
		Key:              "key",
		Secret:           "secret",
		Client:           http.DefaultClient,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}
	m.env = adaptertest.NewEnv(t)
	m.log = m.env.Logger()

	// Put two files into the temp dir:
	// - a good gzip file
	// - a corrupted but recoverable gzip file
	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.
	rec := Record{
		Organization:                 "hi",
		Environment:                  "test",
		ClientReceivedStartTimestamp: ts * 1000,
		ClientReceivedEndTimestamp:   ts * 1000,
	}

	tenant := "hi~test"
	if err := m.prepTenant(tenant); err != nil {
		t.Fatalf("prepTenant: %v", err)
	}
	tempDir := m.getTempDir(tenant)
	stagingDir := m.getStagingDir(tenant)

	goodFile := path.Join(tempDir, "good.json.gz")
	brokeFile := path.Join(tempDir, "broke.json.gz")

	f, err := os.Create(goodFile)
	if err != nil {
		t.Fatalf("error creating good file: %s", err)
	}
	gz := gzip.NewWriter(f)
	json.NewEncoder(gz).Encode(&rec)
	gz.Close()
	f.Close()

	f, _ = os.Create(brokeFile)
	gz = gzip.NewWriter(f)
	json.NewEncoder(gz).Encode(&rec)
	gz.Close()
	f.WriteString("this is not a json record")
	f.Close()

	if err := m.crashRecovery(); len(multierr.Errors(err)) != 1 {
		t.Fatal("should have had an error for the bad file")
	}

	files, err := ioutil.ReadDir(stagingDir)
	if err != nil {
		t.Fatalf("ls %s: %s", stagingDir, err)
	}

	if len(files) != 2 {
		t.Errorf("got %d files in staging, want 2:", len(files))
		for _, fi := range files {
			t.Log(fi.Name())
		}
	}
	for _, fi := range files {
		// Confirm that it's a valid gzip file.
		f, err := os.Open(path.Join(stagingDir, fi.Name()))
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
