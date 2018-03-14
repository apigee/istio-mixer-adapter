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

package apigee

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"strconv"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/config"
	"github.com/apigee/istio-mixer-adapter/template/analytics"
	rpc "istio.io/gogo-genproto/googleapis/google/rpc"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/pkg/adapter/test"
	"istio.io/istio/mixer/pkg/status"
	"istio.io/istio/mixer/template/authorization"
	"istio.io/istio/mixer/template/quota"
)

func TestValidateBuild(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer ts.Close()

	serverURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	b := GetInfo().NewBuilder().(*builder)

	b.SetAdapterConfig(GetInfo().DefaultConfig)
	b.SetAdapterConfig(&config.Params{
		ApigeeBase:   serverURL.String(),
		CustomerBase: serverURL.String(),
		OrgName:      "org",
		EnvName:      "env",
		Key:          "key",
		Secret:       "secret",
	})

	if err := b.Validate(); err != nil {
		t.Errorf("Validate() resulted in unexpected error: %v", err)
	}

	// invoke the empty set methods for coverage
	b.SetAnalyticsTypes(map[string]*analytics.Type{})
	b.SetQuotaTypes(map[string]*quota.Type{})
	b.SetAuthorizationTypes(map[string]*authorization.Type{})

	// check build
	handler, err := b.Build(context.Background(), test.NewEnv(t))

	defer handler.Close()
	if err != nil {
		t.Errorf("Build() resulted in unexpected error: %v", err)
	}
}

func TestHandleAnalytics(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()
	baseURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	h := &handler{
		env:          test.NewEnv(t),
		apigeeBase:   *baseURL,
		customerBase: *baseURL,
		orgName:      "org",
		envName:      "env",
	}

	inst := []*analytics.Instance{
		{Name: "name"},
	}

	err = h.HandleAnalytics(ctx, inst)
	if err != nil {
		t.Errorf("HandleAnalytics(ctx, nil) resulted in an unexpected error: %v", err)
	}

	if err := h.Close(); err != nil {
		t.Errorf("Close() returned an unexpected error")
	}
}

func TestHandleAuthorization(t *testing.T) {
	ctx := context.Background()

	h := &handler{
		env: test.NewEnv(t),
	}

	inst := &authorization.Instance{
		Name:    "",
		Subject: &authorization.Subject{},
		Action:  &authorization.Action{},
	}

	got, err := h.HandleAuthorization(ctx, inst)
	if err != nil {
		t.Errorf("HandleAuthorization(ctx, nil) resulted in an unexpected error: %v", err)
	}

	if got.Status.Code != int32(rpc.PERMISSION_DENIED) {
		t.Errorf("HandleAuthorization(ctx, nil) => %#v, want %#v", got.Status, status.OK)
	}

	if err := h.Close(); err != nil {
		t.Errorf("Close() returned an unexpected error")
	}
}

func TestHandleQuota(t *testing.T) {
	ctx := context.Background()

	h := &handler{
		env: test.NewEnv(t),
	}

	inst := &quota.Instance{
		Name: "",
		Dimensions: map[string]interface{}{
			"": "",
		},
	}

	got, err := h.HandleQuota(ctx, inst, adapter.QuotaArgs{})
	if err != nil {
		t.Errorf("HandleQuota(ctx, nil) resulted in an unexpected error: %v", err)
	}
	if !status.IsOK(got.Status) {
		t.Errorf("HandleQuota(ctx, nil) => %#v, want %#v", got.Status, status.OK)
	}

	if err := h.Close(); err != nil {
		t.Errorf("Close() returned an unexpected error")
	}
}

func TestResolveClaims(t *testing.T) {
	env := test.NewEnv(t)

	input := map[string]string{}
	for i, c := range auth.AllValidClaims {
		input[c] = strconv.Itoa(i)
	}
	input["extra"] = "extra"

	want := map[string]string{}
	for i, c := range auth.AllValidClaims {
		input[c] = strconv.Itoa(i)
	}

	jsonBytes, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(jsonBytes)

	for _, ea := range []struct {
		desc   string
		claims map[string]string
	}{
		{"map of strings", want},
		{"encoded value", map[string]string{
			encodedClaimsKey: encoded,
		}},
	} {
		t.Log(ea.desc)

		claimsOut := resolveClaims(env, ea.claims)

		// normalize the type to same as want
		got := map[string]string{}
		for k, v := range claimsOut {
			got[k] = v.(string)
		}

		if !reflect.DeepEqual(want, got) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	}
}
