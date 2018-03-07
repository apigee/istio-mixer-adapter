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
)

const (
	axPath       = "/axpublisher/organization/%s/environment/%s"
	axRecordType = "APIAnalytics"
)

type AnalyticsProvider interface {
	SendRecords(auth *auth.Context, records []Record) error
}

// TODO(robbrit): Allow setting the backend based on a flag or config setting.
var provider = &apigeeBackend{}

func SendRecords(auth *auth.Context, records []Record) error {
	return provider.SendRecords(auth, records)
}
