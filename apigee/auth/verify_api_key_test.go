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

var (
	badKeyResponse = []byte(`{"fault":{"faultstring":"Invalid ApiKey","detail":{"errorcode":"oauth.v2.InvalidApiKey"}}}`)
)

// goodHandler is an HTTP handler that handles all the requests in a proper fashion.
func goodHandler(apiKey string, t *testing.T) http.HandlerFunc {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, jwksPath) {
			// Handling the JWK verifier
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

		var req apiKeyRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()

		if apiKey != req.APIKey {
			t.Fatalf("expected: %v, got: %v", apiKey, req)
		}

		jwt, err := generateJWT(privateKey)
		if err != nil {
			t.Fatal(err)
		}

		jwtResponse := apiKeyResponse{Token: jwt}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwtResponse)
	}
}

// badHandler gives a handler that just gives a 401 for all requests.
func badHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(badKeyResponse)
	}
}

func TestVerifyAPIKeyValid(t *testing.T) {
	env := test.NewEnv(t)
	jwtMan := newJWTManager()
	jwtMan.start(env)
	defer jwtMan.stop()
	v := newVerifier(jwtMan, keyVerifierOpts{})

	apiKey := "testID"

	ts := httptest.NewServer(goodHandler(apiKey, t))
	defer ts.Close()

	serverURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	ctx := &testContext{
		apigeeBase:   *serverURL,
		customerBase: *serverURL,
		log:          test.NewEnv(t),
	}

	claims, err := v.Verify(ctx, apiKey)
	if err != nil {
		t.Fatal(err)
	}

	if claims["client_id"].(string) != "yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H" {
		t.Errorf("bad client_id, got: %s, want: %s", claims["client_id"].(string), "yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H")
	}

	if claims["application_name"].(string) != "61cd4d83-06b5-4270-a9ee-cf9255ef45c3" {
		t.Errorf("bad client_id, got: %s, want: %s", claims["application_name"].(string), "61cd4d83-06b5-4270-a9ee-cf9255ef45c3")
	}
}

func TestVerifyAPIKeyCacheWithClear(t *testing.T) {
	env := test.NewEnv(t)
	jwtMan := newJWTManager()
	jwtMan.start(env)
	defer jwtMan.stop()
	v := newVerifier(jwtMan, keyVerifierOpts{})

	apiKey := "testID"

	// On the first iteration, use a normal HTTP handler that will return good
	// results for the various HTTP requests that go out. After the first run,
	// replace with bad responses to ensure that we do not go out and fetch any
	// new pages (things are cached).
	called := map[string]bool{}
	good := goodHandler(apiKey, t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !called[r.URL.Path] {
			called[r.URL.Path] = true
			good(w, r)
		} else {
			badHandler()(w, r)
		}
	}))
	defer ts.Close()

	serverURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	ctx := &testContext{
		apigeeBase:   *serverURL,
		customerBase: *serverURL,
		log:          test.NewEnv(t),
	}

	for i := 0; i < 5; i++ {
		claims, err := v.Verify(ctx, apiKey)
		if err != nil {
			t.Fatal(err)
		}

		if claims["client_id"].(string) != "yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H" {
			t.Errorf("bad client_id, got: %s, want: %s", claims["client_id"].(string), "yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H")
		}

		if claims["application_name"].(string) != "61cd4d83-06b5-4270-a9ee-cf9255ef45c3" {
			t.Errorf("bad client_id, got: %s, want: %s", claims["application_name"].(string), "61cd4d83-06b5-4270-a9ee-cf9255ef45c3")
		}
	}

	// Clear the cache.
	v.(*keyVerifierImpl).cache.RemoveAll()

	_, err = v.Verify(ctx, apiKey)
	if err == nil {
		t.Errorf("expected error result on cleared cache")
	}
}

func TestVerifyAPIKeyCacheWithExpiry(t *testing.T) {
	env := test.NewEnv(t)
	jwtMan := newJWTManager()
	jwtMan.start(env)
	defer jwtMan.stop()
	v := newVerifier(jwtMan, keyVerifierOpts{
		CacheEvictionInterval: 50 * time.Millisecond,
	})

	apiKey := "testID"

	// On the first iteration, use a normal HTTP handler that will return good
	// results for the various HTTP requests that go out. After the first run,
	// replace with bad responses to ensure that we do not go out and fetch any
	// new pages (things are cached).
	called := false
	good := goodHandler(apiKey, t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, jwksPath) {
			// We don't care about jwks expiry here.
			good(w, r)
			return
		}
		if !called {
			called = true
			good(w, r)
		} else {
			badHandler()(w, r)
		}
	}))
	defer ts.Close()

	serverURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	ctx := &testContext{
		apigeeBase:   *serverURL,
		customerBase: *serverURL,
		log:          test.NewEnv(t),
	}

	for i := 0; i < 5; i++ {
		t.Logf("run %d", i)
		claims, err := v.Verify(ctx, apiKey)
		if err != nil {
			t.Fatal(err)
		}

		if claims["client_id"].(string) != "yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H" {
			t.Errorf("bad client_id, got: %s, want: %s", claims["client_id"].(string), "yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H")
		}

		if claims["application_name"].(string) != "61cd4d83-06b5-4270-a9ee-cf9255ef45c3" {
			t.Errorf("bad client_id, got: %s, want: %s", claims["application_name"].(string), "61cd4d83-06b5-4270-a9ee-cf9255ef45c3")
		}
	}

	// Wait until the key is expired. This should give us an error since we are
	// now going to make an HTTP request that will fail.
	time.Sleep(200 * time.Millisecond)

	_, err = v.Verify(ctx, apiKey)
	if err == nil {
		t.Errorf("expected error result on cleared cache")
	}
}

func TestVerifyAPIKeyFail(t *testing.T) {
	env := test.NewEnv(t)
	jwtMan := newJWTManager()
	jwtMan.start(env)
	defer jwtMan.stop()
	v := newVerifier(jwtMan, keyVerifierOpts{})

	ts := httptest.NewServer(badHandler())
	defer ts.Close()

	serverURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	ctx := &testContext{
		apigeeBase:   *serverURL,
		customerBase: *serverURL,
		log:          test.NewEnv(t),
	}
	success, err := v.Verify(ctx, "badKey")

	if success != nil {
		t.Errorf("success should be nil, is: %v", success)
	}

	if err == nil {
		t.Errorf("error should not be nil")
	} else if err.Error() != "invalid api key" {
		t.Errorf("got error: '%s', expected: 'invalid api key'", err.Error())
	}
}

func TestVerifyAPIKeyError(t *testing.T) {
	env := test.NewEnv(t)
	jwtMan := newJWTManager()
	jwtMan.start(env)
	defer jwtMan.stop()
	v := newVerifier(jwtMan, keyVerifierOpts{})

	ctx := &testContext{
		apigeeBase:   url.URL{},
		customerBase: url.URL{},
		log:          test.NewEnv(t),
	}
	success, err := v.Verify(ctx, "badKey")

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
		"exp": (time.Now().Add(50 * time.Millisecond)).Unix(),
	})

	token.Header["kid"] = "1"

	t, e := token.SignedString(privateKey)

	return t, e
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
