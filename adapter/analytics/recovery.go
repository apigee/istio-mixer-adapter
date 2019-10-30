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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
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
		files, err := ioutil.ReadDir(tempDir)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}

		// Ensure staging dir
		bufferMode := os.FileMode(0700)
		stageDir := m.getStagingDir(tenant)
		if err := os.MkdirAll(stageDir, bufferMode); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("mkdir %s: %s", stageDir, err))
			continue
		}

		// recover temp to staging
		for _, fi := range files {
			tempFile := filepath.Join(tempDir, fi.Name())
			stageFile := filepath.Join(stageDir, fi.Name())
			if err := m.ensureValidGzip(tempFile); err != nil {
				errs = multierror.Append(errs, err)
				if err != ErrGZIPRepaired {
					continue
				}
			}
			// file should be good, just move to staging
			if err := os.Rename(tempFile, stageFile); err != nil {
				errs = multierror.Append(errs, err)
			}
		}
	}
	return errs
}

// ErrGZIPRepaired is used to indicate a gzip was repaired
var ErrGZIPRepaired = errors.New("GZIP Repaired")
var errGZip = errors.New("GZIP Error")

// returns nil if was a good gzip, ErrGZIPRepaired if a bad gzip was repaired, other errors for unrecoverable
// may replace file, do not have it open
func (m *manager) ensureValidGzip(fileName string) error {
	err := m.validateGZip(fileName)
	if err != errGZip {
		return err
	}

	tempFile, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	tempFilePath := path.Join(os.TempDir(), tempFile.Name())
	defer os.Remove(tempFilePath)

	if err := m.recoverFile(fileName, tempFile); err != nil {
		return err
	}

	return ErrGZIPRepaired
}

// if gzip error, returns gzipError
func (m *manager) validateGZip(fileName string) error {
	// m.log.Debugf("validating gzip: %s", fileName)

	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		m.log.Errorf("gzip.NewReader(%s): %s", fileName, err)
		return err
	}
	defer gz.Close()

	b := make([]byte, 1000)
	for {
		if _, err := gz.Read(b); err != nil {
			if err == io.EOF {
				break
			}
			return errGZip
		}
	}
	return nil
}

// recoverFile recovers gzipped data in a file and puts it into a new file.
// It's OK if the file ends up with invalid JSON, the server will ignore.
func (m *manager) recoverFile(old string, new *os.File) error {
	m.log.Warningf("recovering bad gzip file: %s", old)
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

	// buffer size is arbitrary and doesn't really matter
	b := make([]byte, 1000)
	gzw := gzip.NewWriter(new)
	for {
		var nRead int
		if nRead, err = gzr.Read(b); err != nil {
			if err != io.EOF && err.Error() != "unexpected EOF" && err.Error() != "gzip: invalid header" {
				return fmt.Errorf("scan gzip %s: %s", old, err)
			}
		}
		gzw.Write(b[:nRead])
		if err != nil {
			break
		}
	}
	if err := gzw.Close(); err != nil {
		return fmt.Errorf("close gzw %s: %s", old, err)
	}
	if err := new.Close(); err != nil {
		return fmt.Errorf("close gzw file %s: %s", old, err)
	}

	m.log.Infof("bad gzip %s recovered to: %s", old, new.Name())
	return nil
}
