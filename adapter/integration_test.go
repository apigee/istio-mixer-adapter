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

package adapter_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apigeeAdapter "github.com/apigee/istio-mixer-adapter/adapter"
	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"github.com/apigee/istio-mixer-adapter/adapter/product"
	"github.com/apigee/istio-mixer-adapter/adapter/quota"
	"github.com/dgrijalva/jwt-go"
	"github.com/lestrrat/go-jwx/jwk"
	"istio.io/istio/mixer/pkg/adapter"
	integration "istio.io/istio/mixer/pkg/adapter/test"
	authT "istio.io/istio/mixer/template/authorization"
)

func TestAuthorization(t *testing.T) {
	cases := map[string]struct {
		attrs map[string]interface{}
		want  string
	}{
		"Good api key request": {
			attrs: map[string]interface{}{
				"api.service":     "service",
				"request.path":    "/path",
				"request.api_key": "goodkey",
			},
			want: `
			{
			 "AdapterState": null,
			 "Returns": [
			  {
			   "Check": {
				"Status": {},
				"ValidDuration": 0,
				"ValidUseCount": 1
			   },
			   "Quota": null,
			   "Error": null
			  }
			 ]
			}
			`,
		},
		"Bad api key request": {
			attrs: map[string]interface{}{
				"api.service":     "service",
				"request.path":    "/path",
				"request.api_key": "badkey",
				"request.headers": map[string]string{},
			},
			want: `
			{
			 "AdapterState": null,
			 "Returns": [
			  {
			   "Check": {
				"Status": {
				 "code": 7,
				 "message": "handler.apigee.istio-system:invalid authentication"
				},
				"ValidDuration": 0,
				"ValidUseCount": 0
			   },
			   "Quota": null,
			   "Error": null
			  }
			 ]
			}
			`,
		},
		"Good JWT request": {
			attrs: map[string]interface{}{
				"api.service":             "service",
				"request.path":            "/path",
				"request.auth.raw_claims": `{"access_token":"8E7Az3ZgPHKrgzcQA54qAzXT3Z1G","api_product_list":["IstioTestProduct"],"application_name":"61cd4d83-06b5-4270-a9ee-cf9255ef45c3","audience":"istio","client_id":"yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H","exp":1521845533,"iat":1521845533,"iss":"https://theganyo1-eval-test.apigee.net/istio-auth/token","jti":"29e2320b-787c-4625-8599-acc5e05c68d0","nbf":1507636800,"scopes":["scope1","scope2"]}`,
				"request.headers":         map[string]string{},
			},
			want: `
			{
			 "AdapterState": null,
			 "Returns": [
			  {
			   "Check": {
				"Status": {},
				"ValidDuration": 0,
				"ValidUseCount": 1
			   },
			   "Quota": null,
			   "Error": null
			  }
			 ]
			}
			`,
		},
		"No matching products": {
			attrs: map[string]interface{}{
				"api.service":             "service",
				"request.path":            "/path",
				"request.auth.raw_claims": `{"access_token":"8E7Az3ZgPHKrgzcQA54qAzXT3Z1G","api_product_list":["NoMatchingProduct"],"application_name":"61cd4d83-06b5-4270-a9ee-cf9255ef45c3","audience":"istio","client_id":"yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H","exp":1521845533,"iat":1521845533,"iss":"https://theganyo1-eval-test.apigee.net/istio-auth/token","jti":"29e2320b-787c-4625-8599-acc5e05c68d0","nbf":1507636800,"scopes":["scope1","scope2"]}`,
				"request.headers":         map[string]string{},
			},
			want: `
			{
			 "AdapterState": null,
			 "Returns": [
			  {
			   "Check": {
				"Status": {
				 "code": 7,
				 "message": "handler.apigee.istio-system:not authorized"
				},
				"ValidDuration": 0,
				"ValidUseCount": 0
			   },
			   "Quota": null,
			   "Error": null
			  }
			 ]
			}
			`,
		},
		"Good JWT api key request": {
			attrs: map[string]interface{}{
				"api.service":             "service",
				"request.path":            "/path",
				"request.auth.raw_claims": `{"api_key":"goodkey"}`,
				"request.headers":         map[string]string{},
			},
			want: `
			{
			 "AdapterState": null,
			 "Returns": [
			  {
			   "Check": {
				"Status": {},
				"ValidDuration": 0,
				"ValidUseCount": 1
			   },
			   "Quota": null,
			   "Error": null
			  }
			 ]
			}
			`,
		},
		"Bad JWT api key request": {
			attrs: map[string]interface{}{
				"api.service":             "service",
				"request.path":            "/path",
				"request.auth.raw_claims": `{"api_key":"badkey"}`,
				"request.headers":         map[string]string{},
			},
			want: `
			{
			 "AdapterState": null,
			 "Returns": [
			  {
			   "Check": {
				"Status": {
				 "code": 7,
				 "message": "handler.apigee.istio-system:invalid authentication"
				},
				"ValidDuration": 0,
				"ValidUseCount": 0
			   },
			   "Quota": null,
			   "Error": null
			  }
			 ]
			}
			`,
		},
		"Bad JWT api key request, good api key request": {
			attrs: map[string]interface{}{
				"api.service":             "service",
				"request.path":            "/path",
				"request.auth.raw_claims": `{"api_key":"badkey"}`,
				"request.api_key":         "goodkey",
				"request.headers":         map[string]string{},
			},
			want: `
			{
			 "AdapterState": null,
			 "Returns": [
			  {
			   "Check": {
				"Status": {},
				"ValidDuration": 0,
				"ValidUseCount": 1
			   },
			   "Quota": null,
			   "Error": null
			  }
			 ]
			}
			`,
		},
		"Bad api keys, good JWT request": {
			attrs: map[string]interface{}{
				"api.service":             "service",
				"request.path":            "/path",
				"request.api_key":         "badkey",
				"request.auth.raw_claims": `{"access_token":"8E7Az3ZgPHKrgzcQA54qAzXT3Z1G","api_product_list":["IstioTestProduct"],"application_name":"61cd4d83-06b5-4270-a9ee-cf9255ef45c3","audience":"istio","client_id":"yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H","exp":1521845533,"iat":1521845533,"iss":"https://theganyo1-eval-test.apigee.net/istio-auth/token","jti":"29e2320b-787c-4625-8599-acc5e05c68d0","nbf":1507636800,"scopes":["scope1","scope2"],"api_key":"badkey"}`,
				"request.headers":         map[string]string{},
			},
			want: `
			{
			 "AdapterState": null,
			 "Returns": [
			  {
			   "Check": {
				"Status": {},
				"ValidDuration": 0,
				"ValidUseCount": 1
			   },
			   "Quota": null,
			   "Error": null
			  }
			 ]
			}
			`,
		},
		"No auth request": {
			attrs: map[string]interface{}{
				"api.service":     "service",
				"request.path":    "/path",
				"request.headers": map[string]string{},
			},
			want: `
			{
			 "AdapterState": null,
			 "Returns": [
			  {
			   "Check": {
				"Status": {
				"code": 7,
				"message": "handler.apigee.istio-system:missing authentication"
				},
				"ValidDuration": 0,
				"ValidUseCount": 0
			   },
			   "Quota": null,
			   "Error": null
			  }
			 ]
			}
			`,
		},
		"Quota exceeded request": {
			attrs: map[string]interface{}{
				"api.service":     "service",
				"request.path":    "/ExceededQuota",
				"request.api_key": "goodkey",
				"request.headers": map[string]string{},
			},
			want: `
			{
			 "AdapterState": null,
			 "Returns": [
			  {
			   "Check": {
				"Status": {
				"code": 8,
				"message": "handler.apigee.istio-system:quota exceeded"
				},
				"ValidDuration": 0,
				"ValidUseCount": 1
			   },
			   "Quota": null,
			   "Error": null
			  }
			 ]
			}
			`,
		},
	}

	info := apigeeAdapter.GetInfo()

	// delete analytics as the custom template is not supported by test framework
	info.SupportedTemplates = []string{
		authT.TemplateName,
	}

	ts := httptest.NewServer(cloudMockHandler(t))
	defer ts.Close()

	serviceCfg := strings.Replace(adapterConfig, "__SERVER_BASE_URL__", ts.URL, -1)
	for id, c := range cases {
		t.Logf("** Executing test case '%s' **", id)
		integration.RunTest(
			t,
			func() adapter.Info {
				return info
			},
			integration.Scenario{
				ParallelCalls: []integration.Call{
					{
						CallKind: integration.CHECK,
						Attrs:    c.attrs,
					},
				},

				Setup: func() (interface{}, error) {
					return nil, nil
				},

				Teardown: func(ctx interface{}) {
				},

				GetState: func(ctx interface{}) (interface{}, error) {
					return nil, nil
				},

				Configs: []string{
					serviceCfg,
				},

				Want: c.want,
			},
		)
	}
}

