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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/hashicorp/go-multierror"
)

// ensureStagingSpace ensures that staging has space for N new files
// if not, delete oldest files
func (m *manager) ensureStagingSpace(toStage int) error {
	paths, errs := m.getFilesInStaging()

	need := toStage - (m.stagingFileLimit - len(paths))
	if need <= 0 { // enough space, do nothing
		return errs
	}

	// Loop through deleting files in order of creation time until we have cleared
	// up enough space or until there is nothing left we can delete.
	// Note: this will start breaking on 2286-11-20 since we are sorting
	// lexicographically on a number.
	sort.Slice(paths, func(i, j int) bool {
		return filepath.Base(paths[i]) < filepath.Base(paths[j])
	})
	for _, p := range paths {
		if need <= 0 {
			break
		}

		m.log.Warningf("over staging limit, removing file %s", p)
		if err := os.Remove(p); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("rm %s: %s", p, err))
			continue
		}
		need--
	}

	return errs
}

func (m *manager) getFilesInStaging() ([]string, error) {
	tenantDirs, err := ioutil.ReadDir(m.stagingDir)
	if err != nil {
		return nil, fmt.Errorf("ReadDir(%s): %s", m.tempDir, err)
	}

	var errs error
	var filePaths []string
	for _, tenantDir := range tenantDirs {
		tenantDirPath := filepath.Join(m.stagingDir, tenantDir.Name())

		stagedFiles, err := ioutil.ReadDir(tenantDirPath)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("ls %s: %s", tenantDirPath, err))
			continue
		}

		for _, stagedFile := range stagedFiles {
			filePaths = append(filePaths, filepath.Join(tenantDirPath, stagedFile.Name()))
		}
	}
	return filePaths, errs
}

func (m *manager) stageAllBucketsWait() {
	wait := &sync.WaitGroup{}
	m.stageAllBuckets(wait)
	wait.Wait()
}

func (m *manager) stageAllBuckets(wait *sync.WaitGroup) {
	m.bucketsLock.Lock()
	buckets := m.buckets
	m.buckets = map[string]*bucket{}
	m.bucketsLock.Unlock()
	m.ensureStagingSpace(len(buckets))
	for tenant, bucket := range buckets {
		m.stageBucket(tenant, bucket, wait)
	}
}

func (m *manager) stageBucket(tenant string, b *bucket, wait *sync.WaitGroup) {
	stageDir := filepath.Join(b.manager.stagingDir, tenant)

	if wait != nil {
		wait.Add(1)
	}
	b.close(stageDir, wait)
}

func (m *manager) getStagedFiles(tenant string) ([]os.FileInfo, error) {
	m.stageLock.Lock()
	defer m.stageLock.Unlock()
	p := filepath.Join(m.stagingDir, tenant)
	f, err := ioutil.ReadDir(p)
	if err != nil {
		err = fmt.Errorf("ls %s: %s", p, err)
	}
	return f, err
}
