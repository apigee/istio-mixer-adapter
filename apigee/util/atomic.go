// Copyright 2020 Google LLC
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

import "sync/atomic"

// AtomicBool is a threadsafe bool
type AtomicBool struct {
	boolInt *int32
}

// NewAtomicBool creates an AtomicBool
func NewAtomicBool(flag bool) *AtomicBool {
	boolInt := int32(0)
	if flag {
		boolInt = int32(1)
	}
	return &AtomicBool{
		boolInt: &boolInt,
	}
}

// IsTrue returns true if true
func (a *AtomicBool) IsTrue() bool {
	return atomic.LoadInt32(a.boolInt) == int32(1)
}

// IsFalse returns false if false
func (a *AtomicBool) IsFalse() bool {
	return atomic.LoadInt32(a.boolInt) != int32(1)
}

// SetTrue sets the bool to true, returns true if unchanged
func (a *AtomicBool) SetTrue() bool {
	return atomic.SwapInt32(a.boolInt, 1) == int32(1)
}

// SetFalse sets the bool to false, returns true if unchanged
func (a *AtomicBool) SetFalse() bool {
	return atomic.SwapInt32(a.boolInt, 0) == int32(0)
}
