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
	"bytes"
	"encoding/json"
	"net/http"
	"github.com/apigee/istio-mixer-adapter/apigee/context"
	"path"
)

const verifyApiKeyURL = "/verifyApiKey"

// returns claims or error
// todo: need a better "failed" error
func VerifyAPIKey(ctx context.Context, apiKey string) (map[string]interface{}, error) {
	ctx.Log().Infof("VerifyAPIKey: %v", apiKey)

	verifyRequest := ApiKeyRequest{
		ApiKey: apiKey,
	}

	apiURL := ctx.CustomerBase()
	apiURL.Path = path.Join(apiURL.Path, verifyApiKeyURL)

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(verifyRequest)

	req, err := http.NewRequest(http.MethodPost, apiURL.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	//res, err := ioutil.ReadAll(resp.Body)
	//ctx.Log().Infof("VerifyAPIKey response: %v", string(res))
	//apiKeyResp := ApiKeyResponse{}
	//json.Unmarshal(res, &apiKeyResp)

	apiKeyResp := ApiKeyResponse{}
	json.NewDecoder(resp.Body).Decode(&apiKeyResp)

	return verifyJWT(ctx, apiKeyResp.Token)
}
