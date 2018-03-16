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

// This file defines an implementation of the analytics.Manager interface that
// sends analytics records to UAP.

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
	uapPath          = "/analytics"
	httpTimeout      = 60 * time.Second
	defaultSpoolSize = 100
	pathFmt          = "date=%s/time=%d-%d/%s_%d.%d_%s_writer_0.txt.gz"
	// collection interval is not configurable at the moment because UAP can
	// become unstable if all the Istio adapters are spamming it faster than
	// that. Hard code for now.
	defaultCollectionInterval = 1 * time.Minute
)

// pushKey is the key that we use to group different records together for
// uploading to UAP.
type pushKey struct {
	org    string
	env    string
	key    string
	secret string
	base   string
}

// A uapManager implements analytics.Manager and sends its analytics records
// directly to UAP.
type uapManager struct {
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

func newUAPManager() Manager {
	return &uapManager{
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
func (ub *uapManager) Start(env adapter.Env) {
	ub.log = env.Logger()
	env.ScheduleDaemon(func() {
		ub.flushLoop()
	})
}

// Close shuts down the manager.
func (ub *uapManager) Close() {
	ub.close <- true
	if err := ub.flush(); err != nil {
		ub.log.Errorf("Error pushing analytics: %s", err)
	}
}

// flushLoop runs a timer that periodically pushes everything in the buffer to
// the server.
func (ub *uapManager) flushLoop() {
	t := time.NewTimer(ub.collectionInterval)
	for {
		select {
		case <-t.C:
			if err := ub.flush(); err != nil {
				ub.log.Errorf("Error pushing analytics: %s", err)
			}
		case <-ub.close:
			t.Stop()
			return
		}
	}
}

// flush sends any buffered data off to the server.
func (ub *uapManager) flush() error {
	// Swap out the buffer with a new one so that the other goroutine can still
	// log records while we're uploading the previous ones to the server.
	ub.bufferLock.Lock()
	buff := ub.buffer
	ub.buffer = map[pushKey][]Record{}
	ub.bufferLock.Unlock()

	var errOut error
	for pk, rs := range buff {
		if err := ub.push(pk, rs); err != nil {
			errOut = multierror.Append(errOut, err)
		}
	}
	return errOut
}

// push sends records to UAP.
func (ub *uapManager) push(pk pushKey, records []Record) error {
	url, err := ub.signedURL(pk)
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

// signedURL constructs a signed URL that can be used to upload records.
func (ub *uapManager) signedURL(pk pushKey) (string, error) {
	url := pk.base + uapPath
	ub.log.Infof("Fetching from %s: %#v", url, pk)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	q := req.URL.Query()
	q.Add("tenant", fmt.Sprintf("%s~%s", pk.org, pk.env))
	q.Add("relative_file_path", ub.filePath())
	q.Add("file_content_type", "application/x-gzip")
	q.Add("encrypt", "true")
	req.URL.RawQuery = q.Encode()

	req.SetBasicAuth(pk.key, pk.secret)

	resp, err := ub.client.Do(req)
	if err != nil {
		return "", err
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

// filePath constructs a file path for an analytics record.
func (ub *uapManager) filePath() string {
	now := ub.now()
	d := now.Format("2006-01-02")
	start := now.Unix()
	end := now.Add(ub.collectionInterval).Unix()
	hex := getRandomHex()
	id := ub.instanceID
	return fmt.Sprintf(pathFmt, d, start, end, hex, start, end, id)
}

// SendRecords sends the records asynchronously to the UAP primary server.
func (ub *uapManager) SendRecords(ctx *auth.Context, records []Record) error {
	for _, record := range records {
		if err := ub.validate(record); err != nil {
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

	ub.bufferLock.Lock()
	// TODO(robbrit): Write these records to a persistent store so that if the
	// server dies here, we don't lose the records. If we do that, move it into a
	// different goroutine so that writing the files doesn't slow other things.
	ub.buffer[pk] = append(ub.buffer[pk], records...)
	ub.bufferLock.Unlock()

	return nil
}

// validate confirms that a record has correct values in it.
func (ub *uapManager) validate(record Record) error {
	// TODO(robbrit): What validation do we need?
	return nil
}

func getRandomHex() string {
	buff := make([]byte, 2)
	rand.Read(buff)
	return fmt.Sprintf("%x", buff)
}
