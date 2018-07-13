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
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"istio.io/istio/mixer/pkg/adapter"
)

const (
	analyticsPath = "/analytics/organization/%s/environment/%s"
	axRecordType  = "APIAnalytics"
	pathFmt       = "date=%s/time=%d-%d/"
	bufferMode    = os.FileMode(0700)
	tempDir       = "temp"
	stagingDir    = "staging"

	// This is a list of errors that the signedURL endpoint will return.
	errUnauth     = "401 Unauthorized" // Auth credentials are wrong.
	errNotFound   = "404 Not Found"    // Base URL is wrong.
	errApigeeDown = "code 50"          // Internal Apigee issue.

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

// A manager is a way for Istio to interact with Apigee's analytics platform.
type manager struct {
	close              chan bool
	closed             chan bool
	client             *http.Client
	now                func() time.Time
	log                adapter.Logger
	collectionInterval time.Duration
	tempDir            string // open gzip files being written to
	stagingDir         string // gzip files staged for upload
	bufferSize         int
	bucketsLock        sync.RWMutex
	buckets            map[string]bucket // Map from dirname -> bucket.
	baseURL            url.URL
	key                string
	secret             string
}

// Options allows us to specify options for how this analytics manager will run.
type Options struct {
	// LegacyEndpoint is true if using older direct-submit protocol
	LegacyEndpoint bool
	// BufferPath is the directory where the adapter will buffer analytics records.
	BufferPath string
	// BufferSize is the maximum number of files stored in the staging directory.
	// Once this is reached, the oldest files will start being removed.
	BufferSize int
	// Base Apigee URL
	BaseURL url.URL
	// Key for submit auth
	Key string
	// Secret for submit auth
	Secret string
	// Client is a configured HTTPClient
	Client *http.Client
}

func (o *Options) validate() error {
	if o.BufferPath == "" ||
		o.BufferSize <= 0 ||
		o.Key == "" ||
		o.Client == nil ||
		o.Secret == "" {
		return fmt.Errorf("all analytics options are required")
	}
	return nil
}

// crashRecovery cleans up the temp and staging dirs post-crash. This function
// assumes that both the temp and staging dirs exist and are accessible.
func (m *manager) crashRecovery() error {
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

		// Ensure staging dir
		p := path.Join(m.stagingDir, bucket)
		if err := os.MkdirAll(p, bufferMode); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("mkdir %s: %s", p, err))
			continue
		}

		// recover temp to staging
		for _, fi := range files {
			tempFile := path.Join(m.tempDir, bucket, fi.Name())
			stagingFile := path.Join(p, fi.Name())
			f, err := os.Open(tempFile)
			if err != nil {
				errs = multierror.Append(errs, err)
				continue
			}
			gz, err := gzip.NewReader(f)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("gzip.NewReader(%s): %s", fi.Name(), err))
				f.Close()
				continue
			}
			if _, err := ioutil.ReadAll(gz); err != nil {
				gz.Close()
				f.Close()
				errs = multierror.Append(errs,
					fmt.Errorf("readall(%s): %s. attempting recovery", fi.Name(), err))
				if err := m.recoverFile(tempFile, stagingFile); err != nil {
					errs = multierror.Append(errs, err)
				}
				continue
			}
			gz.Close()
			f.Close()
			if err := os.Rename(tempFile, stagingFile); err != nil {
				errs = multierror.Append(errs, err)
			}
		}
	}
	return errs
}

// recoverFile recovers gzipped data in a file and puts it into a new file.
func (m *manager) recoverFile(old, new string) error {
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

	// The size of this buffer is arbitrary and doesn't really matter
	b := make([]byte, 1000)
	var gzw *gzip.Writer
	for {
		var nRead int
		if nRead, err = gzr.Read(b); err != nil {
			if err.Error() != "unexpected EOF" && err.Error() != "EOF" {
				return fmt.Errorf("scan %s: %s", old, err)
			}
			break
		}
		if nRead > 0 {
			if gzw == nil {
				out, err := os.Create(new)
				if err != nil {
					return fmt.Errorf("create %s: %s", new, err)
				}
				defer out.Close()
				gzw = gzip.NewWriter(out)
				defer gzw.Close()
			}
			gzw.Write(b)
		}
	}
	return nil
}

