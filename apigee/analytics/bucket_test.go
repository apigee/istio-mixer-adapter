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
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBucket(t *testing.T) {

	testDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(testDir)

	now := time.Now

	uploader := &saasUploader{
		client:  http.DefaultClient,
		baseURL: &url.URL{},
		key:     "key",
		secret:  "secret",
		now:     now,
	}

	opts := Options{
		LegacyEndpoint:     true,
		BufferPath:         testDir,
		StagingFileLimit:   10,
		now:                now,
		CollectionInterval: time.Minute,
	}

	m, err := newManager(uploader, opts)
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}

	tenant := getTenantName("test", "test")
	err = m.prepTenant(tenant)
	if err != nil {
		t.Fatalf("prepTenant: %v", err)
	}
	tempDir := m.getTempDir(tenant)
	stageDir := m.getStagingDir(tenant)

	m.Start()
	defer m.Close()

	b, err := newBucket(m, uploader, tenant, tempDir)
	if err != nil {
		t.Fatalf("newBucket: %v", err)
	}

	records := []Record{
		{
			Organization: "test",
			Environment:  "test",
		},
	}
	b.write(records)

	wait := &sync.WaitGroup{}
	wait.Add(1)
	b.close(wait)
	wait.Wait()

	files, err := ioutil.ReadDir(tempDir)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if len(files) != 0 {
		t.Errorf("got %d files, expected %d files: %v", len(files), 0, files)
	}

	files, err = ioutil.ReadDir(stageDir)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, expected %d files: %v", len(files), 1, files)
	}

	if !strings.HasSuffix(files[0].Name(), ".gz") {
		t.Errorf("file %s should have .gz suffix", files[0])
	}

	stagedFile := filepath.Join(stageDir, files[0].Name())

	recs, err := readRecordsFromGZipFile(stagedFile)
	if err != nil {
		t.Fatalf("readRecordsFromGZipFile: %v", err)
	}

	if !reflect.DeepEqual(records, recs) {
		t.Errorf("got: %v, want: %v", recs, records)
	}
}
