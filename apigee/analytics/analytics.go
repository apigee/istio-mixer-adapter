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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/google/uuid"
	multierror "github.com/hashicorp/go-multierror"
	"istio.io/istio/mixer/pkg/adapter"
)

const (
	axRecordType     = "APIAnalytics"
	uapPath          = "/analytics"
	httpTimeout      = 60 * time.Second
	defaultSpoolSize = 100
	pathFmt          = "date=%s/time=%d-%d/"
	fileFmt          = "%s_%d_%s_writer_0.json.gz"
	// collection interval is not configurable at the moment because UAP can
	// become unstable if all the Istio adapters are spamming it faster than
	// that. Hard code for now.
	defaultCollectionInterval = 1 * time.Minute
	defaultBufferPath         = "/tmp/apigee-ax/buffer/"
)

// TimeToUnix converts a time to a UNIX timestamp in milliseconds.
func TimeToUnix(t time.Time) int64 {
	return t.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

// recordData is a struct that wraps some buffered records with the inf
// needed to upload to UAP.
type recordData struct {
	Org            string
	Env            string
	Key            string
	Secret         string
	Base           string
	EncodedRecords []byte
}

// A Manager is a way for Istio to interact with Apigee's analytics platform.
type Manager struct {
	close              chan bool
	client             *http.Client
	now                func() time.Time
	log                adapter.Logger
	collectionInterval time.Duration
	bufferPath         string

	// This needs to be a unique value for this instance of mixer, otherwise
	// different mixers have a small probability of clobbering one another.
	instanceID string
}

// Options allows us to specify options for how this analytics manager will run.
type Options struct {
	// BufferPath is the directory where the adapter will buffer analytics records.
	BufferPath string
}

// NewManager constructs and starts a new Manager. Call Close when you are done.
func NewManager(env adapter.Env, opts Options) *Manager {
	m := newManager(opts)
	m.Start(env)
	return m
}

func newManager(opts Options) *Manager {
	if opts.BufferPath == "" {
		opts.BufferPath = defaultBufferPath
	}
	return &Manager{
		close: make(chan bool),
		client: &http.Client{
			Timeout: httpTimeout,
		},
		now:                time.Now,
		collectionInterval: defaultCollectionInterval,
		bufferPath:         opts.BufferPath,
		instanceID:         uuid.New().String(),
	}
}

// Start starts the manager.
func (m *Manager) Start(env adapter.Env) {
	m.log = env.Logger()
	env.ScheduleDaemon(func() {
		m.uploadLoop()
	})
}

// Close shuts down the manager.
func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.close <- true
	if err := m.upload(); err != nil {
		m.log.Errorf("Error pushing analytics: %s", err)
	}
}

// uploadLoop runs a timer that periodically pushes everything in the buffer
// directory to the server.
func (m *Manager) uploadLoop() {
	t := time.NewTimer(m.collectionInterval)
	for {
		select {
		case <-t.C:
			if err := m.upload(); err != nil {
				m.log.Errorf("Error pushing analytics: %s", err)
			}
		case <-m.close:
			t.Stop()
			return
		}
	}
}

// upload sends any buffered data to UAP.
func (m *Manager) upload() error {
	files, err := ioutil.ReadDir(m.bufferPath)
	if err != nil {
		return fmt.Errorf("ReadDir(%s): %s", m.bufferPath, err)
	}

	var errOut error
	for _, fi := range files {
		fullName := path.Join(m.bufferPath, fi.Name())
		f, err := os.Open(fullName)
		if err != nil {
			errOut = multierror.Append(errOut, err)
			continue
		}

		var rd recordData
		if err := json.NewDecoder(f).Decode(&rd); err != nil {
			errOut = multierror.Append(errOut, fmt.Errorf("json.Decode(): %s", err))
			continue
		}

		if err := m.push(&rd, fi.Name()); err != nil {
			errOut = multierror.Append(errOut, err)
		} else if err := os.Remove(fullName); err != nil {
			errOut = multierror.Append(errOut, fmt.Errorf("rm %s: %s", fullName, err))
		}
	}
	return errOut
}

