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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/util"
	"istio.io/istio/mixer/pkg/adapter"
)

type uploader interface {
	workFunc(tenant, fileName string) util.WorkFunc
	write(records []Record, writer io.Writer) error
	isGzipped() bool
}

type saasUploader struct {
	log     adapter.Logger
	client  *http.Client
	baseURL *url.URL
	key     string
	secret  string
	now     func() time.Time
}

func (s *saasUploader) isGzipped() bool {
	return true
}

// format and write records
func (s *saasUploader) write(records []Record, writer io.Writer) error {
	enc := json.NewEncoder(writer)
	for _, record := range records {
		if err := enc.Encode(record); err != nil {
			return fmt.Errorf("json encode: %s", err)
		}
	}
	return nil
}

func (s *saasUploader) workFunc(tenant, fileName string) util.WorkFunc {
	return func(ctx context.Context) error {
		if ctx.Err() == nil {
			return s.upload(tenant, fileName)
		}

		s.log.Warningf("canceled upload of %s: %v", fileName, ctx.Err())
		err := os.Remove(fileName)
		if err != nil && !os.IsNotExist(err) {
			s.log.Warningf("unable to remove file %s: %v", fileName, err)
		}
		return nil
	}
}

// upload sends a file to SaaS UAP
func (s *saasUploader) upload(tenant, fileName string) error {

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}

	fi, err := file.Stat()
	if err != nil {
		return err
	}

	s.log.Debugf("getting signed URL for %s", fileName)
	signedURL, err := s.signedURL(tenant, fileName)
	if err != nil {
		return fmt.Errorf("signedURL: %s", err)
	}
	req, err := http.NewRequest("PUT", signedURL, file)
	if err != nil {
		return fmt.Errorf("http.NewRequest: %s", err)
	}

	req.Header.Set("Expect", "100-continue")
	req.Header.Set("Content-Type", "application/x-gzip")
	req.Header.Set("x-amz-server-side-encryption", "AES256")
	req.ContentLength = fi.Size()

	s.log.Debugf("uploading %s to %s", fileName, signedURL)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("client.Do(): %s", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("upload %s returned %s", fileName, resp.Status)
	}

	if err := os.Remove(fileName); err != nil {
		return fmt.Errorf("rm %s: %s", fileName, err)
	}

	return nil
}

func (s *saasUploader) orgEnvFromSubdir(subdir string) (string, string) {
	splits := strings.Split(subdir, "~")
	if len(splits) == 2 {
		return splits[0], splits[1]
	}
	return "", ""
}

// uploadDir gets a directory for where we should upload the file.
func (s *saasUploader) uploadDir() string {
	now := s.now()
	d := now.Format("2006-01-02")
	t := now.Format("15-04-00")
	return fmt.Sprintf(pathFmt, d, t)
}

// signedURL asks for a signed URL that can be used to upload gzip file
func (s *saasUploader) signedURL(subdir, fileName string) (string, error) {
	org, env := s.orgEnvFromSubdir(subdir)
	if org == "" || env == "" {
		return "", fmt.Errorf("invalid subdir %s", subdir)
	}

	ur := *s.baseURL
	ur.Path = path.Join(ur.Path, fmt.Sprintf(analyticsPath, org, env))
	req, err := http.NewRequest("GET", ur.String(), nil)
	if err != nil {
		return "", err
	}

	relPath := filepath.Join(s.uploadDir(), filepath.Base(fileName))

	q := req.URL.Query()
	q.Add("tenant", subdir)
	q.Add("relative_file_path", relPath)
	q.Add("file_content_type", "application/x-gzip")
	q.Add("encrypt", "true")
	req.URL.RawQuery = q.Encode()

	req.SetBasicAuth(s.key, s.secret)

	resp, err := s.client.Do(req)
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
