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
	"testing"
	"time"

	"reflect"

	"fmt"

	"istio.io/istio/mixer/pkg/adapter/test"
)

func TestManager(t *testing.T) {

	allDetails := []Details{
		{
			Attributes: []Attribute{
				{Name: "attr name", Value: "attr value"},
			},
			CreatedAt:      time.Now().Unix(),
			CreatedBy:      "test1@apigee.com",
			Description:    "details 1",
			DisplayName:    "Details 1",
			Environments:   []string{"test"},
			LastModifiedAt: time.Now().Unix(),
			LastModifiedBy: "test@apigee.com",
			Name:           "Name 1",
			QuotaLimit:     "10",
			QuotaInterval:  1,
			QuotaTimeUnit:  "minute",
			Resources:      []string{"/"},
			Scopes:         []string{"scope1"},
		},
		{
			Attributes: []Attribute{
				{Name: "attr name", Value: "attr value"},
			},
			CreatedAt:      time.Now().Unix(),
			CreatedBy:      "test2@apigee.com",
			Description:    "details 1",
			DisplayName:    "Details 2",
			Environments:   []string{"prod"},
			LastModifiedAt: time.Now().Unix(),
			LastModifiedBy: "test@apigee.com",
			Name:           "Name 2",
			QuotaLimit:     "20",
			QuotaInterval:  1,
			QuotaTimeUnit:  "hour",
			Resources:      []string{"/"},
			Scopes:         []string{"scope1"},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result = apiResponse{
			APIProducts: allDetails,
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

	pp := createProductManager(*serverURL, env)
	pp.start(env)
	defer pp.close()

	if len(pp.getProducts()) != len(allDetails) {
		t.Errorf("num details want: %d, got: %d", len(allDetails), len(pp.getProducts()))
	}

	for _, want := range allDetails {
		got := pp.getProducts()[want.Name]
		if !reflect.DeepEqual(got, want) {
			t.Errorf("provided and received details don't match, got: %v, want: %v", got, want)
		}
	}
}

func TestManagerPolling(t *testing.T) {

	var count = 0
	var allDetails []Details
	oldPollInterval := pollInterval
	pollInterval = 5 * time.Millisecond
	defer func() {
		pollInterval = oldPollInterval
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		allDetails = append(allDetails, Details{
			Name: fmt.Sprintf("Name %d", count),
		})

		var result = apiResponse{
			APIProducts: allDetails,
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

	pp := createProductManager(*serverURL, env)
	pp.start(env)
	defer pp.close()

	pp1 := len(pp.getProducts())
	pp2 := len(pp.getProducts())
	if pp1 != pp2 {
		t.Errorf("number of products should not have incremented")
	}

	time.Sleep(pollInterval * 2)
	pp2 = len(pp.getProducts())
	if pp1 == pp2 {
		t.Errorf("number of products should have incremented")
	}
}
