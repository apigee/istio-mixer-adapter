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

import (
	"context"
	"time"

	"istio.io/istio/mixer/pkg/adapter"
)

// Looper provides for Backoff and cancellation
type Looper struct {
	Env     adapter.Env
	Backoff Backoff
}

// Start a daemon that repeatedly calls work function according to period.
// Passed ctx should be cancelable - to exit, cancel the Context.
// Passed ctx is passed on to the work function and work should check for cancel if long-running.
// If errHandler itself returns an error, the daemon will exit.
func (a *Looper) Start(ctx context.Context, work func(ctx context.Context) error, period time.Duration, errHandler func(error) error) {
	// a.Env.Logger().Debugf("Looper starting")
	run := time.After(0 * time.Millisecond) // start first run immediately
	// log := a.Env.Logger()

	a.Env.ScheduleDaemon(func() {
		for {
			select {
			case <-ctx.Done():
				// log.Debugf("Looper exiting")
				return
			case <-run:
				// log.Debugf("Looper work running")
				err := work(ctx)
				if ctx.Err() != nil {
					return
				}
				var nextRunIn time.Duration
				if err == nil {
					a.Backoff.Reset()
					nextRunIn = period
				} else {
					if errHandler(err) != nil {
						// log.Debugf("Looper quit on error")
						return
					}
					nextRunIn = a.Backoff.Duration()
				}
				run = time.After(nextRunIn)
				// log.Debugf("Looper work scheduled to run in %s", nextRunIn)
			}
		}
	})
}
