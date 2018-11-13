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
	"errors"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"github.com/hashicorp/go-multierror"
)

// A Record is a single event that is tracked via Apigee analytics.
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
	Organization                 string `json:"organization"`
	Environment                  string `json:"environment"`
	GatewaySource                string `json:"gateway_source"`
}

func (r Record) ensureFields(ctx *auth.Context) Record {
	r.RecordType = axRecordType

	// populate from auth context
	r.DeveloperEmail = ctx.DeveloperEmail
	r.DeveloperApp = ctx.Application
	r.AccessToken = ctx.AccessToken
	r.ClientID = ctx.ClientID
	r.Organization = ctx.Organization()
	r.Environment = ctx.Environment()

	// todo: select best APIProduct based on path, otherwise arbitrary
	if len(ctx.APIProducts) > 0 {
		r.APIProduct = ctx.APIProducts[0]
	}
	return r
}

// validate confirms that a record has correct values in it.
func (r Record) validate(now time.Time) error {
	var err error

	// Validate that certain fields are set.
	if r.Organization == "" {
		err = multierror.Append(err, errors.New("missing Organization"))
	}
	if r.Environment == "" {
		err = multierror.Append(err, errors.New("missing Environment"))
	}
	if r.ClientReceivedStartTimestamp == 0 {
		err = multierror.Append(err, errors.New("missing ClientReceivedStartTimestamp"))
	}
	if r.ClientReceivedEndTimestamp == 0 {
		err = multierror.Append(err, errors.New("missing ClientReceivedEndTimestamp"))
	}
	if r.ClientReceivedStartTimestamp > r.ClientReceivedEndTimestamp {
		err = multierror.Append(err, errors.New("ClientReceivedStartTimestamp > ClientReceivedEndTimestamp"))
	}

	// Validate that timestamps make sense.
	ts := time.Unix(r.ClientReceivedStartTimestamp/1000, 0)
	if ts.After(now) {
		err = multierror.Append(err, errors.New("ClientReceivedStartTimestamp cannot be in the future"))
	}
	if ts.Before(now.Add(-90 * 24 * time.Hour)) {
		err = multierror.Append(err, errors.New("ClientReceivedStartTimestamp cannot be more than 90 days old"))
	}
	return err
}
