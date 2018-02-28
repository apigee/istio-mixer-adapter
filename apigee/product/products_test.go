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
	"reflect"
	"testing"
)

func TestResolve(t *testing.T) {

	productsMap := map[string]APIProduct{
		"Name 1": {
			Attributes: []Attribute{
				{Name: servicesAttr, Value: "service1.istio"},
			},
			Environments: []string{"test"},
			Name:         "Name 1",
			Resources:    []string{"/"},
			Scopes:       []string{"scope1"},
		},
		"Name 2": {
			Attributes: []Attribute{
				{Name: servicesAttr, Value: "service1.istio, service2.istio"},
			},
			Environments: []string{"prod"},
			Name:         "Name 2",
			Resources:    []string{"/**"},
			Scopes:       []string{"scope2"},
		},
	}

	products := []string{"Name 1", "Name 2"}
	scopes := []string{"scope1", "scope2"}
	api := "service1.istio"
	path := "/"

	resolved := resolve(productsMap, products, scopes, api, path)
	if len(resolved) != 2 {
		t.Errorf("want: 2, got: %d", len(resolved))
	}

	path = "/blah"
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
}
