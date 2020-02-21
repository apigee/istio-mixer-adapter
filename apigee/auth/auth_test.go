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

package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/authtest"
	"github.com/apigee/istio-mixer-adapter/apigee/context"
)

type testVerifier struct {
	keyErrors map[string]error
	claims    map[string]interface{}
}

var testJWTClaims = map[string]interface{}{
	"client_id":        "hi",
	"application_name": "taco",
	"exp":              14.0,
	"api_product_list": []string{"superapp"},
	"scopes":           []string{"scope"},
}

func (tv *testVerifier) Verify(ctx context.Context, apiKey string) (map[string]interface{}, error) {
	err := tv.keyErrors[apiKey]
	if err != nil {
		return nil, err
	}

	return testJWTClaims, nil
}

func TestNewManager(t *testing.T) {
	opts := Options{
		PollInterval: time.Hour,
		Client:       &http.Client{},
	}
	man, err := NewManager(opts)
	if err != nil {
		t.Fatalf("create and start manager: %v", err)
	}
	if opts.PollInterval != man.jwtMan.pollInterval {
		t.Errorf("pollInterval want: %v, got: %v", opts.PollInterval, man.jwtMan.pollInterval)
	}
	verifier := man.verifier.(*keyVerifierImpl)
	if opts.Client != verifier.client {
		t.Errorf("client want: %v, got: %v", opts.Client, verifier.client)
	}
	man.Close()
}

func TestAuthenticate(t *testing.T) {
	goodAPIKey := "good"
	badAPIKey := "bad"
	errAPIKey := "error"
	missingProductListError := "api_product_list claim is required"

	for _, test := range []struct {
		desc           string
		apiKey         string
		apiKeyClaimKey string
		claims         map[string]interface{}
		wantError      string
	}{
		{"with valid JWT", "", "", testJWTClaims, ""},
		{"with invalid JWT", "", "", map[string]interface{}{"exp": "1"}, missingProductListError},
		{"with valid API key", goodAPIKey, "", nil, ""},
		{"with invalid API key", badAPIKey, "", nil, ErrBadAuth.Error()},
		{"with valid claims API key", "", "goodkey", map[string]interface{}{
			"exp":              "1",
			"api_product_list": "[]",
			"goodkey":          goodAPIKey,
		}, ""},
		{"with invalid claims API key", "", "badkey", map[string]interface{}{
			"exp":     "1",
			"somekey": goodAPIKey,
			"badkey":  badAPIKey,
		}, ErrBadAuth.Error()},
		{"with missing claims API key", "", "missingkey", map[string]interface{}{
			"exp": "1",
		}, missingProductListError},
		{"error verifying API key", errAPIKey, "", nil, ErrInternalError.Error()},
	} {
		t.Log(test.desc)

		jwtMan := newJWTManager(time.Hour)
		tv := &testVerifier{
			keyErrors: map[string]error{
				goodAPIKey: nil,
				badAPIKey:  ErrBadAuth,
				errAPIKey:  ErrInternalError,
			},
		}
		authMan := &Manager{
			jwtMan:   jwtMan,
			verifier: tv,
		}
		authMan.start()
		defer authMan.Close()

		ctx := authtest.NewContext("")
		_, err := authMan.Authenticate(ctx, test.apiKey, test.claims, test.apiKeyClaimKey)
		if err != nil {
			if test.wantError != err.Error() {
				t.Errorf("wanted error: %s, got: %s", test.wantError, err.Error())
			}
		} else if test.wantError != "" {
			t.Errorf("wanted error, got none")
		}
	}
}
