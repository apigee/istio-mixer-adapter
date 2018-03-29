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
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/google/uuid"
	multierror "github.com/hashicorp/go-multierror"
	"istio.io/istio/mixer/pkg/adapter"
)

const (
	analyticsPath = "/analytics/organization/%s/environment/%s"
	axRecordType  = "APIAnalytics"
	httpTimeout   = 60 * time.Second
	pathFmt       = "date=%s/time=%d-%d/"
	bufferMode    = os.FileMode(0700)
	tempDir       = "temp"
	stagingDir    = "staging"

	// collection interval is not configurable at the moment because UAP can
	// become unstable if all the Istio adapters are spamming it faster than
	// that. Hard code for now.
	defaultCollectionInterval = 1 * time.Minute
)

// A bucket keeps track of all the things we need to read/write the analytics
// files.
type bucket struct {
	filename string
	gz       *gzip.Writer
	f        *os.File
}

// A cred keeps track of the key/secret for a given bucket.
type cred struct {
	key    string
	secret string
	base   string
}

// A Manager is a way for Istio to interact with Apigee's analytics platform.
type Manager struct {
	close              chan bool
	client             *http.Client
	now                func() time.Time
	log                adapter.Logger
	collectionInterval time.Duration
	tempDir            string
	stagingDir         string
	analyticsURL       string
	bucketsLock        sync.RWMutex
	buckets            map[string]bucket // Map from dirname -> bucket.
	creds              sync.Map          // Map from dirname -> cred.
}

// Options allows us to specify options for how this analytics manager will run.
type Options struct {
	// BufferPath is the directory where the adapter will buffer analytics records.
	BufferPath string
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
	td := path.Join(opts.BufferPath, tempDir)
	sd := path.Join(opts.BufferPath, stagingDir)
	if err := os.MkdirAll(td, bufferMode); err != nil {
		return nil, fmt.Errorf("mkdir %s: %s", td, err)
	}
	if err := os.MkdirAll(sd, bufferMode); err != nil {
		return nil, fmt.Errorf("mkdir %s: %s", sd, err)
	}

	return &Manager{
		close: make(chan bool),
		client: &http.Client{
			Timeout: httpTimeout,
		},
		now:                time.Now,
		collectionInterval: defaultCollectionInterval,
		tempDir:            td,
		stagingDir:         sd,
		buckets:            map[string]bucket{},
		creds:              sync.Map{},
	}, nil
}

// crashRecovery cleans up the temp and staging dirs post-crash. This function
// assumes that both the temp and staging dirs exist and are accessible.
func (m *Manager) crashRecovery() error {
	dirs, err := ioutil.ReadDir(m.tempDir)
	if err != nil {
		return err
	}
	var errs error
	for _, d := range dirs {
		bucket := d.Name()
		files, err := ioutil.ReadDir(path.Join(m.tempDir, bucket))
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}

		// Ensure the staging directory exists.
		p := path.Join(m.stagingDir, bucket)
		if err := os.MkdirAll(p, bufferMode); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("mkdir %s: %s", p, err))
			continue
		}

		for _, fi := range files {
			old := path.Join(m.tempDir, bucket, fi.Name())
			new := path.Join(p, fi.Name())
			// Check if it is a valid gzip file.
			f, err := os.Open(old)
			if err != nil {
				errs = multierror.Append(errs, err)
				continue
			}
			gz, err := gzip.NewReader(f)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("gzip.NewReader(%s): %s", fi.Name(), err))
				continue
			}
			if _, err := ioutil.ReadAll(gz); err != nil {
				// File couldn't be read, attempt recovery.
				if err.Error() != "unexpected EOF" {
					errs = multierror.Append(errs, fmt.Errorf("readall(%s): %s", fi.Name(), err))
				} else if err := m.recoverFile(old, new); err != nil {
					errs = multierror.Append(errs, err)
				}
				continue
			}
			if err := os.Rename(old, new); err != nil {
				errs = multierror.Append(errs, err)
				continue
			}
		}
	}
	return errs
}

