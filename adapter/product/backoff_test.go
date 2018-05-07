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
