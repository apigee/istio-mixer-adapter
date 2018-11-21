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
	"testing"

	adaptertest "istio.io/istio/mixer/pkg/adapter/test"
)

func TestBucketClose(t *testing.T) {

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(d)

	env := adaptertest.NewEnv(t)

	opts := Options{
		LegacyEndpoint:   true,
		BufferPath:       d,
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

	dir, err := ioutil.TempDir(d, "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}

	b := &bucket{
		manager:  m,
		log:      m.log,
		dir:      dir,
		tenant:   "tenant",
		incoming: make(chan []Record),
		closer:   make(chan closeReq, 1),
	}
	m.env.ScheduleDaemon(b.runLoop)

	records := []Record{
		{
			Organization: "test",
			Environment:  "test",
		},
	}
	b.write(records)
	m.closeWait.Add(1)
	b.stop()
	m.closeWait.Wait()

	files, err := ioutil.ReadDir(dir)
	if len(files) != 1 {
		t.Errorf("got %d files, expected %d files: %v", len(files), 1, files)
	}
}