// Start starts the manager.
func (m *manager) Start(env adapter.Env) {
	m.log = env.Logger()
	m.log.Infof("starting analytics manager")

	if err := m.crashRecovery(); err != nil {
		m.log.Errorf("Error(s) recovering crashed data: %s", err)
	}

	env.ScheduleDaemon(func() {
		m.uploadLoop()
	})
	m.log.Infof("started analytics manager")
}

// Close shuts down the manager.
func (m *manager) Close() {
	if m == nil {
		return
	}
	m.log.Infof("closing analytics manager")
	m.close <- true
	if err := m.uploadAll(); err != nil {
		m.log.Errorf("Error pushing analytics: %s", err)
	}
	<-m.closed
	m.log.Infof("closed analytics manager")
}

// uploadLoop runs a timer that periodically pushes everything in the buffer
// directory to the server.
func (m *manager) uploadLoop() {
	t := time.NewTicker(m.collectionInterval)
	for {
		select {
		case <-t.C:
			if err := m.uploadAll(); err != nil {
				m.log.Errorf("Error pushing analytics: %s", err)
			}
		case <-m.close:
			m.log.Debugf("analytics close signal received, shutting down")
			t.Stop()
			m.closed <- true
			return
		}
	}
}

// commitStaging moves anything in the temp dir to the staging dir.
func (m *manager) commitStaging() error {
	subdirs, err := ioutil.ReadDir(m.tempDir)
	if err != nil {
		return fmt.Errorf("ReadDir(%s): %s", m.tempDir, err)
	}

	var errs error
	toMove := map[string]string{}
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
		oldPath := path.Join(m.tempDir, sn)
		newPath := path.Join(m.stagingDir, sn)
		files, err := ioutil.ReadDir(oldPath)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("ioutil.ReadDir(%s): %s", oldPath, err))
			continue
		}
		if err := os.MkdirAll(newPath, bufferMode); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("mkdir %s: %s", newPath, err))
			continue
		}
		for _, f := range files {
			oldPath := path.Join(oldPath, f.Name())
			newPath := path.Join(newPath, f.Name())
			toMove[oldPath] = newPath
		}
	}

	if err := m.ensureStagingSpace(len(toMove)); err != nil {
		// Don't bail here or the temp dir will grow without bounds. Copy the files
		// over anyway.
		errs = multierror.Append(errs, fmt.Errorf("cleanupStaging: %s", err))
	}

	successes := 0
	for oldPath, newPath := range toMove {
		if err := os.Rename(oldPath, newPath); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("mv %s: %s", oldPath, err))
			continue
		}
		successes++
	}
	if successes > 0 {
		m.log.Debugf("committed %d analytics packages to staging to be uploaded", successes)
	}
	return errs
}

// ensureStagingSpace ensures that staging has space for N new files.
func (m *manager) ensureStagingSpace(n int) error {
	// TODO(someone): handle case when n > m.bufferSize.

	// Figure out how many files are already in staging.
	subdirs, err := ioutil.ReadDir(m.stagingDir)
	if err != nil {
		return fmt.Errorf("ReadDir(%s): %s", m.tempDir, err)
	}

	var errs error
	var names []string
	fullPath := map[string]string{}
	for _, subdir := range subdirs {
		p := path.Join(m.stagingDir, subdir.Name())

		files, err := ioutil.ReadDir(p)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("ls %s: %s", p, err))
			continue
		}

		for _, f := range files {
			names = append(names, f.Name())
			fullPath[f.Name()] = path.Join(p, f.Name())
		}
	}

	if len(names) <= m.bufferSize-n {
		// We've already got enough space in staging, so don't do anything.
		return errs
	}

	// Amount of space to create: how much we need - how much we have available.
	need := n - (m.bufferSize - len(names))

	// Loop through deleting files in order of creation time until we have cleared
	// up enough space or until there is nothing left we can delete.
	// Note: this will start breaking on 2286-11-20 since we are sorting
	// lexicographically on a number.
	sort.Strings(names)
	for _, f := range names {
		if need <= 0 {
			break
		}

		fn, ok := fullPath[f]
		if !ok {
			// This is really weird, but don't attempt to delete something if we
			// don't know where it is.
			continue
		}
		if err := os.Remove(fn); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("rm %s: %s", fn, err))
			continue
		}
		need--
	}

	return errs
}

