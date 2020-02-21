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
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/authtest"
)

func TestStagingSizeCap(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(t)
	fs.failUpload = http.StatusInternalServerError
	defer fs.close()

	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.
	now := func() time.Time { return time.Unix(ts, 0) }

	workDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(): %s", err)
	}
	defer os.RemoveAll(workDir)

	baseURL, _ := url.Parse(fs.URL())

	uploader := &saasUploader{
		client:  http.DefaultClient,
		baseURL: baseURL,
		key:     "key",
		secret:  "secret",
		now:     now,
	}

	m, err := newManager(uploader, Options{
		BufferPath:         workDir,
		StagingFileLimit:   3,
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

	m.Start()

	t1 := "hi~test"
	tc := authtest.NewContext(fs.URL())
	tc.SetOrganization("hi")
	tc.SetEnvironment("test")
	ctx := &auth.Context{Context: tc}

	for i := 1; i < m.stagingFileLimit+3; i++ {
		if err := m.SendRecords(ctx, records); err != nil {
			t.Errorf("Error on SendRecords(): %s", err)
		}
		m.stageAllBucketsWait()
	}
	time.Sleep(50 * time.Millisecond)

	if f := filesIn(m.getTempDir(t1)); len(f) != 0 {
		t.Errorf("got %d files, want %d: %v", len(f), 0, f)
	}

	if f := filesIn(m.getStagingDir(t1)); len(f) != m.stagingFileLimit {
		t.Errorf("got %d files, want %d: %v", len(f), m.stagingFileLimit, f)
	}

	m.Close()
}

func filesIn(path string) []string {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil
	}
	var result []string
	for _, f := range files {
		result = append(result, filepath.Join(path, f.Name()))
	}
	return result
}
