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
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/lestrrat/go-jwx/jwk"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/pkg/adapter/test"
)

var badKeyResponse = []byte(`{"fault":{"faultstring":"Invalid ApiKey","detail":{"errorcode":"oauth.v2.InvalidApiKey"}}}`)

func TestVerifyAPIKeyValid(t *testing.T) {

	apiKey := "testID"

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, jwksPath) {
			key, err := jwk.New(&privateKey.PublicKey)
			if err != nil {
				t.Fatal(err)
			}
			key.Set("kid", "1")

			type JWKS struct {
				Keys []jwk.Key `json:"keys"`
			}

			jwks := JWKS{
				Keys: []jwk.Key{
					key,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(jwks)
			return
		}

		var req ApiKeyRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()

		if apiKey != req.ApiKey {
			t.Fatalf("expected: %v, got: %v", apiKey, req)
		}

		jwt, err := generateJWT(privateKey)
		if err != nil {
			t.Fatal(err)
		}

		jwtResponse := ApiKeyResponse{Token: jwt}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwtResponse)
	}))
	defer ts.Close()

	serverURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	ctx := &TestContext{
		apigeeBase:   *serverURL,
		customerBase: *serverURL,
		log:          test.NewEnv(t),
	}

	claims, err := VerifyAPIKey(ctx, apiKey)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("\nclaims: %v", claims)

	if claims["client_id"].(string) != "yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H" {
		t.Errorf("bad client_id, got: %s, want: %s", claims["client_id"].(string), "yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H")
	}

	if claims["application_name"].(string) != "61cd4d83-06b5-4270-a9ee-cf9255ef45c3" {
		t.Errorf("bad client_id, got: %s, want: %s", claims["application_name"].(string), "61cd4d83-06b5-4270-a9ee-cf9255ef45c3")
	}
}

func TestVerifyAPIKeyFail(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(badKeyResponse)
	}))
	defer ts.Close()

	serverURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	ctx := &TestContext{
		apigeeBase:   *serverURL,
		customerBase: *serverURL,
		log:          test.NewEnv(t),
	}
	success, err := VerifyAPIKey(ctx, "badKey")

	if success != nil {
		t.Errorf("success should be nil, is: %v", success)
	}

	if err == nil {
		t.Errorf("error should not be nil")
	}
}

func TestVerifyAPIKeyError(t *testing.T) {

	ctx := &TestContext{
		apigeeBase:   url.URL{},
		customerBase: url.URL{},
		log:          test.NewEnv(t),
	}
	success, err := VerifyAPIKey(ctx, "badKey")

	if err == nil {
		t.Errorf("error should be nil")
	}

	if success != nil {
		t.Errorf("success should be nil, is: %v", success)
	}
}

func generateJWT(privateKey *rsa.PrivateKey) (string, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"api_product_list": []string{
			"EdgeMicroTestProduct",
		},
		"audience":         "microgateway",
		"jti":              "29e2320b-787c-4625-8599-acc5e05c68d0",
		"iss":              "https://theganyo1-eval-test.apigee.net/edgemicro-auth/token",
		"access_token":     "8E7Az3ZgPHKrgzcQA54qAzXT3Z1G",
		"client_id":        "yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H",
		"application_name": "61cd4d83-06b5-4270-a9ee-cf9255ef45c3",
		"scopes": []string{
			"scope1",
			"scope2",
		},
		"nbf": time.Date(2017, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
		"iat": time.Now().Unix(),
		"exp": (time.Now().Add(time.Minute)).Unix(),
	})

	token.Header["kid"] = "1"

	t, e := token.SignedString(privateKey)

	return t, e
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

/*
jwt claims:
{
 api_product_list: [
  "EdgeMicroTestProduct"
 ],
 audience: "microgateway",
 jti: "29e2320b-787c-4625-8599-acc5e05c68d0",
 iss: "https://theganyo1-eval-test.apigee.net/edgemicro-auth/token",
 access_token: "8E7Az3ZgPHKrgzcQA54qAzXT3Z1G",
 client_id: "yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H",
 nbf: 1516387728,
 iat: 1516387728,
 application_name: "61cd4d83-06b5-4270-a9ee-cf9255ef45c3",
 scopes: [
  "scope1",
  "scope2"
 ],
 exp: 1516388028
}
*/
