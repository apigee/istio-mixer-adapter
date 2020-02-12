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
)

func TestAtomicBool(t *testing.T) {
	ab := NewAtomicBool(true)
	if !ab.IsTrue() {
		t.Error("should be true")
	}
	if ab.IsFalse() {
		t.Error("should not be false")
	}

	if ab.SetFalse() {
		t.Errorf("should have changed to false")
	}
	if !ab.SetFalse() {
		t.Errorf("should not have changed to false")
	}
	if ab.IsTrue() {
		t.Error("should not be true")
	}
	if !ab.IsFalse() {
		t.Error("should be false")
	}

	if ab.SetTrue() {
		t.Errorf("should have changed to true")
	}
	if !ab.SetTrue() {
		t.Errorf("should not have changed to true")
	}
	if !ab.IsTrue() {
		t.Error("should be true")
	}
	if ab.IsFalse() {
		t.Error("should not be false")
	}

}
