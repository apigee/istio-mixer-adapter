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
	"os"
	"path"
	"sync"

	"github.com/google/uuid"
	"istio.io/istio/mixer/pkg/adapter"
)

type writer struct {
	gz   *gzip.Writer
	f    *os.File
	lock sync.Mutex
}

func (w *writer) write(records []Record) error {
	w.lock.Lock()
	defer w.lock.Unlock()
	if err := json.NewEncoder(w.gz).Encode(records); err != nil {
		return fmt.Errorf("json encode: %s", err)
	}
	if err := w.gz.Flush(); err != nil {
		return fmt.Errorf("gz.Flush: %s", err)
	}
	return nil
}

func (w *writer) close() error {
	w.lock.Lock()
	defer w.lock.Unlock()
	if err := w.gz.Close(); err != nil {
		return fmt.Errorf("gz.Close: %s", err)
	}
	if err := w.f.Close(); err != nil {
		return fmt.Errorf("f.Close: %s", err)
	}
	return nil
}

// A bucket keeps track of a tenant's analytics
type bucket struct {
	manager  *manager
	log      adapter.Logger
	dir      string // containing dir
	tenant   string // org~env
	w        *writer
	incoming chan []Record
	closer   chan string
}

func (b *bucket) runLoop() {
	for {
		select {
		case records := <-b.incoming:
			if b.w == nil {
				u, err := uuid.NewRandom()
				if err != nil {
					b.log.Errorf("AX Records Lost. uuid.Random(): %s", err)
					return
				}

				// Use the timestamp as the prefix to sort by creation time
				fn := fmt.Sprintf("%d_%s.json.gz", b.manager.now().Unix(), u.String())

				// create new bucket file
				f, err := os.Create(path.Join(b.dir, fn))
				if err != nil {
					b.log.Errorf("AX Records Lost. Can't create bucket file: %s", err)
					return
				}
				b.w = &writer{
					f:  f,
					gz: gzip.NewWriter(f),
				}

				b.log.Debugf("new bucket file created: %s", f.Name())
			}
			if err := b.w.write(records); err != nil {
				b.log.Errorf("writing records: %s", err)
			}
		case filename := <-b.closer:
			if b.w != nil {
				if filename == "" || b.w.f.Name() == filename {
					b.w.close()
					b.log.Debugf("bucket file closed: %s", b.w.f.Name())
				}
				b.w = nil
			}
			return
		}
	}
}

func (b *bucket) write(records []Record) {
	b.incoming <- records
}

// will close if passed filename is current file, pass "" to force close
func (b *bucket) close(filename string) {
	b.closer <- filename
}
