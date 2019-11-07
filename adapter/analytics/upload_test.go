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
	"strings"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"github.com/apigee/istio-mixer-adapter/adapter/authtest"
	adaptertest "istio.io/istio/mixer/pkg/adapter/test"
)

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
	m, err := newManager(Options{
		BufferPath:       d,
		StagingFileLimit: 10,
		BaseURL:          *baseURL,
		Key:              "key",
		Secret:           "secret",
		Client:           http.DefaultClient,
		SendChannelSize:  0,
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
	authCtx := &auth.Context{Context: tc}

	// since we're using a custom errorHandler we can't call m.Start() and need to do this setup
	var uploadError error
	errH := func(err error) error {
		env.Logger().Errorf("errH: %v", err)
		uploadError = err
		return err
	}
	m.startUploader(env, errH)
	m.startStagingSweeper(env)

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
