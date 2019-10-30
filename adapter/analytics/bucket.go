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
	"path/filepath"
	"sync"
)

func newBucket(m *manager, dir string) *bucket {
	b := &bucket{
		manager:  m,
		dir:      dir,
		incoming: make(chan []Record, m.sendChannelSize),
		closer:   make(chan closeReq),
	}
	m.env.ScheduleDaemon(b.runLoop)
	return b
}

// A bucket writes analytics to a temp file
type bucket struct {
	manager  *manager
	dir      string
	w        *writer
	incoming chan []Record
	closer   chan closeReq
}

// write records to bucket
func (b *bucket) write(records []Record) {
	if b != nil && len(records) > 0 {
		b.incoming <- records
	}
}

// close bucket
func (b *bucket) close(moveTo string, wait *sync.WaitGroup) {
	b.closer <- closeReq{
		wait:   wait,
		moveTo: moveTo,
	}
}

func (b *bucket) runLoop() {
	log := b.manager.log

	for {
		select {

		// write records
		case records := <-b.incoming:

			// lazy create file
			if b.w == nil {
				f, err := ioutil.TempFile(b.dir, fmt.Sprintf("%d-", b.manager.now().Unix()))
				if err != nil {
					log.Errorf("AX Records Lost. Can't create bucket file: %s", err)
					return
				}
				b.w = &writer{
					f:  f,
					gz: gzip.NewWriter(f),
				}

				log.Debugf("new bucket created: %s", f.Name())
			}

			if err := b.w.write(records); err != nil {
				log.Errorf("writing records: %s", err)
			} else {
				// log.Debugf("%d records written to %s", len(records), b.w.f.Name())
			}

		// close and move to staging
		case req := <-b.closer:
			log.Debugf("closing bucket: %s", b.w.f.Name())

			if b.w != nil {
				if err := b.w.close(); err != nil {
					log.Errorf("Can't close bucket file: %s", err)
				}
			}

			newFile := filepath.Join(req.moveTo, filepath.Base(b.w.f.Name()))
			if err := os.Rename(b.w.f.Name(), newFile); err != nil {
				log.Errorf("can't rename file: %s", err)
			} else {
				b.manager.log.Debugf("staged file: %s", newFile)
			}

			if req.wait != nil {
				req.wait.Done()
			}

			return
		}
	}
}

type closeReq struct {
	wait   *sync.WaitGroup
	moveTo string
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