// recoverFile recovers gzipped data in a file and puts it into a new file.
func (m *Manager) recoverFile(old, new string) error {
	in, err := os.Open(old)
	if err != nil {
		return fmt.Errorf("open %s: %s", old, err)
	}
	br := bufio.NewReader(in)
	gzr, err := gzip.NewReader(br)
	if err != nil {
		return fmt.Errorf("gzip.NewReader(%s): %s", old, err)
	}
	defer gzr.Close()

	out, err := os.Create(new)
	if err != nil {
		return fmt.Errorf("create %s: %s", new, err)
	}
	defer out.Close()
	gzw := gzip.NewWriter(out)
	defer gzw.Close()

	// The size of this buffer is arbitrary and doesn't really matter.
	b := make([]byte, 1000)
	for {
		if _, err := gzr.Read(b); err != nil {
			if err.Error() != "unexpected EOF" && err.Error() != "EOF" {
				return fmt.Errorf("scan %s: %s", old, err)
			}
			break
		}
		gzw.Write(b)
	}
	return nil
}

// Start starts the manager.
func (m *Manager) Start(env adapter.Env) {
	m.log = env.Logger()

	if err := m.crashRecovery(); err != nil {
		m.log.Errorf("Error(s) recovering crashed data: %s", err)
	}

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
	if err := m.uploadAll(); err != nil {
		m.log.Errorf("Error pushing analytics: %s", err)
	}
}

// uploadLoop runs a timer that periodically pushes everything in the buffer
// directory to the server.
func (m *Manager) uploadLoop() {
	t := time.NewTicker(m.collectionInterval)
	for {
		select {
		case <-t.C:
			if err := m.uploadAll(); err != nil {
				m.log.Errorf("Error pushing analytics: %s", err)
			}
		case <-m.close:
			m.log.Infof("analytics close signal received, shutting down")
			t.Stop()
			return
		}
	}
}

// commitStaging moves anything in the temp dir to the staging dir.
func (m *Manager) commitStaging() error {
	subdirs, err := ioutil.ReadDir(m.tempDir)
	if err != nil {
		return fmt.Errorf("ReadDir(%s): %s", m.tempDir, err)
	}

	var errs error
	successes := 0
	for _, subdir := range subdirs {
		sn := subdir.Name()

		m.bucketsLock.Lock()
		if b, ok := m.buckets[sn]; ok {
			// Remove the bucket, this shouldn't be an issue since the file will still
			// be there and we can pick it up the next iteration.
			delete(m.buckets, sn)
			if err := b.gz.Close(); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("gzip.Close %s: %s", sn, err))
			} else if err := b.f.Close(); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("file.Close %s: %s", sn, err))
			}
		}
		m.bucketsLock.Unlock()

		// Copy over all the files in the temp dir to the staging dir.
		old := path.Join(m.tempDir, sn)
		new := path.Join(m.stagingDir, sn)
		files, err := ioutil.ReadDir(old)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("ioutil.ReadDir(%s): %s", old, err))
			continue
		}
		if err := os.MkdirAll(new, bufferMode); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("mkdir %s: %s", new, err))
			continue
		}
		for _, f := range files {
			oldf := path.Join(old, f.Name())
			newf := path.Join(new, f.Name())
			if err := os.Rename(oldf, newf); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("mv %s: %s", oldf, err))
				continue
			}
			successes++
		}
	}
	m.log.Infof("committed %d analytics packages to staging to be uploaded", successes)
	return errs
}

// uploadAll commits everything from staging and then uploads it.
func (m *Manager) uploadAll() error {
	if err := m.commitStaging(); err != nil {
		m.log.Errorf("Error moving analytics into staging dir: %s", err)
		// Don't return here, we may have committed some dirs and will want to
		// upload them.
	}

	subdirs, err := ioutil.ReadDir(m.stagingDir)
	if err != nil {
		return fmt.Errorf("ReadDir(%s): %s", m.stagingDir, err)
	}

	var errOut error
	// TODO(someone): If this is slow, use a pool of goroutines to upload.
	for _, subdir := range subdirs {
		if err := m.upload(subdir.Name()); err != nil {
			errOut = multierror.Append(errOut, err)
		}
	}
	m.log.Infof("completed analytics upload, going back to sleep")
	return errOut
}

