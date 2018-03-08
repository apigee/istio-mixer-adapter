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
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"istio.io/istio/mixer/pkg/adapter"
)

const (
	uapPath     = "/analytics"
	httpTimeout = 60 * time.Second
	pathFmt     = "date=%s/time=%d-%d/%s_%d.%d_%s_writer_0.txt.gz"
)

type uapBackend struct {
	spool  chan *pushRequest
	stop   chan bool
	client *http.Client
	now    func() time.Time
}

type pushRequest struct {
	auth *auth.Context
	req  *Request
}

func newUAPBackend() AnalyticsProvider {
	return &uapBackend{
		spool: make(chan *pushRequest),
		stop:  make(chan bool),
		client: &http.Client{
			Timeout: httpTimeout,
		},
		now: time.Now,
	}
}

func (ub *uapBackend) Start(env adapter.Env) {
	env.ScheduleDaemon(func() {
		ub.pollingLoop()
	})
}

func (ub *uapBackend) Stop() {
	ub.stop <- true
}

func (ub *uapBackend) pollingLoop() {
	for {
		select {
		case r := <-ub.spool:
			// TODO(robbrit): do we need to batch these requests?
			if err := ub.push(r.auth, r.req); err != nil {
				r.auth.Log().Errorf("analytics not sent: %s", err)
			}
		case <-ub.stop:
			return
		}
	}
}

// push sends a request to UAP.
func (ub *uapBackend) push(ctx *auth.Context, r *Request) error {
	url, err := ub.signedURL(ctx, r)
	if err != nil {
		return fmt.Errorf("signedURL: %s", err)
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(r); err != nil {
		return fmt.Errorf("JSON encode: %s", err)
	}

	gz, err := gzip.NewReader(buf)
	if err != nil {
		return fmt.Errorf("gzip.NewReader(): %s", err)
	}

	req, err := http.NewRequest("PUT", url, gz)
	if err != nil {
		return fmt.Errorf("http.NewRequest: %s", err)
	}

	req.Header.Set("Expect", "100-continue")
	req.Header.Set("Content-Type", "application/x-gzip")
	req.Header.Set("x-amz-server-side-encryption", "AES256")
	req.ContentLength = int64(buf.Len())

	resp, err := ub.client.Do(req)
	if err != nil {
		return fmt.Errorf("client.Do(): %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("push to store returned %v", resp.Status)
	}
	return nil
}

// signedURL constructs a signed URL that can be used to upload the given request.
func (ub *uapBackend) signedURL(ctx *auth.Context, r *Request) (string, error) {
	base := ctx.ApigeeBase()
	url := path.Join(base.String(), uapPath)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	q := req.URL.Query()
	q.Add("tenant", ub.formatTenant(r))
	q.Add("relative_file_path", ub.filePath(r))
	q.Add("file_content_type", "application/x-gzip")
	q.Add("encrypt", "true")
	req.URL.RawQuery = q.Encode()

	token := "" // TODO(robbrit): How to get the token?
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := ub.client.Do(req)
	if err != nil {
		return "", nil
	}
	defer resp.Body.Close()

	var data struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("error decoding response: %s", err)
	}
	return data.URL, nil
}

func (ub *uapBackend) formatTenant(r *Request) string {
	return fmt.Sprintf("%s~%s", r.Organization, r.Environment)
}

func (ub *uapBackend) filePath(r *Request) string {
	now := ub.now()
	d := now.Format("2006-01-02")
	start := now.Unix()
	end := now.Unix()
	hex := getRandomHex()
	id := "5" // TODO(robbrit): Figure out what this is.
	return fmt.Sprintf(pathFmt, d, start, end, hex, start, end, id)
}

// SendRecords sends the records asynchronously to the UAP primary server.
func (ub *uapBackend) SendRecords(ctx *auth.Context, records []Record) error {
	for _, record := range records {
		if err := ub.validate(record); err != nil {
			return fmt.Errorf("validate(%v): %s", record, err)
		}
	}

	r, err := buildRequest(ctx, records)
	if r == nil || err != nil {
		return err
	}

	ub.spool <- &pushRequest{ctx, r}

	return nil
}

// validate confirms that a record has correct values in it.
func (ub *uapBackend) validate(record Record) error {
	return nil
}

func getRandomHex() string {
	buff := make([]byte, 2)
	rand.Read(buff)
	return fmt.Sprintf("%x", buff)
}
