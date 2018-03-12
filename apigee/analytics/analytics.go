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
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"istio.io/istio/mixer/pkg/adapter"
)

// A Manager is how we interact with some Apigee analytics backend.
type Manager interface {
	Start(env adapter.Env)
	Close()
	SendRecords(auth *auth.Context, records []Record) error
}

// NewManager constructs a new analytics Manager. Call Close when you are done.
func NewManager(env adapter.Env) Manager {
	// TODO(robbrit): Allow setting the backend based on a flag or config setting.
	m := &apigeeBackend{}
	m.Start(env)
	return m
}

// TimeToUnix converts a time to a UNIX timestamp in milliseconds.
func TimeToUnix(t time.Time) int64 {
	return t.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}
