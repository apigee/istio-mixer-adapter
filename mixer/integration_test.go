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

package mixer_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apigee/istio-mixer-adapter/mixer"
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
				"Returns": [{
					"Check": {
						"Status": {},
						"ValidDuration": 0,
						"ValidUseCount": 1
					},
					"Quota": null,
					"Error": null
				}]
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
				"Returns": [{
					"Check": {
						"Status": {
							"code": 7,
							"message": "handler.apigee.istio-system:permission denied"
						},
						"ValidDuration": 0,
						"ValidUseCount": 0
					},
					"Quota": null,
					"Error": null
				}]
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
				"Returns": [{
					"Check": {
						"Status": {},
						"ValidDuration": 0,
						"ValidUseCount": 1
					},
					"Quota": null,
					"Error": null
				}]
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
				"Returns": [{
					"Check": {
						"Status": {
							"code": 7,
							"message": "handler.apigee.istio-system:permission denied"
						},
						"ValidDuration": 0,
						"ValidUseCount": 0
					},
					"Quota": null,
					"Error": null
				}]
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
				"Returns": [{
					"Check": {
						"Status": {},
						"ValidDuration": 0,
						"ValidUseCount": 1
					},
					"Quota": null,
					"Error": null
				}]
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
				"Returns": [{
					"Check": {
						"Status": {
							"code": 7,
							"message": "handler.apigee.istio-system:permission denied"
						},
						"ValidDuration": 0,
						"ValidUseCount": 0
					},
					"Quota": null,
					"Error": null
				}]
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
				"Returns": [{
					"Check": {
						"Status": {},
						"ValidDuration": 0,
						"ValidUseCount": 1
					},
					"Quota": null,
					"Error": null
				}]
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
				"Returns": [{
					"Check": {
						"Status": {},
						"ValidDuration": 0,
						"ValidUseCount": 1
					},
					"Quota": null,
					"Error": null
				}]
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
				"Returns": [{
					"Check": {
						"Status": {
							"code": 16,
							"message": "handler.apigee.istio-system:missing authentication"
						},
						"ValidDuration": 0,
						"ValidUseCount": 0
					},
					"Quota": null,
					"Error": null
				}]
			}
			`,
		},
		"Quota request": { // note: cannot test quota exceeded via integration
			attrs: map[string]interface{}{
				"api.service":     "service",
				"request.path":    "/ExceededQuota",
				"request.api_key": "goodkey",
				"request.headers": map[string]string{},
			},
			want: `
			{
				"Returns": [{
					"Check": {
						"Status": {},
						"ValidDuration": 0,
						"ValidUseCount": 1
					}
				}]
			}			
			`,
		},
	}

	info := mixer.GetInfo()

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
  auth:
    api_key_claim: api_key
    api_key_cache_duration: 30m
  allowUnverifiedSSLCert: false
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
    service: api.service | destination.service.host | ""
    path: api.operation | request.path | ""
    method: request.method | ""
`
)

func TestUnknownCert(t *testing.T) {
	c := struct {
		attrs map[string]interface{}
		want  string
	}{
		attrs: map[string]interface{}{
			"api.service":     "service",
			"request.path":    "/path",
			"request.api_key": "goodkey",
		},
		want: `
		{
			"AdapterState": null,
			"Returns": [{
				"Check": {
					"Status": {
						"code": 7,
						"message": "handler.apigee.istio-system:internal error"
					},
					"ValidDuration": 0,
					"ValidUseCount": 0
				},
				"Quota": null,
				"Error": {}
			}]
		}		
		`,
	}

	info := mixer.GetInfo()

	// delete analytics as the custom template is not supported by test framework
	info.SupportedTemplates = []string{
		authT.TemplateName,
	}

	ts := httptest.NewTLSServer(cloudMockHandler(t))
	defer ts.Close()

	serviceCfg := strings.Replace(adapterConfig, "__SERVER_BASE_URL__", ts.URL, -1)

	aif := func() adapter.Info {
		return info
	}

	// should fail, bad SSL cert
	integration.RunTest(t, aif, integration.Scenario{
		ParallelCalls: []integration.Call{
			{
				CallKind: integration.CHECK,
				Attrs:    c.attrs,
			},
		},
		Configs: []string{serviceCfg},
		Want:    c.want,
	})

	serviceCfg = strings.Replace(serviceCfg, "allowUnverifiedSSLCert: false", "allowUnverifiedSSLCert: true", -1)

	c.want = `
	{
		"AdapterState": null,
		"Returns": [{
			"Check": {
				"Status": {},
				"ValidDuration": 0,
				"ValidUseCount": 1
			},
			"Quota": null,
			"Error": null
		}]
	}	
	`

	// should succeed, ignore bad SSL cert
	integration.RunTest(t, aif, integration.Scenario{
		ParallelCalls: []integration.Call{
			{
				CallKind: integration.CHECK,
				Attrs:    c.attrs,
			},
		},
		Configs: []string{serviceCfg},
		Want:    c.want,
	})

}