// push sends records to UAP.
func (m *Manager) push(rd *recordData, filename string) error {
	url, err := m.signedURL(rd, filename)
	if err != nil {
		return fmt.Errorf("signedURL: %s", err)
	}

	buf := bytes.NewBuffer(rd.EncodedRecords)
	req, err := http.NewRequest("PUT", url, buf)
	if err != nil {
		return fmt.Errorf("http.NewRequest: %s", err)
	}

	req.Header.Set("Expect", "100-continue")
	req.Header.Set("Content-Type", "application/x-gzip")
	req.Header.Set("x-amz-server-side-encryption", "AES256")
	req.ContentLength = int64(buf.Len())

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("client.Do(): %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("push to store returned %v", resp.Status)
	}
	return nil
}

// signedURL constructs a signed URL that can be used to upload records.
func (m *Manager) signedURL(rd *recordData, filename string) (string, error) {
	url := rd.Base + uapPath
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	q := req.URL.Query()
	q.Add("tenant", fmt.Sprintf("%s~%s", rd.Org, rd.Env))
	q.Add("relative_file_path", path.Join(m.uploadDir(), filename))
	q.Add("file_content_type", "application/x-gzip")
	q.Add("encrypt", "true")
	req.URL.RawQuery = q.Encode()

	req.SetBasicAuth(rd.Key, rd.Secret)

	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("non-200 status returned from %s: %s", url, resp.Status)
	}

	var data struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("error decoding response: %s", err)
	}
	return data.URL, nil
}

// uploadDir gets a directory for where we should upload the file.
func (m *Manager) uploadDir() string {
	now := m.now()
	d := now.Format("2006-01-02")
	start := now.Unix()
	end := now.Add(m.collectionInterval).Unix()
	return fmt.Sprintf(pathFmt, d, start, end)
}

// SendRecords sends the records asynchronously to the UAP primary server.
func (m *Manager) SendRecords(ctx *auth.Context, records []Record) error {
	for _, record := range records {
		if err := m.validate(record); err != nil {
			return fmt.Errorf("validate(%v): %s", record, err)
		}
	}

	r, err := buildRequest(ctx, records)
	if r == nil || err != nil {
		return err
	}

	// Encode records into gzipped JSON
	buf := new(bytes.Buffer)
	gz := gzip.NewWriter(buf)
	if err := json.NewEncoder(gz).Encode(records); err != nil {
		return fmt.Errorf("JSON encode: %s", err)
	}
	if err := gz.Flush(); err != nil {
		return fmt.Errorf("gzip upload: %s", err)
	}

	base := ctx.ApigeeBase()
	rd := &recordData{
		ctx.Organization(),
		ctx.Environment(),
		ctx.Key(),
		ctx.Secret(),
		base.String(),
		buf.Bytes(),
	}

	u, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("uuid.Random(): %s", err)
	}

	fn := path.Join(m.bufferPath, u.String()+".json.gz")

	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(f).Encode(rd); err != nil {
		return fmt.Errorf("json.Encode(): %s", err)
	}

	return nil
}

// validate confirms that a record has correct values in it.
func (m *Manager) validate(record Record) error {
	// TODO(robbrit): What validation do we need?
	return nil
}

func buildRequest(auth *auth.Context, records []Record) (*request, error) {
	if auth == nil || len(records) == 0 {
		return nil, nil
	}
	if auth.Organization() == "" || auth.Environment() == "" {
		return nil, fmt.Errorf("organization and environment are required in auth: %v", auth)
	}

	for i := range records {
		records[i].RecordType = axRecordType

		// populate from auth context
		records[i].DeveloperEmail = auth.DeveloperEmail
		records[i].DeveloperApp = auth.Application
		records[i].AccessToken = auth.AccessToken
		records[i].ClientID = auth.ClientID

		// todo: select best APIProduct based on path, otherwise arbitrary
		if len(auth.APIProducts) > 0 {
			records[i].APIProduct = auth.APIProducts[0]
		}
	}

	return &request{
		Organization: auth.Organization(),
		Environment:  auth.Environment(),
		Records:      records,
	}, nil
}
