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

	"istio.io/istio/mixer/pkg/adapter"
)

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

// A bucket keeps track of a tenant's analytics
type bucket struct {
	manager  *manager
	log      adapter.Logger
	dir      string // containing dir
	tenant   string // org~env
	w        *writer
	incoming chan []Record
	closer   chan closeReq
}

func (b *bucket) runLoop() {
	for {
		select {
		case records := <-b.incoming:
			if b.w == nil {
				f, err := ioutil.TempFile(b.dir, fmt.Sprintf("%d-", b.manager.now().Unix()))
				if err != nil {
					b.log.Errorf("AX Records Lost. Can't create bucket file: %s", err)
					return
				}
				b.w = &writer{
					f:  f,
					gz: gzip.NewWriter(f),
				}

				b.log.Debugf("new bucket created: %s", f.Name())
			}
			w := b.w
			if err := w.write(records); err != nil {
				b.log.Errorf("writing records: %s", err)
			} else {
				b.log.Debugf("%d records written to %s", len(records), b.w.f.Name())
			}
		case req := <-b.closer:
			if b.w != nil {
				if req.filename == "" || b.w.f.Name() == req.filename {
					b.w.close()
					b.log.Debugf("bucket file closed: %s", b.w.f.Name())
				}
				b.w = nil
			}
			if req.stop {
				b.log.Debugf("bucket loop closed")
				b.manager.closeWait.Done()
				return
			}
		}
	}
}

type closeReq struct {
	filename string
	stop     bool
}

func (b *bucket) write(records []Record) {
	b.incoming <- records
}

// will close bucket if passed filename is current file or ""
func (b *bucket) close(filename string) {
	b.closer <- closeReq{
		filename: filename,
	}
}

// exit bucket loop
func (b *bucket) stop() {
	b.closer <- closeReq{
		stop: true,
	}
}
