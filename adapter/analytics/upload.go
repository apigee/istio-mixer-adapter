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

// uploadAll uploads everything in staging
func (m *manager) uploadAll() error {
	tenants, err := ioutil.ReadDir(m.stagingDir)
	if err != nil {
		return fmt.Errorf("ReadDir(%s): %s", m.stagingDir, err)
	}

	var errOut error
	for _, tenant := range tenants {
		if err := m.uploadTenant(tenant.Name()); err != nil {
			errOut = multierror.Append(errOut, err)
			if m.shortCircuitErr(err) {
				return errOut
			}
		}
	}
	return errOut
}

// upload sends all the files in a given staging subdir to UAP.
func (m *manager) uploadTenant(tenant string) error {
	files, err := m.getStagedFiles(tenant)
	if err != nil || len(files) == 0 {
		return err
	}
	m.log.Debugf("uploading %d files for tenant %s", len(files), tenant)

	var errs error
	var lastFn string
	successes := 0
	p := path.Join(m.stagingDir, tenant)
	for _, fi := range files {
		fn := path.Join(p, fi.Name())
		if fn == lastFn {
			continue
		}
		lastFn = fn
		u := uploader{
			m: m,
		}
		up := upload{
			tenant: tenant,
			fn:     fn,
			fi:     fi,
		}
		err := u.upload(up)
		if err != nil {
			errs = multierror.Append(errs, err)
			if m.shortCircuitErr(err) {
				return errs
			}
		} else {
			successes++
		}
	}
	if successes > 0 {
		m.log.Debugf("uploaded %d analytics packages.", successes)
	}
	return errs
}

type uploader struct {
	m *manager
}

type upload struct {
	tenant string
	fn     string
	fi     os.FileInfo
}

// todo
func (u *uploader) start() {
	up := <-uploadQueue
	u.upload(up)
}

// upload sends a file to UAP
func (u *uploader) upload(up upload) error {
	tenant := up.tenant
	fn := up.fn
	fi := up.fi

	m := u.m

	f, err := os.Open(fn)
	if err != nil {
		return err
	}

	// // bad files shouldn't happen here, but bad things happen on the server if they do
	// // so pedantically ensure absolutely, positively no bad gzip files end up on the server
	// err = m.ensureValidGzip(fn)
	// if err != nil && err != ErrGZIPRepaired {
	// 	errs = multierror.Append(errs,
	// 		fmt.Errorf("unrecoverable gzip in staging (%s), removing: %s", fn, err))
	// 	if err := os.Remove(fn); err != nil {
	// 		errs = multierror.Append(errs, fmt.Errorf("failed remove: %s", err))
	// 	}
	// 	continue
	// }

	nameWithSuffix := fi.Name() + ".gz" // suffix needed by Apigee processor
	signedURL, err := u.signedURL(tenant, nameWithSuffix)
	if err != nil {
		return fmt.Errorf("signedURL: %s", err)
	}
	req, err := http.NewRequest("PUT", signedURL, f)
	if err != nil {
		return fmt.Errorf("http.NewRequest: %s", err)
	}

	req.Header.Set("Expect", "100-continue")
	req.Header.Set("Content-Type", "application/x-gzip")
	req.Header.Set("x-amz-server-side-encryption", "AES256")
	req.ContentLength = fi.Size()

	m.log.Debugf("uploading analytics: %s to: %s", fn, signedURL)
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("client.Do(): %s", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("upload %s returned %s", f.Name(), resp.Status)
	}

	if err := os.Remove(fn); err != nil {
		return fmt.Errorf("rm %s: %s", fn, err)
	}

	return nil
}

func (u *uploader) orgEnvFromSubdir(subdir string) (string, string) {
	s := strings.Split(subdir, "~")
	if len(s) == 2 {
		return s[0], s[1]
	}
	return "", ""
}

// uploadDir gets a directory for where we should upload the file.
func (u *uploader) uploadDir() string {
	now := u.m.now()
	d := now.Format("2006-01-02")
	t := now.Format("15-04-00")
	return fmt.Sprintf(pathFmt, d, t)
}

// signedURL constructs a signed URL that can be used to upload records.
func (u *uploader) signedURL(subdir, filename string) (string, error) {
	m := u.m

	org, env := u.orgEnvFromSubdir(subdir)
	if org == "" || env == "" {
		return "", fmt.Errorf("invalid subdir %s", subdir)
	}

	ur := m.baseURL
	ur.Path = path.Join(ur.Path, fmt.Sprintf(analyticsPath, org, env))
	req, err := http.NewRequest("GET", ur.String(), nil)
	if err != nil {
		return "", err
	}

	q := req.URL.Query()
	q.Add("tenant", subdir)
	q.Add("relative_file_path", path.Join(u.uploadDir(), filename))
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
		return "", fmt.Errorf("status (code %d) returned from %s: %s", resp.StatusCode, ur.String(), resp.Status)
	}

	var data struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("error decoding response: %s", err)
	}
	return data.URL, nil
}
