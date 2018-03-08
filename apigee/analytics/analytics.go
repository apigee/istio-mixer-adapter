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
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"istio.io/istio/mixer/pkg/adapter"
)

type AnalyticsProvider interface {
	Start(env adapter.Env)
	Stop()
	SendRecords(auth *auth.Context, records []Record) error
}

// TODO(robbrit): Allow setting the backend based on a flag or config setting.
var provider = &apigeeBackend{}

// Start starts the main analytics provider.
func Start(env adapter.Env) {
	provider.Start(env)
}

// Stop stops the main analytics provider.
func Stop() {
	provider.Stop()
}

// SendRecords sends analytics records to the backend server.
func SendRecords(auth *auth.Context, records []Record) error {
	return provider.SendRecords(auth, records)
}
