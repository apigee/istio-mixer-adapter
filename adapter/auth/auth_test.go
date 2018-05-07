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
	"fmt"
	"testing"

	"github.com/apigee/istio-mixer-adapter/adapter/authtest"
	"github.com/apigee/istio-mixer-adapter/adapter/context"
	adaptertest "istio.io/istio/mixer/pkg/adapter/test"
)

type testVerifier struct {
	goodAPIKey string
	claims     map[string]interface{}
}

var testJWTClaims = map[string]interface{}{
	"client_id":        "hi",
	"application_name": "taco",
	"exp":              14.0,
	"api_product_list": []string{"superapp"},
	"scopes":           []string{"scope"},
}

func (tv *testVerifier) Verify(ctx context.Context, apiKey string) (map[string]interface{}, error) {
	if apiKey != tv.goodAPIKey {
		return nil, fmt.Errorf("invalid auth key")
	}
	// Just return some dummy value.
	return testJWTClaims, nil
}

func TestAuthenticate(t *testing.T) {
	goodAPIKey := "good"

	for _, test := range []struct {
		desc      string
		apiKey    string
		claims    map[string]interface{}
		wantError bool
	}{
		{"with valid JWT", "", testJWTClaims, false},
		{"with invalid JWT", "", map[string]interface{}{
			"client_id": "bad",
		}, true},
		{"with valid API key", "good", nil, false},
		{"with invalid API key", "bad", nil, true},
	} {
		t.Log(test.desc)

		env := adaptertest.NewEnv(t)

		jwtMan := newJWTManager()
		tv := &testVerifier{
			goodAPIKey: goodAPIKey,
		}
		authMan := &Manager{
			env:      env,
			jwtMan:   jwtMan,
			verifier: tv,
		}
		authMan.start()
		defer authMan.Close()

		ctx := authtest.NewContext("", adaptertest.NewEnv(t))
		_, err := authMan.Authenticate(ctx, test.apiKey, test.claims)
		if err != nil {
			if !test.wantError {
				t.Errorf("unexpected error: %s", err)
			}
			continue
		}
		if test.wantError {
			t.Errorf("wanted error, got none")
			continue
		}
	}
}
