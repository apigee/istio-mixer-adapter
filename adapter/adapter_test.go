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

package adapter

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/analytics"
	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"github.com/apigee/istio-mixer-adapter/adapter/config"
	analyticsT "github.com/apigee/istio-mixer-adapter/template/analytics"
	pbtypes "github.com/gogo/protobuf/types"
	"istio.io/istio/mixer/pkg/adapter/test"
	"istio.io/istio/mixer/template/authorization"
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

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %s", err)
	}
	defer os.RemoveAll(d)

	b := GetInfo().NewBuilder().(*builder)

	// missing config items
	b.SetAdapterConfig(&config.Params{})

	if errs := b.Validate(); errs == nil {
		t.Errorf("Validate() missing config should have errors")
	} else {
		want := `6 errors occurred:

* apigee_base: required
* customer_base: required
* org_name: required
* env_name: required
* key: required
* secret: required`
		if errs.String() != want {
			t.Errorf("Validate() want: \n%s.\nGot: \n%s", want, errs)
		}
	}

	// bad config items
	b.SetAdapterConfig(&config.Params{
		ApigeeBase:   "not an url",
		CustomerBase: "not an url",
		OrgName:      "org",
		EnvName:      "env",
		Key:          "key",
		Secret:       "secret",
	})
	if errs := b.Validate(); errs == nil {
		t.Errorf("Validate() bad config should have errors")
	} else {
		want := `2 errors occurred:

* apigee_base: must be a valid url: parse not an url: invalid URI for request
* customer_base: must be a valid url: parse not an url: invalid URI for request`
		if errs.String() != want {
			t.Errorf("Validate() want: \n%s.\nGot: \n%s", want, errs)
		}
	}

	// good config
	b.SetAdapterConfig(GetInfo().DefaultConfig)
	validConfig := config.Params{
		ApigeeBase:    serverURL.String(),
		CustomerBase:  serverURL.String(),
		OrgName:       "org",
		EnvName:       "env",
		Key:           "key",
		Secret:        "secret",
		TempDir:       d,
		ClientTimeout: pbtypes.DurationProto(30 * time.Second),
		Analytics: &config.ParamsAnalyticsOptions{
			FileLimit: 10,
		},
		Products: &config.ParamsProductOptions{
			RefreshRate: pbtypes.DurationProto(2 * time.Minute),
		},
	}
	b.SetAdapterConfig(&validConfig)

	if err := b.Validate(); err != nil {
		t.Fatalf("Validate() resulted in unexpected error: %v", err)
	}

	// invoke the empty set methods for coverage
	b.SetAnalyticsTypes(map[string]*analyticsT.Type{})
	b.SetAuthorizationTypes(map[string]*authorization.Type{})

	// check build
	h, err := b.Build(context.Background(), test.NewEnv(t))
	if err != nil {
		t.Fatalf("Build() resulted in unexpected error: %v", err)
	}

	ah := h.(*handler)
	derivedConfig := config.Params{
		ApigeeBase:    ah.ApigeeBase().String(),
		CustomerBase:  ah.CustomerBase().String(),
		OrgName:       ah.Organization(),
		EnvName:       ah.Environment(),
		Key:           ah.Key(),
		Secret:        ah.Secret(),
		TempDir:       d,
		ClientTimeout: validConfig.ClientTimeout,
		Analytics: &config.ParamsAnalyticsOptions{
			FileLimit: 10,
		},
		Products: &config.ParamsProductOptions{
			RefreshRate: pbtypes.DurationProto(2 * time.Minute),
		},
	}
	if !reflect.DeepEqual(validConfig, derivedConfig) {
		t.Errorf("bad derived config. want: %#v. got: %#v", validConfig, derivedConfig)
	}

	defer h.Close()
	if err != nil {
		t.Errorf("Build() resulted in unexpected error: %v", err)
	}
}

func TestHandleAnalytics(t *testing.T) {

	basePath := "/some/path"
	queryString := "with=query"
	pathWithQueryString := basePath + "?" + queryString
	env := test.NewEnv(t)
	ctx := context.Background()
	var baseURL *url.URL

	uploaded := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if strings.HasPrefix(r.URL.Path, "/analytics/") {
			u := *baseURL
			u.Path = "/upload"
			w.Write([]byte(fmt.Sprintf(`{ "url": "%s" }`, u.String())))
			return
		}

		if strings.HasPrefix(r.URL.Path, "/upload") {
			uploaded = true

			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				t.Fatalf("bad gzip: %v", err)
			}
			var recs []analytics.Record
			if err := json.NewDecoder(gz).Decode(&recs); err != nil {
				t.Fatalf("bad JSON: %v", err)
			}

			if len(recs) != 1 {
				t.Errorf("Should have received 1 rec, got %v", recs)
			}

			rec := recs[0]
			if rec.RequestPath != basePath {
				t.Errorf("RequestPath expected %s, got %s", rec.RequestPath, basePath)
			}

			if rec.RequestURI != pathWithQueryString {
				t.Errorf("RequestURI expected %s, got %s", rec.RequestURI, pathWithQueryString)
			}

			w.WriteHeader(200)
			return
		}

		t.Fatalf("invalid URL called: %s", r.URL.String())
	}))
	defer ts.Close()
	var err error
	baseURL, err = url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %s", err)
	}
	defer os.RemoveAll(d)

	analyticsMan, err := analytics.NewManager(env, analytics.Options{
		BufferPath: d,
		BufferSize: 10,
		BaseURL:    *baseURL,
		Key:        "key",
		Secret:     "secret",
		Client:     http.DefaultClient,
	})
	if err != nil {
		t.Fatalf("analytics.NewManager: %s", err)
	}

	h := &handler{
		env:          env,
		apigeeBase:   baseURL,
		customerBase: baseURL,
		orgName:      "org",
		envName:      "env",
		analyticsMan: analyticsMan,
	}

	inst := []*analyticsT.Instance{
		{
			Name: "name",
			ClientReceivedStartTimestamp: time.Now(),
			ClientReceivedEndTimestamp:   time.Now(),
			RequestUri:                   pathWithQueryString,
			RequestPath:                  pathWithQueryString,
		},
	}

	err = h.HandleAnalytics(ctx, inst)
	if err != nil {
		t.Errorf("HandleAnalytics(ctx, nil) resulted in an unexpected error: %v", err)
	}

	if err := h.Close(); err != nil {
		t.Errorf("Close() returned an unexpected error")
	}

	if !uploaded {
		t.Errorf("analytics not delivered")
	}
}

func TestResolveClaims(t *testing.T) {
	input := map[string]string{}
	for i, c := range auth.AllValidClaims {
		input[c] = strconv.Itoa(i)
	}
	input["extra"] = "extra"

	want := map[string]string{}
	for i, c := range auth.AllValidClaims {
		want[c] = strconv.Itoa(i)
	}

	jsonBytes, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}

	h := handler{
		env: test.NewEnv(t),
	}

	for _, ea := range []struct {
		desc   string
		claims map[string]string
	}{
		{"map of strings", want},
		{"encoded value", map[string]string{
			jsonClaimsKey: string(jsonBytes),
		}},
	} {
		t.Log(ea.desc)

		claimsOut := h.resolveClaims(ea.claims)

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

func TestToDuration(t *testing.T) {
	expected := time.Second + time.Nanosecond
	protoDuration := pbtypes.DurationProto(expected)
	convertedDuration := toDuration(protoDuration)

	if convertedDuration != expected {
		t.Errorf(`expected time.Duration %v, but got %v`, expected, convertedDuration)
	}
}
