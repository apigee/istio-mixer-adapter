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

package integrationtest

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/product"
	"github.com/apigee/istio-mixer-adapter/apigee/quota"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/lestrrat/go-jwx/jwk"
)

func CloudMockHandler(t *testing.T) http.HandlerFunc {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	apiProducts := []product.APIProduct{
		{
			Attributes: []product.Attribute{
				{Name: servicesAttr, Value: "service"},
			},
			Name:          "EdgeMicroTestProduct",
			Resources:     []string{"/"},
			Scopes:        []string{"scope1"},
			QuotaLimit:    "1",
			QuotaTimeUnit: "second",
			QuotaInterval: "1",
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		switch {
		case strings.HasPrefix(r.URL.Path, "/jwkPublicKeys"):
			key, err := jwk.New(&privateKey.PublicKey)
			if err != nil {
				t.Fatal(err)
			}
			key.Set("kid", "1")

			jwks := struct {
				Keys []jwk.Key `json:"keys"`
			}{
				Keys: []jwk.Key{
					key,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(jwks)

		case strings.HasPrefix(r.URL.Path, "/verifyApiKey"):
			keyReq := apiKeyRequest{}
			json.NewDecoder(r.Body).Decode(&keyReq)
			if keyReq.APIKey != "goodkey" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(401)
				w.Write([]byte(`{"fault":{"faultstring":"Invalid ApiKey","detail":{"errorcode":"oauth.v2.InvalidApiKey"}}}`))
				return
			}

			jwt, err := generateJWT(privateKey)
			if err != nil {
				t.Fatal(err)
			}
			jwtResponse := apiKeyResponse{Token: jwt}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(jwtResponse)

		case strings.HasPrefix(r.URL.Path, "/quotas"):
			result := quota.Result{
				Allowed:    1,
				Used:       1,
				Exceeded:   0,
				ExpiryTime: time.Now().Unix(),
				Timestamp:  time.Now().Unix(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)

		case strings.HasPrefix(r.URL.Path, "/products"):
			var result = apiResponse{
				APIProducts: apiProducts,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)

		}
	})
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

const servicesAttr = "istio-services"

type apiResponse struct {
	APIProducts []product.APIProduct `json:"apiProduct"`
}

type apiKeyResponse struct {
	Token string `json:"token"`
}

type apiKeyRequest struct {
	APIKey string `json:"apiKey"`
}
