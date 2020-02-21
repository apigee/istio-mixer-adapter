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
	"testing"
	"time"
)

func TestExponentialBackoff(t *testing.T) {
	initial := 200 * time.Millisecond
	max := 2 * time.Second
	factor := float64(2)
	jitter := false
	b := NewExponentialBackoff(initial, max, factor, jitter)

	for i := 0; i < 2; i++ {
		durations := []time.Duration{
			200 * time.Millisecond,
			400 * time.Millisecond,
			800 * time.Millisecond,
			1600 * time.Millisecond,
			2000 * time.Millisecond,
		}

		for i, want := range durations {
			got := b.Duration()
			if want != got {
				t.Errorf("duration want: %d, got: %d", want, got)
			}
			if i+1 != b.Attempt() {
				t.Errorf("attempt want: %d, got: %d", i+1, b.Attempt())
			}
		}

		b.Reset()
	}
}

func TestBackoffWithJitter(t *testing.T) {
	initial := 200 * time.Millisecond
	max := 2 * time.Second
	factor := float64(2)
	jitter := true
	b := NewExponentialBackoff(initial, max, factor, jitter)

	durations := []time.Duration{
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1600 * time.Millisecond,
		2000 * time.Millisecond,
	}

	for i, want := range durations {
		got := b.Duration()
		if got < initial || got > want {
			t.Errorf("duration out of bounds. got: %v, iter: %d", got, i)
		}
	}

	b.Reset()
}

func TestDefaultBackoff(t *testing.T) {
	backoff := DefaultExponentialBackoff()
	eb, ok := backoff.(*ExponentialBackoff)
	if !ok {
		t.Errorf("not an *ExponentialBackoff")
	}
	if eb.initial != defaultInitial {
		t.Errorf("want: %v, got: %v", defaultInitial, eb.initial)
	}
	if defaultInitial != eb.initial {
		t.Errorf("want: %v, got: %v", defaultInitial, eb.initial)
	}
	if defaultMax != eb.max {
		t.Errorf("want: %v, got: %v", defaultMax, eb.max)
	}
	if defaultFactor != eb.factor {
		t.Errorf("want: %v, got: %v", defaultFactor, eb.factor)
	}
	if defaultJitter != eb.jitter {
		t.Errorf("want: %v, got: %v", defaultJitter, eb.jitter)
	}
}

func TestCloneExponentialBackoff(t *testing.T) {
	backoff1 := DefaultExponentialBackoff()
	backoff2 := backoff1.Clone()

	if &backoff1 == &backoff2 {
		t.Errorf("must not be the same object!")
	}

	if backoff1.Attempt() != 0 {
		t.Errorf("want 0, got %d", backoff1.Attempt())
	}
	backoff1.Duration()
	if backoff1.Attempt() != 1 {
		t.Errorf("want 1, got %d", backoff1.Attempt())
	}

	if backoff2.Attempt() != 0 {
		t.Errorf("want 0, got %d", backoff2.Attempt())
	}

	backoff3 := backoff1.Clone()
	if backoff3.Attempt() != 1 {
		t.Errorf("want 1, got %d", backoff3.Attempt())
	}

	if backoff2.Duration() != backoff3.Duration() {
		t.Errorf("durations not equal")
	}

	if backoff1.Duration() == backoff3.Duration() {
		t.Errorf("durations should not be equal")
	}
}
