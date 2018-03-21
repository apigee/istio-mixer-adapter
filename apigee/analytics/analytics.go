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
	"sync"
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
	pathFmt          = "date=%s/time=%d-%d/%s_%d.%d_%s_writer_0.txt.gz"
	// collection interval is not configurable at the moment because UAP can
	// become unstable if all the Istio adapters are spamming it faster than
	// that. Hard code for now.
	defaultCollectionInterval = 1 * time.Minute
)

// TimeToUnix converts a time to a UNIX timestamp in milliseconds.
func TimeToUnix(t time.Time) int64 {
	return t.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

// pushKey is the key that we use to group different records together for
// uploading to UAP.
type pushKey struct {
	org    string
	env    string
	key    string
	secret string
	base   string
}

// A Manager is a way for Istio to interact with Apigee's analytics platform.
type Manager struct {
	close              chan bool
	buffer             map[pushKey][]Record
	bufferLock         sync.Mutex
	client             *http.Client
	now                func() time.Time
	log                adapter.Logger
	collectionInterval time.Duration

	// This needs to be a unique value for this instance of mixer, otherwise
	// different mixers have a small probability of clobbering one another.
	instanceID string
}

// NewManager constructs and starts a new Manager. Call Close when you are done.
func NewManager(env adapter.Env) *Manager {
	m := newManager()
	m.Start(env)
	return m
}

func newManager() *Manager {
	return &Manager{
		buffer: map[pushKey][]Record{},
		close:  make(chan bool),
		client: &http.Client{
			Timeout: httpTimeout,
		},
		now:                time.Now,
		collectionInterval: defaultCollectionInterval,
		instanceID:         uuid.New().String(),
	}
}

// Start starts the manager.
func (m *Manager) Start(env adapter.Env) {
	m.log = env.Logger()
	env.ScheduleDaemon(func() {
		m.flushLoop()
	})
}

// Close shuts down the manager.
func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.close <- true
	if err := m.flush(); err != nil {
		m.log.Errorf("Error pushing analytics: %s", err)
	}
}

// flushLoop runs a timer that periodically pushes everything in the buffer to
// the server.
func (m *Manager) flushLoop() {
	t := time.NewTimer(m.collectionInterval)
	for {
		select {
		case <-t.C:
			if err := m.flush(); err != nil {
				m.log.Errorf("Error pushing analytics: %s", err)
			}
		case <-m.close:
			t.Stop()
			return
		}
	}
}

// flush sends any buffered data off to the server.
func (m *Manager) flush() error {
	// Swap out the buffer with a new one so that the other goroutine can still
	// log records while we're uploading the previous ones to the server.
	m.bufferLock.Lock()
	buff := m.buffer
	m.buffer = map[pushKey][]Record{}
	m.bufferLock.Unlock()

	var errOut error
	for pk, rs := range buff {
		if err := m.push(pk, rs); err != nil {
			// On a failure, push records back into the buffer so that we attempt to
			// push them again later.
			// TODO(robbrit): Do we always want to reload records? What if the records
			// are corrupt?
			m.loadRecords(pk, rs)
			errOut = multierror.Append(errOut, err)
		}
	}
	return errOut
}

// push sends records to UAP.
func (m *Manager) push(pk pushKey, records []Record) error {
	url, err := m.signedURL(pk)
	if err != nil {
		return fmt.Errorf("signedURL: %s", err)
	}

	buf := new(bytes.Buffer)

	// First, write gzipped JSON to the buffer.
	gz := gzip.NewWriter(buf)
	if err := json.NewEncoder(gz).Encode(records); err != nil {
		return fmt.Errorf("JSON encode: %s", err)
	}
	if err := gz.Flush(); err != nil {
		return fmt.Errorf("gzip flush: %s", err)
	}

	// Now send the buffer to the server.
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
func (m *Manager) signedURL(pk pushKey) (string, error) {
	url := pk.base + uapPath
	m.log.Infof("Fetching from %s: %#v", url, pk)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	q := req.URL.Query()
	q.Add("tenant", fmt.Sprintf("%s~%s", pk.org, pk.env))
	q.Add("relative_file_path", m.filePath())
	q.Add("file_content_type", "application/x-gzip")
	q.Add("encrypt", "true")
	req.URL.RawQuery = q.Encode()

	req.SetBasicAuth(pk.key, pk.secret)

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

// filePath constructs a file path for an analytics record.
func (m *Manager) filePath() string {
	now := m.now()
	d := now.Format("2006-01-02")
	start := now.Unix()
	end := now.Add(m.collectionInterval).Unix()
	hex := randomHex()
	id := m.instanceID
	return fmt.Sprintf(pathFmt, d, start, end, hex, start, end, id)
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

	base := ctx.ApigeeBase()
	pk := pushKey{
		ctx.Organization(),
		ctx.Environment(),
		ctx.Key(),
		ctx.Secret(),
		base.String(),
	}

	m.loadRecords(pk, records)

	return nil
}

func (m *Manager) loadRecords(pk pushKey, records []Record) {
	m.bufferLock.Lock()
	// TODO(robbrit): Write these records to a persistent store so that if the
	// server dies here, we don't lose the records. If we do that, move it into a
	// different goroutine so that writing the files doesn't slow other things.
	m.buffer[pk] = append(m.buffer[pk], records...)
	m.bufferLock.Unlock()
}

// validate confirms that a record has correct values in it.
func (m *Manager) validate(record Record) error {
	// TODO(robbrit): What validation do we need?
	return nil
}

func randomHex() string {
	buff := make([]byte, 2)
	rand.Read(buff)
	return fmt.Sprintf("%x", buff)
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
