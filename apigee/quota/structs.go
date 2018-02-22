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

package quota

type QuotaRequest struct {
	Identifier string `json:"identifier"`
	Weight     int64  `json:"weight"`
	Interval   int64  `json:"interval"`
	Allow      int64  `json:"allow"`
	TimeUnit   string `json:"timeUnit"`
}

type QuotaResult struct {
	Allowed    int64 `json:"allowed"`
	Used       int64 `json:"used"`
	Exceeded   int64 `json:"exceeded"`
	ExpiryTime int64 `json:"expiryTime"`
	Timestamp  int64 `json:"timestamp"`
}
