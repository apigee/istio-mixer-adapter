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
	"sort"

	"github.com/hashicorp/go-multierror"
)

// stageUpload moves anything in the temp dir to the staging dir
func (m *manager) stageUpload() error {
	subDirs, err := ioutil.ReadDir(m.tempDir)
	if err != nil {
		return fmt.Errorf("ReadDir(%s): %s", m.tempDir, err)
	}

	var errs error
	toMove := map[string]string{}
	for _, tenantDir := range subDirs {
		tenant := tenantDir.Name()

		tempDir := path.Join(m.tempDir, tenant)
		stageDir := path.Join(m.stagingDir, tenant)
		files, err := ioutil.ReadDir(tempDir)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("ioutil.ReadDir(%s): %s", tempDir, err))
			continue
		}
		if err := os.MkdirAll(stageDir, bufferMode); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("mkdir %s: %s", stageDir, err))
			continue
		}

		for _, f := range files {
			// close any files in use
			m.bucketsLock.RLock()
			if bucket := m.buckets[tenant]; bucket != nil {
				fn := path.Join(tempDir, f.Name())
				bucket.close(fn)
			}
			m.bucketsLock.RUnlock()

			oldFile := path.Join(tempDir, f.Name())
			newFile := path.Join(stageDir, f.Name())
			toMove[oldFile] = newFile
		}
	}

	if err := m.ensureStagingSpace(len(toMove)); err != nil {
		// Note error, but don't bail or the temp dir will grow without bounds
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

// ensureStagingSpace ensures that staging has space for N new files
// if not, delete oldest files
func (m *manager) ensureStagingSpace(n int) error {

	paths, errs := m.getFilesInStaging()

	if len(paths) <= m.bufferSize-n { // enough space, do nothing
		return errs
	}

	// Amount of space to create: how much we need - how much we have available.
	need := n - (m.bufferSize - len(paths))

	// Loop through deleting files in order of creation time until we have cleared
	// up enough space or until there is nothing left we can delete.
	// Note: this will start breaking on 2286-11-20 since we are sorting
	// lexicographically on a number.
	sort.Slice(paths, func(i, j int) bool {
		return path.Base(paths[i]) < path.Base(paths[j])
	})
	for _, p := range paths {
		if need <= 0 {
			break
		}

		if err := os.Remove(p); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("rm %s: %s", p, err))
			continue
		}
		need--
	}

	return errs
}

func (m *manager) getFilesInStaging() ([]string, error) {
	subdirs, err := ioutil.ReadDir(m.stagingDir)
	if err != nil {
		return nil, fmt.Errorf("ReadDir(%s): %s", m.tempDir, err)
	}

	var errs error
	var filePaths []string
	for _, subdir := range subdirs {
		p := path.Join(m.stagingDir, subdir.Name())

		files, err := ioutil.ReadDir(p)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("ls %s: %s", p, err))
			continue
		}

		for _, f := range files {
			filePaths = append(filePaths, path.Join(p, f.Name()))
		}
	}
	return filePaths, errs
}

// crashRecovery cleans up the temp and staging dirs post-crash. This function
// assumes that the temp dir exists and is accessible.
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
			if err := m.ensureValidGzip(tempFile); err != nil {
				errs = multierror.Append(errs, err)
				if err != GZIPRepaired {
					continue
				}
			}
			// file should be good, just move to staging
			if err := os.Rename(tempFile, stagingFile); err != nil {
				errs = multierror.Append(errs, err)
			}
		}
	}
	return errs
}

var GZIPError = errors.New("GZIP Error")
var GZIPRepaired = errors.New("GZIP Repaired")

// returns nil if was a good gzip, GZIPRepaired if a bad gzip was repaired, other errors for unrecoverable
// may replace file, do not have it open
func (m *manager) ensureValidGzip(fileName string) error {
	err := m.validateGZip(fileName)
	if err != GZIPError {
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

	return GZIPRepaired
}

// if gzip error, returns gzipError
func (m *manager) validateGZip(fileName string) error {

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
		if _, err = gz.Read(b); err != nil {
			if err == io.EOF {
				break
			}
			return GZIPError
		}
	}
	return nil
}

// recoverFile recovers gzipped data in a file and puts it into a new file.
// It's OK if the file ends up with invalid JSON, the server will ignore.
func (m *manager) recoverFile(old string, new *os.File) error {
	m.log.Infof("recovering bad gzip file: %s", old)
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

	m.log.Infof("bad gzip %s recovered to: %s", old, new)
	return nil
}
