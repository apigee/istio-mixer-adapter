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
	"encoding/base64"
	"encoding/json"
	"errors"
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
	axRecordType = "APIAnalytics"
	httpTimeout  = 60 * time.Second
	pathFmt      = "date=%s/time=%d-%d/"
	// collection interval is not configurable at the moment because UAP can
	// become unstable if all the Istio adapters are spamming it faster than
	// that. Hard code for now.
	defaultCollectionInterval = 1 * time.Minute
)

// recordData is a struct that wraps some buffered records with the inf
// needed to upload to UAP.
type recordData struct {
	Org            string
	Env            string
	Key            string
	Secret         string
	EncodedRecords string
}

// A Manager is a way for Istio to interact with Apigee's analytics platform.
type Manager struct {
	close              chan bool
	client             *http.Client
	now                func() time.Time
	log                adapter.Logger
	collectionInterval time.Duration
	bufferPath         string
	analyticsURL       string
}

// Options allows us to specify options for how this analytics manager will run.
type Options struct {
	// BufferPath is the directory where the adapter will buffer analytics records.
	BufferPath string
	// AnalyticsURL is where analytics get uploaded to.
	AnalyticsURL string
}

// NewManager constructs and starts a new Manager. Call Close when you are done.
func NewManager(env adapter.Env, opts Options) (*Manager, error) {
	m, err := newManager(opts)
	if err != nil {
		return nil, err
	}
	m.Start(env)
	return m, nil
}

func newManager(opts Options) (*Manager, error) {
	// Ensure that the buffer path exists and we can access it.
	if _, err := os.Stat(opts.BufferPath); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(opts.BufferPath, os.FileMode(0700)); err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("stat buffer path: %s", err)
		}
	}

	return &Manager{
		close: make(chan bool),
		client: &http.Client{
			Timeout: httpTimeout,
		},
		now:                time.Now,
		collectionInterval: defaultCollectionInterval,
		bufferPath:         opts.BufferPath,
		analyticsURL:       opts.AnalyticsURL,
	}, nil
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
	var success int
	// TODO(someone): If this is slow, use a pool of goroutines to upload.
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
		} else {
			success++
		}
	}
	m.log.Infof("Uploaded %d analytics records.", success)
	return errOut
}

// push sends records to UAP.
func (m *Manager) push(rd *recordData, filename string) error {
	url, err := m.signedURL(rd, filename)
	if err != nil {
		return fmt.Errorf("signedURL: %s", err)
	}

	b, err := base64.StdEncoding.DecodeString(rd.EncodedRecords)
	if err != nil {
		return fmt.Errorf("base64 decode: %s", err)
	}

	buf := bytes.NewBuffer(b)
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
	req, err := http.NewRequest("GET", m.analyticsURL, nil)
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
		return "", fmt.Errorf("non-200 status returned from %s: %s", m.analyticsURL, resp.Status)
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
	// First ensure that all the records have some additional fields in them.
	for i := range records {
		records[i].RecordType = axRecordType

		// populate from auth context
		records[i].DeveloperEmail = ctx.DeveloperEmail
		records[i].DeveloperApp = ctx.Application
		records[i].AccessToken = ctx.AccessToken
		records[i].ClientID = ctx.ClientID
		records[i].Organization = ctx.Organization()
		records[i].Environment = ctx.Environment()

		// todo: select best APIProduct based on path, otherwise arbitrary
		if len(ctx.APIProducts) > 0 {
			records[i].APIProduct = ctx.APIProducts[0]
		}
	}

	// Validate the records.
	goodRecords := []Record{}
	for _, record := range records {
		if err := m.validate(record); err != nil {
			m.log.Errorf("invalid record %v: %s", record, err)
			continue
		}
		goodRecords = append(goodRecords, record)
	}

	// Records are stored as JSON on disk, with one field containing all the
	// info that will be uploaded to the server.
	buf := new(bytes.Buffer)
	b64 := base64.NewEncoder(base64.StdEncoding, buf)
	gz := gzip.NewWriter(b64)
	if err := json.NewEncoder(gz).Encode(goodRecords); err != nil {
		return fmt.Errorf("JSON encode: %s", err)
	}
	if err := gz.Flush(); err != nil {
		return fmt.Errorf("gzip flush: %s", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("gzip close: %s", err)
	}
	if err := b64.Close(); err != nil {
		return fmt.Errorf("b64 close: %s", err)
	}

	rd := &recordData{
		ctx.Organization(),
		ctx.Environment(),
		ctx.Key(),
		ctx.Secret(),
		buf.String(),
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
func (m *Manager) validate(r Record) error {
	var err error

	// Validate that certain fields are set.
	if r.Organization == "" {
		err = multierror.Append(err, errors.New("missing Organization"))
	}
	if r.Environment == "" {
		err = multierror.Append(err, errors.New("missing Environment"))
	}
	if r.ClientReceivedStartTimestamp == 0 {
		err = multierror.Append(err, errors.New("missing ClientReceivedStartTimestamp"))
	}
	if r.ClientReceivedEndTimestamp == 0 {
		err = multierror.Append(err, errors.New("missing ClientReceivedEndTimestamp"))
	}
	if r.ClientReceivedStartTimestamp > r.ClientReceivedEndTimestamp {
		err = multierror.Append(err, errors.New("ClientReceivedStartTimestamp > ClientReceivedEndTimestamp"))
	}

	// Validate that timestamps make sense.
	ts := time.Unix(r.ClientReceivedStartTimestamp/1000, 0)
	if ts.After(m.now()) {
		err = multierror.Append(err, errors.New("ClientReceivedStartTimestamp cannot be in the future"))
	}
	if ts.Before(m.now().Add(-90 * 24 * time.Hour)) {
		err = multierror.Append(err, errors.New("ClientReceivedStartTimestamp cannot be more than 90 days old"))
	}
	return err
}
