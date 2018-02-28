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

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/product"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/pkg/adapter/test"
)

func TestQuota(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := QuotaResult{
			Allowed:    1,
			Used:       1,
			Exceeded:   1,
			ExpiryTime: time.Now().Unix(),
			Timestamp:  time.Now().Unix(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer ts.Close()

	serverURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	context := &TestContext{
		apigeeBase:   *serverURL,
		customerBase: *serverURL,
		log:          test.NewEnv(t),
	}
	authContext := &auth.Context{
		Context:        context,
		DeveloperEmail: "email",
		Application:    "app",
		AccessToken:    "token",
		ClientID:       "clientId",
	}

	p := product.APIProduct{
		QuotaLimit:    "1",
		QuotaInterval: 1,
		QuotaTimeUnit: "second",
	}

	args := adapter.QuotaArgs{
		DeduplicationID: "X",
		QuotaAmount:     1,
		BestEffort:      true,
	}

	result, err := Apply(*authContext, p, args)
	if err != nil {
		t.Errorf("error should be nil")
	}

	if result.Used != 1 {
		t.Errorf("result used should be 1")
	}
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
