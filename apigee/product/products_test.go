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

package product

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/pkg/adapter/test"
)

func TestStartStop(t *testing.T) {

	apiProducts := []APIProduct{
		{
			Attributes: []Attribute{
				{Name: servicesAttr, Value: "service1.istio"},
			},
			Name:      "Name 1",
			Resources: []string{"/"},
			Scopes:    []string{"scope1"},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result = apiResponse{
			APIProducts: apiProducts,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer ts.Close()

	env := test.NewEnv(t)
	serverURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	p := NewManager(*serverURL, env, env)
	defer p.Close()
	context := &testContext{
		log: env,
	}
	ac := auth.Context{
		Context:     context,
		APIProducts: []string{apiProducts[0].Name},
		Scopes:      apiProducts[0].Scopes,
	}
	products := p.Resolve(ac, apiProducts[0].Attributes[0].Value, "/")
	if len(products) != 1 {
		t.Errorf("want: 1, got: %d", len(products))
	}
}

func TestResolve(t *testing.T) {

	productsMap := map[string]APIProduct{
		"Name 1": {
			Attributes: []Attribute{
				{Name: servicesAttr, Value: "service1.istio, shared.istio"},
			},
			Name:      "Name 1",
			Resources: []string{"/"},
			Scopes:    []string{"scope1"},
			Targets:   []string{"service1.istio", "shared.istio"},
		},
		"Name 2": {
			Attributes: []Attribute{
				{Name: servicesAttr, Value: "service2.istio,shared.istio"},
			},
			Environments: []string{"prod"},
			Name:         "Name 2",
			Resources:    []string{"/**"},
			Scopes:       []string{"scope2"},
			Targets:      []string{"service2.istio", "shared.istio"},
		},
	}

	products := []string{"Name 1", "Name 2"}
	scopes := []string{"scope1", "scope2"}
	api := "shared.istio"
	path := "/"

	resolved := resolve(productsMap, products, scopes, api, path)
	if len(resolved) != 2 {
		t.Errorf("want: 2, got: %d", len(resolved))
	}

	scopes = []string{"scope2"}
	resolved = resolve(productsMap, products, scopes, api, path)
	if len(resolved) != 1 {
		t.Errorf("want: 1, got: %d", len(resolved))
	} else {
		got := resolved[0]
		want := productsMap["Name 2"]
		if !reflect.DeepEqual(want, got) {
			t.Errorf("\nwant: %v\n got: %v", want, got)
		}
	}

	products = []string{"Name 1"}
	resolved = resolve(productsMap, products, scopes, api, path)
	if len(resolved) != 0 {
		t.Errorf("want: 0, got: %d", len(resolved))
	}
}

// https://docs.apigee.com/developer-services/content/create-api-products#resourcebehavior
func TestValidPath(t *testing.T) {

	resources := []string{"/", "/v1/*", "/v1/**", "/v1/weatherapikey/*/2/**"}
	specs := []struct {
		Path    string
		Results []bool
	}{
		{"/v1/weatherapikey", []bool{true, true, true, false}},
		{"/v1/weatherapikey/", []bool{true, false, true, false}},
		{"/v1/weatherapikey/1", []bool{true, false, true, false}},
		{"/v1/weatherapikey/1/", []bool{true, false, true, false}},
		{"/v1/weatherapikey/1/2", []bool{true, false, true, false}},
		{"/v1/weatherapikey/1/2/", []bool{true, false, true, true}},
		{"/v1/weatherapikey/1/2/3/", []bool{true, false, true, true}},
		{"/v1/weatherapikey/1/a/2/3/", []bool{true, false, true, false}},
	}

	for _, spec := range specs {
		for j, resource := range resources {
			p := APIProduct{
				Resources: []string{resource},
			}
			if p.isValidPath(spec.Path) != spec.Results[j] {
				t.Errorf("expected: %v got: %v for path: %s, resource: %s",
					spec.Results[j], p.isValidPath(spec.Path), spec.Path, resource)
			}
		}
	}
}

func TestValidScopes(t *testing.T) {
	p := APIProduct{
		Scopes: []string{"scope1"},
	}
	if !p.isValidScopes([]string{"scope1"}) {
		t.Errorf("expected %s is valid", p.Scopes)
	}
	if !p.isValidScopes([]string{"scope1", "scope2"}) {
		t.Errorf("expected %s is valid", p.Scopes)
	}
	if p.isValidScopes([]string{"scope2"}) {
		t.Errorf("expected %s is not valid", p.Scopes)
	}
	p.Scopes = []string{"scope1", "scope2"}
	if !p.isValidScopes([]string{"scope1"}) {
		t.Errorf("expected %s is valid", p.Scopes)
	}
	if !p.isValidScopes([]string{"scope1", "scope2"}) {
		t.Errorf("expected %s is valid", p.Scopes)
	}
	if !p.isValidScopes([]string{"scope2"}) {
		t.Errorf("expected %s is valid", p.Scopes)
	}
}

func TestBadResource(t *testing.T) {
	if _, e := makeResourceRegex("/**/bad"); e == nil {
		t.Errorf("expected error for resource: %s", "/**/bad")
	}
}

type testContext struct {
	apigeeBase   url.URL
	customerBase url.URL
	orgName      string
	envName      string
	key          string
	secret       string
	log          adapter.Logger
}

func (h *testContext) Log() adapter.Logger {
	return h.log
}
func (h *testContext) ApigeeBase() url.URL {
	return h.apigeeBase
}
func (h *testContext) CustomerBase() url.URL {
	return h.customerBase
}
func (h *testContext) Organization() string {
	return h.orgName
}
func (h *testContext) Environment() string {
	return h.envName
}
func (h *testContext) Key() string {
	return h.key
}
func (h *testContext) Secret() string {
	return h.secret
}
