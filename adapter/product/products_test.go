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
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"istio.io/istio/mixer/pkg/adapter/test"
)

func TestValidate(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer ts.Close()

	env := test.NewEnv(t)
	serverURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	opts := Options{
		BaseURL:     serverURL,
		RefreshRate: time.Hour,
		Client:      http.DefaultClient,
	}
	p, err := NewManager(env, opts)
	if err == nil {
		t.Fatal("should be invalid options")
	}

	opts = Options{
		BaseURL:     serverURL,
		RefreshRate: time.Second,
		Client:      http.DefaultClient,
		Key:         "x",
		Secret:      "x",
	}
	p, err = NewManager(env, opts)
	if err == nil {
		t.Fatal("should be invalid options")
	}

	opts = Options{
		BaseURL:     serverURL,
		RefreshRate: time.Hour,
		Client:      http.DefaultClient,
		Key:         "x",
		Secret:      "x",
	}
	p, err = NewManager(env, opts)
	if err != nil {
		t.Fatalf("invalid error: %v", err)
	}
	defer p.Close()
}

func TestResolve(t *testing.T) {

	productsMap := map[string]*APIProduct{
		"Name 1": {
			Attributes: []Attribute{
				{Name: ServicesAttr, Value: "service1.istio, shared.istio"},
			},
			Name:      "Name 1",
			Resources: []string{"/"},
			Scopes:    []string{"scope1"},
			Targets:   []string{"service1.istio", "shared.istio"},
		},
		"Name 2": {
			Attributes: []Attribute{
				{Name: ServicesAttr, Value: "service2.istio,shared.istio"},
			},
			Environments: []string{"prod"},
			Name:         "Name 2",
			Resources:    []string{"/**"},
			Scopes:       []string{"scope2"},
			Targets:      []string{"service2.istio", "shared.istio"},
		},
		"Name 3": {
			Attributes: []Attribute{
				{Name: ServicesAttr, Value: "shared.istio"},
			},
			Environments: []string{"prod"},
			Name:         "Name 3",
			Resources:    []string{"/"},
			Scopes:       []string{},
			Targets:      []string{"shared.istio"},
		},
	}

	pMan := createManager(Options{}, nil)
	for _, p := range productsMap {
		pMan.resolveResourceMatchers(p)
	}

	api := "shared.istio"
	path := "/"

	ac := &auth.Context{
		APIProducts: []string{"Name 1", "Name 2", "Name 3", "Invalid"},
		Scopes:      []string{"scope1", "scope2"},
	}
	resolved, failHints := resolve(ac, productsMap, api, path)
	if len(resolved) != 3 {
		t.Errorf("want: 3, got: %v", failHints)
	}
	if len(failHints) != 1 {
		t.Errorf("want: 1, got: %v", failHints)
	}

	ac.Scopes = []string{"scope2"}
	resolved, failHints = resolve(ac, productsMap, api, path)
	if len(resolved) != 2 {
		t.Errorf("want: 2, got: %d", len(resolved))
	} else {
		got := resolved[0]
		want := productsMap["Name 2"]
		if !reflect.DeepEqual(want, got) {
			t.Errorf("\nwant: %v\n got: %v", want, got)
		}
	}
	if len(failHints) != 2 {
		t.Errorf("want: 2, got: %v", failHints)
	}

	ac.APIProducts = []string{"Name 1"}
	resolved, failHints = resolve(ac, productsMap, api, path)
	if len(resolved) != 0 {
		t.Errorf("want: 0, got: %d", len(resolved))
	}
	if len(failHints) != 1 {
		t.Errorf("want: 1, got: %v", failHints)
	}

	// check API Key - no scopes required!
	ac.APIKey = "x"
	ac.APIProducts = []string{"Name 1", "Name 2", "Name 3"}
	ac.Scopes = []string{}
	resolved, failHints = resolve(ac, productsMap, api, path)
	if len(resolved) != 3 {
		t.Errorf("want: 3, got: %d", len(resolved))
	}
	if len(failHints) != 0 {
		t.Errorf("want: 0, got: %v", failHints)
	}
}

// Path matching is similar to wildcard semantics described in the Apigee product documentation here:
// https://docs.apigee.com/developer-services/content/create-api-products#resourcebehavior.
// However, as there is no base path, it is simplified as follows:
// 1. A single slash (/) by itself matches any path.
// 2. * is valid anywhere and matches within a segment (between slashes).
// 3. ** is valid at the end and matches anything to the end of line.
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

	pMan := createManager(Options{}, nil)

	for _, spec := range specs {
		for j, resource := range resources {
			p := &APIProduct{
				Resources: []string{resource},
			}
			pMan.resolveResourceMatchers(p)
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
