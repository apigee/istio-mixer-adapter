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

package product

import (
	"math"
	"math/rand"
	"time"
)

const defaultInitial = 200 * time.Millisecond
const defaultMax = 10 * time.Second
const defaultFactor float64 = 2

type Backoff struct {
	attempt         int
	initial, max    time.Duration
	jitter          bool
	backoffStrategy func() time.Duration
}

type ExponentialBackoff struct {
	Backoff
	factor float64
}

func NewExponentialBackoff(initial, max time.Duration, factor float64, jitter bool) *ExponentialBackoff {
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

func (b *Backoff) Duration() time.Duration {
	d := b.backoffStrategy()
	b.attempt++
	return d
}

func (b *ExponentialBackoff) exponentialBackoffStrategy() time.Duration {

	initial := float64(b.Backoff.initial)
	attempt := float64(b.Backoff.attempt)
	duration := initial * math.Pow(b.factor, attempt)

	if duration > math.MaxInt64 {
		return b.max
	}
	dur := time.Duration(duration)

	if b.jitter {
		duration = rand.Float64()*(duration-initial) + initial
	}

	if dur > b.max {
		return b.max
	}

	return dur
}

func (b *Backoff) Reset() {
	b.attempt = 0
}

func (b *Backoff) Attempt() int {
	return b.attempt
}
