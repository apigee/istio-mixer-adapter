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

	"github.com/apigee/istio-mixer-adapter/apigee"
	mixer "istio.io/api/mixer/v1"
	"istio.io/istio/mixer/pkg/adapter"
	integration "istio.io/istio/mixer/pkg/adapter/test"
	quotaT "istio.io/istio/mixer/template/quota"
)

const (
	adapterConfig = `
apiVersion: config.istio.io/v1alpha2
kind: apigee
metadata:
  name: apigee-handler
  namespace: istio-system
spec:
  apigee_base: __SERVER_BASE_URL__
  customer_base: __SERVER_BASE_URL__
  org_name: org
  env_name: env
  key: key
  secret: secret

---

apiVersion: config.istio.io/v1alpha2
kind: rule
metadata:
  name: apigee-rule
  namespace: istio-system
spec:
  actions:
  - handler: apigee-handler.apigee
    instances:
    - apigee.quota

---

# instance configuration for template 'apigee.quota'
apiVersion: config.istio.io/v1alpha2
kind: quota
metadata:
  name: apigee
  namespace: istio-system
spec:
  dimensions:
    api: api.service | destination.service | ""
    path: request.path | ""
    api_key: request.api_key | request.headers["x-api-key"] | ""
    encoded_claims: request.headers["sec-istio-auth-userinfo"] | ""
`
)

func testGetInfo() adapter.Info {
	info := apigee.GetInfo()
	info.SupportedTemplates = []string{quotaT.TemplateName}
	return info
}

/*
   api: api.service | destination.service | ""
   path: request.path | ""
   api_key: request.api_key | request.headers["x-api-key"] | ""
   encoded_claims: request.headers["sec-istio-auth-userinfo"] | ""
*/
func TestQuota(t *testing.T) {
	cases := map[string]struct {
		attrs  map[string]interface{}
		quotas map[string]mixer.CheckRequest_QuotaParams
		want   string
	}{
		"Request 30": {
			attrs: map[string]interface{}{
				"api.service":     "service",
				"request.path":    "/path",
				"request.api_key": "key",
			},
			quotas: map[string]mixer.CheckRequest_QuotaParams{
				"key1": {
					Amount:     30,
					BestEffort: true,
				},
			},
			want: `
			 {
			  "AdapterState": null,
			  "Returns": [
			   {
			    "Quota": {
			 	"key1": {
			 	 "ValidDuration": 0,
			 	 "Amount": 1
			 	}
			    }
			   }
			  ]
			 }
			`,
		},
	}

	ts := httptest.NewServer(CloudMockHandler(t))
	defer ts.Close()

	for id, c := range cases {
		serviceCfg := adapterConfig
		serviceCfg = strings.Replace(serviceCfg, "__SERVER_BASE_URL__", ts.URL, -1)

		t.Logf("**Executing test case '%s'**", id)
		integration.RunTest(
			t,
			testGetInfo,
			integration.Scenario{
				ParallelCalls: []integration.Call{
					{
						CallKind: integration.CHECK,
						Attrs:    c.attrs,
						Quotas:   c.quotas,
					},
				},

				//Templates: map[string]template.Info{
				//	quotaT.TemplateName: quotaT,
				//},

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
