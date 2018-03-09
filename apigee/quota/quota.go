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

package quota

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/product"
	"istio.io/istio/mixer/pkg/adapter"
)

// todo: support args.DeduplicationID

const quotaPath = "/quotas/organization/%s/environment/%s"

// todo: async
func Apply(auth auth.Context, p product.APIProduct, args adapter.QuotaArgs) (QuotaResult, error) {

	quotaURL := auth.ApigeeBase()
	quotaURL.Path = path.Join(quotaURL.Path, fmt.Sprintf(quotaPath, auth.Organization(), auth.Environment()))

	allow, err := strconv.ParseInt(p.QuotaLimit, 10, 64)
	if err != nil {
		return QuotaResult{}, err
	}

	interval, err := strconv.ParseInt(p.QuotaInterval, 10, 64)
	if err != nil {
		return QuotaResult{}, err
	}

	quotaID := fmt.Sprintf("%s-%s", auth.Application, p.Name)
	request := QuotaRequest{
		Identifier: quotaID,
		Weight:     args.QuotaAmount,
		Interval:   interval,
		Allow:      allow,
		TimeUnit:   p.QuotaTimeUnit,
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(request)

	req, err := http.NewRequest(http.MethodPost, quotaURL.String(), body)
	if err != nil {
		return QuotaResult{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	auth.Log().Infof("Sending to (%s): %s\n", quotaURL.String(), body)

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return QuotaResult{}, err
	}
	defer resp.Body.Close()

	buf := bytes.NewBuffer(make([]byte, 0, resp.ContentLength))
	_, err = buf.ReadFrom(resp.Body)
	respBody := buf.Bytes()

	switch resp.StatusCode {
	case 200:
		var quotaResult QuotaResult
		if err = json.Unmarshal(respBody, &quotaResult); err != nil {
			err = auth.Log().Errorf("Error unmarshalling: %s\n", string(respBody))
		}
		return quotaResult, err

	default:
		return QuotaResult{}, auth.Log().Errorf("quota apply failed. result: %s\n", string(respBody))
	}
}

type ErrorResponse struct {
	ErrorCode string `json:"errorCode"`
	Reason    string `json:"reason"`
}
