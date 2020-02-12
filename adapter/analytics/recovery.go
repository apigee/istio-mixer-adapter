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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
)

// crashRecovery cleans up the temp and staging dirs post-crash. This function
// assumes that the temp dir exists and is accessible.
func (m *manager) crashRecovery() error {
	dirs, err := ioutil.ReadDir(m.tempDir)
	if err != nil {
		return err
	}
	var errs error
	for _, d := range dirs {
		tenant := d.Name()
		tempDir := m.getTempDir(tenant)
		tempFiles, err := ioutil.ReadDir(tempDir)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}

		m.prepTenant(tenant)
		stageDir := m.getStagingDir(tenant)

		// put staged files in upload queue
		stagedFiles, err := m.getFilesInStaging()
		for _, fi := range stagedFiles {
			m.upload(tenant, fi)
		}

		// recover temp to staging and upload
		for _, fi := range tempFiles {
			tempFile := filepath.Join(tempDir, fi.Name())
			stageFile := filepath.Join(stageDir, fi.Name())

			dest, err := os.Create(stageFile)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("create recovery file %s: %s", tempDir, err))
				continue
			}
			if err := m.recoverFile(tempFile, dest); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("recoverFile %s: %s", tempDir, err))
				if err := os.Remove(stageFile); err != nil {
					errs = multierror.Append(errs, fmt.Errorf("remove stage file %s: %s", tempDir, err))
				}
				continue
			}

			if err := os.Remove(tempFile); err != nil {
				m.log.Warningf("unable to remove temp file: %s", tempFile)
			}

			m.upload(tenant, stageFile)
		}
	}
	return errs
}

// recoverFile recovers gzipped data in a file and puts it into a new file.
func (m *manager) recoverFile(oldName string, newFile *os.File) error {
	m.log.Warningf("recover file: %s", oldName)
	in, err := os.Open(oldName)
	if err != nil {
		return fmt.Errorf("open %s: %s", oldName, err)
	}
	br := bufio.NewReader(in)
	gzr, err := gzip.NewReader(br)
	if err != nil {
		return fmt.Errorf("gzip.NewReader(%s): %s", oldName, err)
	}
	defer gzr.Close()

	// buffer size is arbitrary and doesn't really matter
	b := make([]byte, 1000)
	gzw := gzip.NewWriter(newFile)
	for {
		var nRead int
		if nRead, err = gzr.Read(b); err != nil {
			if err != io.EOF && err.Error() != "unexpected EOF" && err.Error() != "gzip: invalid header" {
				return fmt.Errorf("scan gzip %s: %s", oldName, err)
			}
		}
		gzw.Write(b[:nRead])
		if err != nil {
			break
		}
	}
	if err := gzw.Close(); err != nil {
		return fmt.Errorf("close gzw %s: %s", oldName, err)
	}
	if err := newFile.Close(); err != nil {
		return fmt.Errorf("close gzw file %s: %s", oldName, err)
	}

	m.log.Infof("%s recovered to: %s", oldName, newFile.Name())
	return nil
}