// upload sends all the files in a given staging subdir to UAP.
func (m *Manager) upload(subdir string) error {
	p := path.Join(m.stagingDir, subdir)
	files, err := ioutil.ReadDir(p)
	if err != nil {
		return fmt.Errorf("ls %s: %s", p, err)
	}
	var errs error
	successes := 0
	for _, fi := range files {
		url, err := m.signedURL(subdir, fi.Name())
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("signedURL: %s", err))
			continue
		}
		fn := path.Join(p, fi.Name())
		f, err := os.Open(fn)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}

		req, err := http.NewRequest("PUT", url, f)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("http.NewRequest: %s", err))
			continue
		}

		req.Header.Set("Expect", "100-continue")
		req.Header.Set("Content-Type", "application/x-gzip")
		req.Header.Set("x-amz-server-side-encryption", "AES256")
		req.ContentLength = fi.Size()

		resp, err := m.client.Do(req)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("client.Do(): %s", err))
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			errs = multierror.Append(errs, fmt.Errorf("push %s/%s to store returned %v", p, fi.Name(), resp.Status))
			continue
		}

		if err := os.Remove(fn); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("rm %s: %s", fn, err))
			continue
		}
		successes++
	}
	m.log.Infof("uploaded %d analytics packages.", successes)
	return errs
}

func (m *Manager) orgEnvFromSubdir(subdir string) (string, string) {
	s := strings.Split(subdir, "~")
	if len(s) == 2 {
		return s[0], s[1]
	}
	return "", ""
}

// signedURL constructs a signed URL that can be used to upload records.
func (m *Manager) signedURL(subdir, filename string) (string, error) {
	credsI, ok := m.creds.Load(subdir)
	if !ok {
		return "", fmt.Errorf("no auth creds for %s", subdir)
	}
	creds := credsI.(cred)

	org, env := m.orgEnvFromSubdir(subdir)
	if org == "" || env == "" {
		return "", fmt.Errorf("invalid subdir %s", subdir)
	}

	p := creds.base + fmt.Sprintf(analyticsPath, org, env)

	req, err := http.NewRequest("GET", p, nil)
	if err != nil {
		return "", err
	}

	q := req.URL.Query()
	q.Add("tenant", subdir)
	q.Add("relative_file_path", path.Join(m.uploadDir(), filename))
	q.Add("file_content_type", "application/x-gzip")
	q.Add("encrypt", "true")
	req.URL.RawQuery = q.Encode()

	req.SetBasicAuth(creds.key, creds.secret)

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

// ensureFields makes sure all the records in a list have the fields they need.
func (m *Manager) ensureFields(ctx *auth.Context, records []Record) {
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
}

// SendRecords sends the records asynchronously to the UAP primary server.
func (m *Manager) SendRecords(ctx *auth.Context, records []Record) error {
	m.ensureFields(ctx, records)

	// Validate the records.
	goodRecords := []Record{}
	for _, record := range records {
		if err := m.validate(record); err != nil {
			m.log.Errorf("invalid record %v: %s", record, err)
			continue
		}
		goodRecords = append(goodRecords, record)
	}

	// Write records to the file on disk.
	d := m.bucketDir(ctx)
	m.bucketsLock.RLock()
	if _, ok := m.buckets[d]; !ok {
		m.bucketsLock.RUnlock()
		if err := m.createBucket(ctx, d); err != nil {
			return err
		}
		m.bucketsLock.RLock()
	}
	defer m.bucketsLock.RUnlock()

	gz := m.buckets[d].gz
	if err := json.NewEncoder(gz).Encode(goodRecords); err != nil {
		return fmt.Errorf("JSON encode: %s", err)
	}
	if err := gz.Flush(); err != nil {
		return fmt.Errorf("gzip.Flush(): %s", err)
	}
	return nil
}

func (m *Manager) bucketDir(ctx *auth.Context) string {
	return fmt.Sprintf("%s~%s", ctx.Organization(), ctx.Environment())
}

func (m *Manager) createBucket(ctx *auth.Context, d string) error {
	m.bucketsLock.Lock()
	defer m.bucketsLock.Unlock()

	u, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("uuid.Random(): %s", err)
	}

	p := path.Join(m.tempDir, d)

	if err := os.MkdirAll(p, bufferMode); err != nil {
		return fmt.Errorf("mkdir %s: %s", p, err)
	}

	fn := u.String() + ".json.gz"

	f, err := os.Create(path.Join(p, fn))
	if err != nil {
		return err
	}

	m.buckets[d] = bucket{fn, gzip.NewWriter(f), f}
	base := ctx.ApigeeBase()
	m.creds.Store(d, cred{ctx.Key(), ctx.Secret(), base.String()})
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
