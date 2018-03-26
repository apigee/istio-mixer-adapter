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

	mixer "istio.io/api/mixer/v1"
	integration "istio.io/istio/mixer/pkg/adapter/test"
)

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
		"Good request": {
			attrs: map[string]interface{}{
				"api.service":     "service",
				"request.path":    "/path",
				"request.api_key": "goodkey",
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

	serviceCfg := strings.Replace(adapterConfigForQuota(), "__SERVER_BASE_URL__", ts.URL, -1)

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
