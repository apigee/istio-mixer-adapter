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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/hashicorp/go-multierror"
)

// uploadAll commits everything from staging and then uploads it
func (m *manager) uploadAll() error {
	if err := m.stageUpload(); err != nil {
		m.log.Errorf("Error moving analytics into staging dir: %s", err)
		// Don't return here, we may have committed some dirs and will want to upload them
	}

	tenants, err := ioutil.ReadDir(m.stagingDir)
	if err != nil {
		return fmt.Errorf("ReadDir(%s): %s", m.stagingDir, err)
	}

	var errOut error
	// TODO: If this is slow, use a pool of goroutines to upload
	for _, tenant := range tenants {
		if err := m.upload(tenant.Name()); err != nil {
			errOut = multierror.Append(errOut, err)
			if m.shortCircuitErr(err) {
				return errOut
			}
		}
	}
	return errOut
}

// upload sends all the files in a given staging subdir to UAP.
func (m *manager) upload(tenant string) error {
	p := path.Join(m.stagingDir, tenant)
	files, err := ioutil.ReadDir(p)
	if err != nil {
		return fmt.Errorf("ls %s: %s", p, err)
	}
	var errs error
	successes := 0
	for _, fi := range files {
		fn := path.Join(p, fi.Name())
		f, err := os.Open(fn)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}

		// bad files shouldn't happen here, but bad things happen on the server if they do
		// so pedantically ensure absolutely, positively no bad gzip files end up on the server
		err = m.ensureValidGzip(fn)
		if err != nil && err != GZIPRepaired {
			errs = multierror.Append(errs,
				fmt.Errorf("unrecoverable gzip in staging (%s), removing: %s", fn, err))
			if err := os.Remove(fn); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("failed remove: %s", err))
			}
			continue
		}

		signedURL, err := m.signedURL(tenant, fi.Name())
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("signedURL: %s", err))
			if m.shortCircuitErr(err) {
				return errs
			}
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
			errs = multierror.Append(errs,
				fmt.Errorf("push %s/%s to store returned %v", p, fi.Name(), resp.Status))
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
