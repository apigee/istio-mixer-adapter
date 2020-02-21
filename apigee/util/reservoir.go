// Copyright 2019 Google LLC
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

package util

import "github.com/apigee/istio-mixer-adapter/apigee/log"

// NewReservoir sends from one channel to another without blocking until closed.
// Once "in" channel is closed, "out" will continue to drain before closing.
// if buffer limit is reached, new messages (LIFO) are sent to overflow: non-blocking, can be overrun.
func NewReservoir(limit int) (chan<- interface{}, <-chan interface{}, <-chan interface{}) {
	in := make(chan interface{})
	out := make(chan interface{})
	overflow := make(chan interface{}, 1)
	go func() {
		var reservoir []interface{}

		outChan := func() chan<- interface{} {
			if len(reservoir) == 0 {
				return nil // block if empty
			}
			return out
		}

		next := func() interface{} {
			if len(reservoir) == 0 {
				return nil // block if empty
			}
			return reservoir[0]
		}

		for len(reservoir) > 0 || in != nil {
			select {
			case v, ok := <-in:
				if ok {
					if len(reservoir) < limit {
						// log.Debugf("queue: %v (%d)", v, len(reservoir))
						reservoir = append(reservoir, v)
					} else {
						// no stopping here
						select {
						case overflow <- v:
							// log.Debugf("overflow: %v", v)
						default:
							log.Warningf("dropped: %v", v)
						}
					}
				} else {
					in = nil
				}
			case outChan() <- next():
				// log.Debugf("dequeue: %v", reservoir[0])
				reservoir = reservoir[1:]
			}
		}
		close(out)
		close(overflow)
	}()

	return in, out, overflow
}
