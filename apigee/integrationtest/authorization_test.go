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
	"net/http/httptest"
	"strings"
	"testing"

	integration "istio.io/istio/mixer/pkg/adapter/test"
)

/*
  subject:
    user: ""
    groups: ""
    properties:
      encoded_claims: request.headers["sec-istio-auth-userinfo"] | ""
      api_key: request.api_key | request.headers["x-api-key"] | ""
  action:
    namespace: destination.namespace | "default"
    service: api.service | destination.service | ""
    path: api.operation | request.path | ""
    method: request.method | ""
*/
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
				"request.headers": map[string]string{
					"sec-istio-auth-userinfo": "",
				},
			},
			want: `
			{
			 "AdapterState": null,
			 "Returns": [
			  {
			   "Check": {
				"Status": {
				 "code": 7,
				 "message": "apigee-handler.apigee.istio-system:invalid api key"
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
				"api.service":  "service",
				"request.path": "/path",
				"request.headers": map[string]string{
					"sec-istio-auth-userinfo": "eyJhY2Nlc3NfdG9rZW4iOiI4RTdBejNaZ1BIS3JnemNRQTU0cUF6WFQzWjFHIiwiYXBpX3Byb2R1Y3RfbGlzdCI6WyJFZGdlTWljcm9UZXN0UHJvZHVjdCJdLCJhcHBsaWNhdGlvbl9uYW1lIjoiNjFjZDRkODMtMDZiNS00MjcwLWE5ZWUtY2Y5MjU1ZWY0NWMzIiwiYXVkaWVuY2UiOiJtaWNyb2dhdGV3YXkiLCJjbGllbnRfaWQiOiJ5QlE1ZVhaQThyU29pcFlFaTFSbW4wWjhSS3RrR0k0SCIsImV4cCI6MTUyMTg0NTUzMywiaWF0IjoxNTIxODQ1NTMzLCJpc3MiOiJodHRwczovL3RoZWdhbnlvMS1ldmFsLXRlc3QuYXBpZ2VlLm5ldC9lZGdlbWljcm8tYXV0aC90b2tlbiIsImp0aSI6IjI5ZTIzMjBiLTc4N2MtNDYyNS04NTk5LWFjYzVlMDVjNjhkMCIsIm5iZiI6MTUwNzYzNjgwMCwic2NvcGVzIjpbInNjb3BlMSIsInNjb3BlMiJdfQ",
				},
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
	}

	ts := httptest.NewServer(CloudMockHandler(t))
	defer ts.Close()

	serviceCfg := strings.Replace(adapterConfigForAuthorization(), "__SERVER_BASE_URL__", ts.URL, -1)

	for id, c := range cases {
		t.Logf("** Executing test case '%s' **", id)
		integration.RunTest(
			t,
			testGetInfo,
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

				Configs: []string{
					serviceCfg,
				},
				Want: c.want,
			},
		)
	}
}

/*
JWT Payload:
{
 "access_token": "8E7Az3ZgPHKrgzcQA54qAzXT3Z1G",
 "api_product_list": [
  "EdgeMicroTestProduct"
 ],
 "application_name": "61cd4d83-06b5-4270-a9ee-cf9255ef45c3",
 "audience": "microgateway",
 "client_id": "yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H",
 "exp": 1521849238,
 "iat": 1521845533,
 "iss": "https://theganyo1-eval-test.apigee.net/edgemicro-auth/token",
 "jti": "29e2320b-787c-4625-8599-acc5e05c68d0",
 "nbf": 1507636800,
 "scopes": [
  "scope1",
  "scope2"
 ]
}
*/
