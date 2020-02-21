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
	"path/filepath"
	"testing"
	"time"
)

func TestRecoverFile(t *testing.T) {
	t.Parallel()

	brokeFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("ioutil.TempFile(): %v", err)
	}

	rec := Record{
		Organization: "hi",
		Environment:  "test",
	}

	gzWriter := gzip.NewWriter(brokeFile)
	if err := json.NewEncoder(gzWriter).Encode(&rec); err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("gz.Close(): %v", err)
	}
	if _, err := brokeFile.WriteString("this is not a json record"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := brokeFile.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	_, err = readRecordsFromGZipFile(brokeFile.Name())
	if err == nil {
		t.Fatalf("file should be bad")
	}

	// repair
	m := manager{}
	fixedFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("ioutil.TempFile(): %v", err)
	}
	m.recoverFile(brokeFile.Name(), fixedFile)

	// test repair
	recs, err := readRecordsFromGZipFile(fixedFile.Name())
	if err != nil {
		t.Fatalf("ReadRecords %s: %s", brokeFile.Name(), err)
	}
	if len(recs) != 1 {
		t.Fatalf("want 1, got %d: %v", len(recs), recs)
	}
	got := recs[0]
	if got != rec {
		t.Errorf("file %s: got %v, want %v", brokeFile.Name(), got, rec)
	}
}

func readRecordsFromGZipFile(fileName string) ([]Record, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("Open(): %v", err)
	}
	defer file.Close()
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("gzip error %s: %s", file.Name(), err)
	}
	defer gzReader.Close()

	return ReadRecords(gzReader, false)
}

func TestCrashRecovery(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	fs.failUpload = http.StatusInternalServerError
	defer fs.close()

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(d)
	baseURL, _ := url.Parse(fs.URL())
	now := time.Now

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
		now:                now,
		CollectionInterval: time.Minute,
	})
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}

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

	goodFile := filepath.Join(tempDir, "good.json.gz")
	brokeFile := filepath.Join(tempDir, "broke.json.gz")
	stagedFile := filepath.Join(stagingDir, "staged.gz")

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

	f, err = os.Create(stagedFile)
	if err != nil {
		t.Fatalf("error creating stagedFile file: %s", err)
	}
	gz = gzip.NewWriter(f)
	json.NewEncoder(gz).Encode(&rec)
	gz.Close()
	f.Close()

	m.Start()

	files, err := ioutil.ReadDir(stagingDir)
	if err != nil {
		t.Fatalf("ls %s: %s", stagingDir, err)
	}

	if len(files) != 3 {
		t.Errorf("got %d files in staging, want 3:", len(files))
		for _, fi := range files {
			t.Log(fi.Name())
		}
	}
	for _, fi := range files {
		recs, err := readRecordsFromGZipFile(filepath.Join(stagingDir, fi.Name()))
		if err != nil {
			t.Fatalf("error opening %s: %s", fi.Name(), err)
		}
		if len(recs) != 1 {
			t.Errorf("file %s: want 1 rec, got: %v", fi.Name(), recs)
		}
		if recs[0] != rec {
			t.Errorf("file %s: want %v, got %v", fi.Name(), rec, recs[0])
		}
	}

	fs.lock.Lock()
	fs.failUpload = 0 // allow upload now
	fs.lock.Unlock()
	time.Sleep(50 * time.Millisecond)

	m.Close()

	if f := filesIn(m.getTempDir(tenant)); len(f) != 0 {
		t.Errorf("got %d files, want %d: %#v", len(f), 0, f)
	}

	if f := filesIn(m.getStagingDir(tenant)); len(f) != 0 {
		t.Errorf("got %d files, want %d: %#v", len(f), 0, f)
	}

	if uploaded := fs.uploadedRecords(tenant); len(uploaded) != 3 {
		t.Errorf("Got %d records sent, want 3: %v", len(uploaded), uploaded)
	}
}
