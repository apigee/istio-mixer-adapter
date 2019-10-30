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
	"sync"
	"testing"

	adaptertest "istio.io/istio/mixer/pkg/adapter/test"
)

func TestBucketClose(t *testing.T) {

	testDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(testDir)

	env := adaptertest.NewEnv(t)

	opts := Options{
		LegacyEndpoint:   true,
		BufferPath:       testDir,
		StagingFileLimit: 10,
		BaseURL:          url.URL{},
		Key:              "key",
		Secret:           "secret",
		Client:           http.DefaultClient,
	}

	m, err := newManager(opts)
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}
	m.env = env
	m.log = env

	tempDir, err := ioutil.TempDir(testDir, "temp")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	b := newBucket(m, tempDir)

	records := []Record{
		{
			Organization: "test",
			Environment:  "test",
		},
	}
	b.write(records)

	stageDir, err := ioutil.TempDir(testDir, "stage")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}

	wait := &sync.WaitGroup{}
	wait.Add(1)
	b.close(stageDir, wait)
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
		t.Errorf("got %d files, expected %d files: %v", len(files), 1, files)
	}

	stagedFile := filepath.Join(stageDir, files[0].Name())
	err = b.manager.validateGZip(stagedFile)
	if err != nil {
		t.Errorf("error validating gzip: %v", err)
	}
}