// shortCircuitErr checks if we should bail early on this error (i.e. an error
// that will be the same for all requests, like an auth fail or Apigee is down).
func (m *manager) shortCircuitErr(err error) bool {
	s := err.Error()
	return strings.Contains(s, errUnauth) ||
		strings.Contains(s, errNotFound) ||
		strings.Contains(s, errApigeeDown)
}

// uploadAll commits everything from staging and then uploads it.
func (m *manager) uploadAll() error {
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
			if m.shortCircuitErr(err) {
				return errOut
			}
		}
	}
	return errOut
}

// upload sends all the files in a given staging subdir to UAP.
func (m *manager) upload(subdir string) error {
	p := path.Join(m.stagingDir, subdir)
	files, err := ioutil.ReadDir(p)
	if err != nil {
		return fmt.Errorf("ls %s: %s", p, err)
	}
	var errs error
	successes := 0
	for _, fi := range files {
		signedURL, err := m.signedURL(subdir, fi.Name())
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("signedURL: %s", err))
			if m.shortCircuitErr(err) {
				return errs
			}
			continue
		}
		fn := path.Join(p, fi.Name())
		f, err := os.Open(fn)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}

		req, err := http.NewRequest("PUT", signedURL, f)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("http.NewRequest: %s", err))
			continue
		}

		req.Header.Set("Expect", "100-continue")
		req.Header.Set("Content-Type", "application/x-gzip")
		req.Header.Set("x-amz-server-side-encryption", "AES256")
		req.ContentLength = fi.Size()

		m.log.Debugf("uploading analytics package: %s to: %s", fn, signedURL)
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
	if successes > 0 {
		m.log.Debugf("uploaded %d analytics packages.", successes)
	}
	return errs
}

func (m *manager) orgEnvFromSubdir(subdir string) (string, string) {
	s := strings.Split(subdir, "~")
	if len(s) == 2 {
		return s[0], s[1]
	}
	return "", ""
}

// signedURL constructs a signed URL that can be used to upload records.
func (m *manager) signedURL(subdir, filename string) (string, error) {
	org, env := m.orgEnvFromSubdir(subdir)
	if org == "" || env == "" {
		return "", fmt.Errorf("invalid subdir %s", subdir)
	}

	u := m.baseURL
	u.Path = path.Join(u.Path, fmt.Sprintf(analyticsPath, org, env))
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return "", err
	}

	q := req.URL.Query()
	q.Add("tenant", subdir)
	q.Add("relative_file_path", path.Join(m.uploadDir(), filename))
	q.Add("file_content_type", "application/x-gzip")
	q.Add("encrypt", "true")
	req.URL.RawQuery = q.Encode()

	req.SetBasicAuth(m.key, m.secret)

	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status (code %d) returned from %s: %s", resp.StatusCode, u.String(), resp.Status)
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
func (m *manager) uploadDir() string {
	now := m.now()
	d := now.Format("2006-01-02")
	start := now.Unix()
	end := now.Add(m.collectionInterval).Unix()
	return fmt.Sprintf(pathFmt, d, start, end)
}

// SendRecords sends the records asynchronously to the UAP primary server.
func (m *manager) SendRecords(ctx *auth.Context, records []Record) error {
	EnsureFields(ctx, records)

	// Validate the records.
	var goodRecords []Record
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

func (m *manager) bucketDir(ctx *auth.Context) string {
	return fmt.Sprintf("%s~%s", ctx.Organization(), ctx.Environment())
}

func (m *manager) createBucket(ctx *auth.Context, d string) error {
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

	// Use the timestamp as the prefix so we can sort them easily by creation time.
	fn := fmt.Sprintf("%d_%s.json.gz", m.now().Unix(), u.String())

	f, err := os.Create(path.Join(p, fn))
	if err != nil {
		return err
	}

	m.buckets[d] = bucket{fn, gzip.NewWriter(f), f}
	return nil
}

// validate confirms that a record has correct values in it.
func (m *manager) validate(r Record) error {
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

// EnsureFields makes sure all the records in a list have the fields they need.
func EnsureFields(ctx *auth.Context, records []Record) {
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