func cloudMockHandler(t *testing.T) http.HandlerFunc {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	apiProducts := []product.APIProduct{
		{
			Attributes: []product.Attribute{
				{Name: product.ServicesAttr, Value: "service"},
			},
			Name:          "IstioTestProduct",
			Resources:     []string{"/"},
			Scopes:        []string{"scope1"},
			QuotaLimit:    "1",
			QuotaTimeUnit: "second",
			QuotaInterval: "1",
		},
		{
			Attributes: []product.Attribute{
				{Name: product.ServicesAttr, Value: "service"},
			},
			Name:          "ExceededQuota",
			Resources:     []string{"/ExceededQuota"},
			Scopes:        []string{"scope1"},
			QuotaLimit:    "1",
			QuotaTimeUnit: "second",
			QuotaInterval: "1",
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		switch {
		case strings.HasPrefix(r.URL.Path, "/certs"):
			key, err := jwk.New(&privateKey.PublicKey)
			if err != nil {
				t.Fatal(err)
			}
			key.Set("kid", "1")
			key.Set("alg", jwt.SigningMethodRS256.Alg())

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
			keyReq := auth.APIKeyRequest{}
			json.NewDecoder(r.Body).Decode(&keyReq)
			if keyReq.APIKey != "goodkey" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(401)
				w.Write([]byte(`{"fault":{"faultstring":"Invalid ApiKey","detail":{"errorcode":"oauth.v2.InvalidApiKey"}}}`))
				return
			}

			jwtToken, err := generateJWT(privateKey)
			if err != nil {
				t.Fatal(err)
			}
			jwtResponse := auth.APIKeyResponse{Token: jwtToken}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(jwtResponse)

		case strings.HasPrefix(r.URL.Path, "/quotas"):
			req := quota.Request{}
			json.NewDecoder(r.Body).Decode(&req)
			result := quota.Result{
				Allowed:    20,
				Used:       req.Weight,
				Exceeded:   0,
				ExpiryTime: time.Now().Unix(),
				Timestamp:  time.Now().Unix(),
			}
			if strings.HasSuffix(req.Identifier, "ExceededQuota") {
				result.Used = 25
			}
			if result.Used > result.Allowed {
				result.Exceeded = result.Used - result.Allowed
				result.Used = result.Allowed
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)

		case strings.HasPrefix(r.URL.Path, "/products"):
			var result = product.APIResponse{
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
			"IstioTestProduct",
			"ExceededQuota",
		},
		"audience":         "istio",
		"jti":              "29e2320b-787c-4625-8599-acc5e05c68d0",
		"iss":              "https://theganyo1-eval-test.apigee.net/istio-auth/token",
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

const (
	adapterConfig = `
apiVersion: config.istio.io/v1alpha2
kind: apigee
metadata:
  name: handler
  namespace: istio-system
spec:
  apigee_base: __SERVER_BASE_URL__
  customer_base: __SERVER_BASE_URL__
  org_name: org
  env_name: env
  key: key
  secret: secret
  api_key_claim: api_key
---
apiVersion: config.istio.io/v1alpha2
kind: rule
metadata:
  name: apigee-rule
  namespace: istio-system
spec:
  actions:
  - handler: handler.apigee
    instances:
    - apigee.authorization
---
# instance configuration for template 'apigee.authorization'
apiVersion: config.istio.io/v1alpha2
kind: authorization
metadata:
  name: apigee
  namespace: istio-system
spec:
  subject:
    user: ""
    groups: ""
    properties:
      json_claims: request.auth.raw_claims | ""
      api_key: request.api_key | request.headers["x-api-key"] | ""
  action:
    namespace: destination.namespace | "default"
    service: api.service | destination.service | ""
    path: api.operation | request.path | ""
    method: request.method | ""
`
)
