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

type Record struct {
	ClientReceivedStartTimestamp int64  `json:"client_received_start_timestamp"`
	ClientReceivedEndTimestamp   int64  `json:"client_received_end_timestamp"`
	ClientSentStartTimestamp     int64  `json:"client_sent_start_timestamp"`
	ClientSentEndTimestamp       int64  `json:"client_sent_end_timestamp"`
	TargetReceivedStartTimestamp int64  `json:"target_received_start_timestamp,omitempty"`
	TargetReceivedEndTimestamp   int64  `json:"target_received_end_timestamp,omitempty"`
	TargetSentStartTimestamp     int64  `json:"target_sent_start_timestamp,omitempty"`
	TargetSentEndTimestamp       int64  `json:"target_sent_end_timestamp,omitempty"`
	RecordType                   string `json:"recordType"`
	APIProxy                     string `json:"apiproxy"`
	RequestURI                   string `json:"request_uri"`
	RequestPath                  string `json:"request_path"`
	RequestVerb                  string `json:"request_verb"`
	ClientIP                     string `json:"client_ip,omitempty"`
	UserAgent                    string `json:"useragent"`
	APIProxyRevision             int    `json:"apiproxy_revision"`
	ResponseStatusCode           int    `json:"response_status_code"`
	DeveloperEmail               string `json:"developer_email,omitempty"`
	DeveloperApp                 string `json:"developer_app,omitempty"`
	AccessToken                  string `json:"access_token,omitempty"`
	ClientID                     string `json:"client_id,omitempty"`
	APIProduct                   string `json:"api_product,omitempty"`
}

type Request struct {
	Organization string   `json:"organization"`
	Environment  string   `json:"environment"`
	Records      []Record `json:"records"`
}

type Response struct {
	Accepted int `json:"accepted"`
	Rejected int `json:"rejected"`
}
