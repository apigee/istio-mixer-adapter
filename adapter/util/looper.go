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

// WorkFunc does work
type WorkFunc func(ctx context.Context) error

// ErrorFunc handles errors
type ErrorFunc func(error) error

// LogErrorsHandler just logs errors and continues
func LogErrorsHandler(env adapter.Env) ErrorFunc {
	return func(err error) error {
		env.Logger().Errorf("looper: %v", err)
		return nil
	}
}

// Start a daemon that repeatedly calls work function according to period.
// Passed ctx should be cancelable - to exit, cancel the Context.
// Passed ctx is passed on to the work function and work should check for cancel if long-running.
// If errHandler itself returns an error, the daemon will exit.
func (l *Looper) Start(ctx context.Context, work WorkFunc, period time.Duration, errHandler ErrorFunc) {
	// l.Env.Logger().Debugf("Looper starting")
	run := time.After(0 * time.Millisecond) // start first run immediately

	l.Env.ScheduleDaemon(func() {
		for {
			select {
			case <-ctx.Done():
				// l.Env.Logger().Debugf("Looper exiting")
				return
			case <-run:
				// l.Env.Logger().Debugf("Looper work running")
				err := l.Run(ctx, work, errHandler)
				if err != nil {
					return
				}
				run = time.After(period)
			}
		}
	})
}

// Run the work until successful (or ctx canceled) with backoff.
// Passed ctx should be cancelable - to exit, cancel the Context.
// Passed ctx is passed on to the work function and work should check for cancel if long-running.
// If errHandler itself returns an error, the daemon will exit.
func (l *Looper) Run(ctx context.Context, work WorkFunc, errHandler ErrorFunc) error {
	run := time.After(0 * time.Millisecond) // start immediately
	for {
		select {
		case <-ctx.Done():
			// l.Env.Logger().Debugf("Looper exiting")
			return nil
		case <-run:
			// l.Env.Logger().Debugf("Looper work running")
			err := work(ctx)
			if err == nil || ctx.Err() != nil {
				return nil
			}

			if err := errHandler(err); err != nil {
				// l.Env.Logger().Debugf("Looper quit on error")
				return err
			}

			run = time.After(l.Backoff.Duration())
			// l.Env.Logger().Debugf("Looper work scheduled to run in %s", nextRunIn)
		}
	}
}

// Chan pulls work from work channel until channel is closed of Context canceled.
// Passed ctx is passed on to the work function and work should check for cancel if long-running.
// If errHandler itself returns an error, the daemon will exit.
func (l *Looper) Chan(ctx context.Context, work <-chan (WorkFunc), errHandler ErrorFunc) {
	for {
		select {
		case <-ctx.Done():
			// l.Env.Logger().Debugf("Looper exiting")
			return
		case work, ok := <-work:
			if !ok {
				// l.Env.Logger().Debugf("Looper channel close")
				return
			}
			// l.Env.Logger().Debugf("Looper work running")
			err := l.Run(ctx, work, errHandler)
			if err != nil {
				return
			}
		}
	}
}

// NewChannelWithWorkerPool returns a channel to send work to and the set of workers.
// Close the returned channel or cancel the Context and the workers will exit.
// Passed ctx is passed on to the work function and work should check for cancel if long-running.
// Beware: If errHandler itself returns an error, the worker will exit.
func NewChannelWithWorkerPool(ctx context.Context, size int, env adapter.Env, errHandler ErrorFunc, backoff Backoff) chan WorkFunc {
	channel := make(chan WorkFunc)
	for i := 0; i < size; i++ {
		l := Looper{
			Env:     env,
			Backoff: backoff.Clone(),
		}
		env.ScheduleDaemon(func() {
			l.Chan(ctx, channel, errHandler)
		})
	}
	return channel
}
