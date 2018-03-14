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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"istio.io/istio/mixer/pkg/adapter/test"
)

func TestManager(t *testing.T) {

	apiProducts := []APIProduct{
		{
			Attributes: []Attribute{
				{Name: servicesAttr, Value: "attr value"},
			},
			Description:    "product 1",
			DisplayName:    "APIProduct 1",
			Environments:   []string{"test"},
			LastModifiedAt: time.Now().Unix(),
			LastModifiedBy: "test@apigee.com",
			Name:           "Name 1",
			QuotaLimit:     "10",
			QuotaInterval:  "1",
			QuotaTimeUnit:  "minute",
			Resources:      []string{"/"},
			Scopes:         []string{"scope1"},
		},
		{
			Attributes: []Attribute{
				{Name: servicesAttr, Value: "attr value"},
			},
			CreatedAt:      time.Now().Unix(),
			CreatedBy:      "test2@apigee.com",
			Description:    "product 2",
			DisplayName:    "APIProduct 2",
			Environments:   []string{"prod"},
			LastModifiedAt: time.Now().Unix(),
			LastModifiedBy: "test@apigee.com",
			Name:           "Name 2",
			QuotaLimit:     "20",
			QuotaInterval:  "1",
			QuotaTimeUnit:  "hour",
			Resources:      []string{"/"},
			Scopes:         []string{"scope1"},
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

	pp := createManager(*serverURL, env)
	pp.start(env)
	defer pp.Close()

	if len(pp.Products()) != len(apiProducts) {
		t.Errorf("num products want: %d, got: %d", len(apiProducts), len(pp.Products()))
	}

	for _, want := range apiProducts {
		got := pp.Products()[want.Name]
		if want.Attributes[0].Value != got.Targets[0] {
			t.Errorf("targets not created: %v", got)
		}
	}
}

func TestManagerPolling(t *testing.T) {

	var count = 0
	var apiProducts []APIProduct
	oldPollInterval := pollInterval
	pollInterval = 5 * time.Millisecond
	defer func() {
		pollInterval = oldPollInterval
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		apiProducts = append(apiProducts, APIProduct{
			Name: fmt.Sprintf("Name %d", count),
			Attributes: []Attribute{
				{Name: servicesAttr, Value: "attr value"},
			},
		})

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

	pp := createManager(*serverURL, env)
	pp.start(env)
	defer pp.Close()

	pp1 := len(pp.Products())
	pp2 := len(pp.Products())
	if pp1 != pp2 {
		t.Errorf("number of products should not have incremented")
	}

	time.Sleep(pollInterval * 2)
	pp2 = len(pp.Products())
	if pp1 == pp2 {
		t.Errorf("number of products should have incremented")
	}
}
