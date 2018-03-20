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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/pkg/adapter/test"
)

func TestAnalyticsSubmit(t *testing.T) {

	startTime := time.Now()
	context := &TestContext{
		orgName: "org",
		envName: "env",
		log:     test.NewEnv(t),
	}
	authContext := &auth.Context{
		Context:        context,
		DeveloperEmail: "email",
		Application:    "app",
		AccessToken:    "token",
		ClientID:       "clientId",
	}
	axRecord := Record{
		ResponseStatusCode:           201,
		RequestVerb:                  "PATCH",
		RequestPath:                  "/test",
		UserAgent:                    "007",
		ClientReceivedStartTimestamp: TimeToUnix(startTime),
		ClientReceivedEndTimestamp:   TimeToUnix(startTime),
		ClientSentStartTimestamp:     TimeToUnix(startTime),
		ClientSentEndTimestamp:       TimeToUnix(startTime),
		TargetSentStartTimestamp:     TimeToUnix(startTime),
		TargetSentEndTimestamp:       TimeToUnix(startTime),
		TargetReceivedStartTimestamp: TimeToUnix(startTime),
		TargetReceivedEndTimestamp:   TimeToUnix(startTime),
	}
	ts := makeTestServer(authContext, axRecord, t)
	defer ts.Close()
	baseURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	context.apigeeBase = *baseURL
	context.customerBase = *baseURL
	ab := &apigeeBackend{}
	err = ab.SendRecords(authContext, []Record{axRecord})
	if err != nil {
		t.Error(err)
	}
	// todo: check record sent
}

func TestBadServerBase(t *testing.T) {

	context := &TestContext{
		orgName:      "org",
		envName:      "env",
		apigeeBase:   url.URL{},
		customerBase: url.URL{},
		log:          test.NewEnv(t),
	}
	authContext := &auth.Context{
		Context:        context,
		DeveloperEmail: "email",
		Application:    "app",
		AccessToken:    "token",
		ClientID:       "clientId",
	}
	axRecord := Record{}
	ts := makeTestServer(authContext, axRecord, t)
	defer ts.Close()
	ab := &apigeeBackend{}
	err := ab.SendRecords(authContext, []Record{axRecord})
	if err == nil {
		t.Errorf("should get bad base error")
	}
}

func TestMissingOrg(t *testing.T) {

	context := &TestContext{
		orgName: "",
		envName: "env",
		log:     test.NewEnv(t),
	}
	authContext := &auth.Context{
		Context:        context,
		DeveloperEmail: "email",
		Application:    "app",
		AccessToken:    "token",
		ClientID:       "clientId",
	}
	axRecord := Record{}
	ts := makeTestServer(authContext, axRecord, t)
	defer ts.Close()
	baseURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	context.apigeeBase = *baseURL
	context.customerBase = *baseURL
	ab := &apigeeBackend{}
	err = ab.SendRecords(authContext, []Record{axRecord})
	if err == nil || !strings.Contains(err.Error(), "organization") {
		t.Errorf("should get missing organization error, got: %s", err)
	}
}

func TestMissingEnv(t *testing.T) {

	context := &TestContext{
		orgName: "org",
		envName: "",
		log:     test.NewEnv(t),
	}
	authContext := &auth.Context{
		Context:        context,
		DeveloperEmail: "email",
		Application:    "app",
		AccessToken:    "token",
		ClientID:       "clientId",
	}
	axRecord := Record{}
	ts := makeTestServer(authContext, axRecord, t)
	defer ts.Close()
	baseURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	context.apigeeBase = *baseURL
	context.customerBase = *baseURL
	ab := &apigeeBackend{}
	err = ab.SendRecords(authContext, []Record{axRecord})
	if err == nil || !strings.Contains(err.Error(), "environment") {
		t.Errorf("should get missing environment error, got: %s", err)
	}
}

func makeTestServer(auth *auth.Context, rec Record, t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var axRequest request
		err := decoder.Decode(&axRequest)
		if err != nil {
			t.Error(err)
		}
		defer r.Body.Close()

		auth.Context.Log().Errorf("axRequest: %#v", axRequest)

		if axRequest.Organization != auth.Organization() {
			t.Errorf("bad orgName")
		}
		if axRequest.Environment != auth.Environment() {
			t.Errorf("bad envName")
		}
		if len(axRequest.Records) != 1 {
			t.Errorf("record missing")
			return
		}

		axRecord := axRequest.Records[0]
		if axRecord.AccessToken == "" {
			t.Errorf("access_token missing")
		}
		if axRecord.ClientReceivedStartTimestamp != rec.ClientReceivedStartTimestamp {
			t.Errorf("client_received_start_timestamp expected: %v, got: %v",
				rec.ClientReceivedStartTimestamp, axRecord.ClientReceivedStartTimestamp)
		}

		w.WriteHeader(200)
	}))

}

type TestContext struct {
	apigeeBase   url.URL
	customerBase url.URL
	orgName      string
	envName      string
	key          string
	secret       string
	log          adapter.Logger
}

func (h *TestContext) Log() adapter.Logger {
	return h.log
}
func (h *TestContext) ApigeeBase() url.URL {
	return h.apigeeBase
}
func (h *TestContext) CustomerBase() url.URL {
	return h.customerBase
}
func (h *TestContext) Organization() string {
	return h.orgName
}
func (h *TestContext) Environment() string {
	return h.envName
}
func (h *TestContext) Key() string {
	return h.key
}
func (h *TestContext) Secret() string {
	return h.secret
}
