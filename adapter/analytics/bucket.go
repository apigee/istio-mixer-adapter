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
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
)

func newBucket(m *manager, tenant, dir string) (*bucket, error) {
	b := &bucket{
		manager:  m,
		tenant:   tenant,
		dir:      dir,
		incoming: make(chan []Record, m.sendChannelSize),
	}

	f, err := ioutil.TempFile(b.dir, fmt.Sprintf("%d-*.gz", b.manager.now().Unix()))
	if err != nil {
		m.log.Errorf("AX Records Lost. Can't create bucket file: %s", err)
		return nil, err
	}
	b.w = &writer{
		f:  f,
		gz: gzip.NewWriter(f),
	}

	m.env.ScheduleDaemon(b.runLoop)
	return b, nil
}

// A bucket writes analytics to a temp file
type bucket struct {
	manager  *manager
	tenant   string
	dir      string
	w        *writer
	incoming chan []Record
	wait     *sync.WaitGroup
}

// write records to bucket
func (b *bucket) write(records []Record) {
	if b != nil && len(records) > 0 {
		b.incoming <- records
	}
}

// close bucket
func (b *bucket) close(wait *sync.WaitGroup) {
	b.wait = wait
	close(b.incoming)
}

func (b *bucket) fileName() string {
	return b.w.f.Name()
}

func (b *bucket) runLoop() {
	log := b.manager.log

	for records := range b.incoming {
		if err := b.w.write(records); err != nil {
			log.Errorf("writing records: %s", err)
		}
	}

	if err := b.w.close(); err != nil {
		log.Errorf("Can't close bucket file: %s", err)
	}

	b.manager.stageFile(b.tenant, b.fileName())

	if b.wait != nil {
		b.wait.Done()
	}
	log.Debugf("bucket closed: %s", b.fileName())
}

type writer struct {
	gz *gzip.Writer
	f  *os.File
}

func (w *writer) write(records []Record) error {
	enc := json.NewEncoder(w.gz)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("json encode: %s", err)
		}
	}
	if err := w.gz.Flush(); err != nil {
		return fmt.Errorf("gz.Flush: %s", err)
	}
	return nil
}

func (w *writer) close() error {
	if err := w.gz.Close(); err != nil {
		return fmt.Errorf("gz.Close: %s", err)
	}
	if err := w.f.Close(); err != nil {
		return fmt.Errorf("f.Close: %s", err)
	}
	return nil
}
