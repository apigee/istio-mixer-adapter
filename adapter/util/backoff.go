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

package util

import (
	"math"
	"math/rand"
	"time"
)

const (
	defaultInitial         = 200 * time.Millisecond
	defaultMax             = 10 * time.Second
	defaultFactor  float64 = 2
	defaultJitter          = false
)

// Backoff defines functions for RPC Backoff strategy.
type Backoff interface {
	Duration() time.Duration
	Attempt() int
	Reset()
	Clone() Backoff
}

// ExponentialBackoff is a backoff strategy that backs off exponentially.
type ExponentialBackoff struct {
	attempt         int
	initial, max    time.Duration
	jitter          bool
	backoffStrategy func() time.Duration
	factor          float64
}

// DefaultExponentialBackoff constructs a new ExponentialBackoff with defaults.
func DefaultExponentialBackoff() Backoff {
	return NewExponentialBackoff(0, 0, 0, defaultJitter)
}

// NewExponentialBackoff constructs a new ExponentialBackoff.
func NewExponentialBackoff(initial, max time.Duration, factor float64, jitter bool) Backoff {
	backoff := &ExponentialBackoff{}

	if initial <= 0 {
		initial = defaultInitial
	}
	if max <= 0 {
		max = defaultMax
	}

	if factor <= 0 {
		factor = defaultFactor
	}

	backoff.initial = initial
	backoff.max = max
	backoff.attempt = 0
	backoff.factor = factor
	backoff.jitter = jitter
	backoff.backoffStrategy = backoff.exponentialBackoffStrategy

	return backoff
}

// Duration calculates how long should be waited before attempting again. Note
// that this method is stateful - each call counts as an "attempt".
func (b *ExponentialBackoff) Duration() time.Duration {
	d := b.backoffStrategy()
	b.attempt++
	return d
}

func (b *ExponentialBackoff) exponentialBackoffStrategy() time.Duration {

	initial := float64(b.initial)
	attempt := float64(b.attempt)
	duration := initial * math.Pow(b.factor, attempt)

	if b.jitter {
		duration = rand.Float64()*(duration-initial) + initial
	}

	if duration > math.MaxInt64 {
		return b.max
	}

	dur := time.Duration(duration)
	if dur > b.max {
		return b.max
	}

	return dur
}

// Reset clears any state that the backoff strategy has.
func (b *ExponentialBackoff) Reset() {
	b.attempt = 0
}

// Attempt returns how many attempts have been made.
func (b *ExponentialBackoff) Attempt() int {
	return b.attempt
}

// Clone returns a copy
func (b *ExponentialBackoff) Clone() Backoff {
	return &ExponentialBackoff{
		attempt:         b.attempt,
		initial:         b.initial,
		max:             b.max,
		jitter:          b.jitter,
		backoffStrategy: b.backoffStrategy,
		factor:          b.factor,
	}
}
