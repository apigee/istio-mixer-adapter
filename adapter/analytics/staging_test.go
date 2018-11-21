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
	"reflect"
	"sort"
	"strings"
	"testing"

	"go.uber.org/multierr"
	adaptertest "istio.io/istio/mixer/pkg/adapter/test"
)

func TestStagingSizeCap(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	// Let's pretend Apigee is down, so we end up backlogging.
	fs.failUpload = http.StatusInternalServerError
	defer fs.Close()

	for _, test := range []struct {
		desc     string
		files    map[string][]string
		wantStag []string
	}{
		{"too many files, clean some up",
			map[string][]string{
				tempDir:    {"6", "7"},
				stagingDir: {"1", "2", "3", "4"},
			},
			[]string{"3", "4", "6", "7"},
		},
		{"under limit, don't delete",
			map[string][]string{
				tempDir:    {"6", "7"},
				stagingDir: {"1", "2"},
			},
			[]string{"1", "2", "6", "7"},
		},
		{"clean up even if nothing in temp",
			map[string][]string{
				tempDir:    {},
				stagingDir: {"1", "2", "3", "4", "5"},
			},
			[]string{"2", "3", "4", "5"},
		},
	} {
		t.Log(test.desc)

		d, err := ioutil.TempDir("", "")
		if err != nil {
			t.Fatalf("ioutil.TempDir(): %s", err)
		}
		defer os.RemoveAll(d)

		baseURL, _ := url.Parse(fs.URL())
		m, err := newManager(Options{
			BufferPath:       d,
			StagingFileLimit: 4,
			BaseURL:          *baseURL,
			Key:              "key",
			Secret:           "secret",
			Client:           http.DefaultClient,
		})
		if err != nil {
			t.Fatalf("newManager: %s", err)
		}
		m.log = adaptertest.NewEnv(t).Logger()

		// Add a bunch of files in staging and temp, and then try to commit.
		ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.
		rec := Record{
			Organization:                 "hi",
			Environment:                  "test",
			ClientReceivedStartTimestamp: ts * 1000,
			ClientReceivedEndTimestamp:   ts * 1000,
			APIProxy:                     "proxy",
		}
		for dir, files := range test.files {
			p := path.Join(d, dir, "hi~test")
			if err := os.MkdirAll(p, 0700); err != nil {
				t.Fatalf("mkdir %s: %s", p, err)
			}
			for _, f := range files {
				f, err := os.Create(path.Join(p, fmt.Sprintf("%s.json.gz", f)))
				if err != nil {
					t.Fatalf("unexpected error on create: %s", err)
				}

				gz := gzip.NewWriter(f)
				json.NewEncoder(gz).Encode([]Record{rec})
				gz.Close()
				f.Close()
			}
		}

		if err = m.uploadAll(); err == nil {
			t.Fatalf("got nil error on upload, want one")
		} else if !strings.Contains(err.Error(), "500 Internal Server Error") {
			t.Fatalf("got error %s, want 500", err)
		}

		// Confirm that the files we want are in staging.
		var got []string
		fis, err := ioutil.ReadDir(path.Join(m.stagingDir, "hi~test"))
		if err != nil {
			t.Fatalf("ReadDir(%s): %s", m.stagingDir, err)
		}
		for _, fi := range fis {
			got = append(got, strings.TrimSuffix(fi.Name(), ".json.gz"))
		}

		// Should delete the oldest ones: 1 and 2.
		sort.Strings(got)
		if !reflect.DeepEqual(got, test.wantStag) {
			t.Errorf("got staging files %v, want %v", got, test.wantStag)
		}
	}
}

func TestCrashRecoveryInvalidFiles(t *testing.T) {
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
	// - an unrecoverable file
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
	gz.Close()
	f.Close()

	f, _ = os.Create(brokeFile)
	f.WriteString("this is not a json record")
	f.Close()

	if err := m.crashRecovery(); len(multierr.Errors(err)) != 1 {
		t.Fatal("should have had an error for the bad file")
	}

	files, err := ioutil.ReadDir(targetBucket)
	if err != nil {
		t.Fatalf("ls %s: %s", targetBucket, err)
	}

	if len(files) != 1 {
		t.Errorf("got %d files in staging, want 1:", len(files))
		for _, fi := range files {
			t.Log(fi.Name())
		}
	}
	for _, fi := range files {
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
