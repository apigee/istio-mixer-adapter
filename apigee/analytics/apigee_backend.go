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

package analytics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"istio.io/istio/mixer/pkg/adapter"
)

const (
	axPath       = "/axpublisher/organization/%s/environment/%s"
	axRecordType = "APIAnalytics"
)

type apigeeBackend struct {
}

type errorResponse struct {
	ErrorCode string `json:"errorCode"`
	Reason    string `json:"reason"`
}

func (ab *apigeeBackend) Start(env adapter.Env) {}
func (ab *apigeeBackend) Stop()                 {}

// todo: select best APIProduct based on path, otherwise arbitrary
func (ab *apigeeBackend) SendRecords(auth *auth.Context, records []Record) error {
	axURL := auth.ApigeeBase()
	axURL.Path = path.Join(axURL.Path, fmt.Sprintf(axPath, auth.Organization(), auth.Environment()))

	request, err := buildRequest(auth, records)
	if request == nil || err != nil {
		return err
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(request)

	req, err := http.NewRequest(http.MethodPost, axURL.String(), body)
	if err != nil {
		return err
	}

	req.SetBasicAuth(auth.Key(), auth.Secret())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	auth.Log().Infof("Sending to (%s): %s", axURL.String(), body)

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	buf := bytes.NewBuffer(make([]byte, 0, resp.ContentLength))
	_, err = buf.ReadFrom(resp.Body)
	respBody := buf.Bytes()

	switch resp.StatusCode {
	case 200:
		auth.Log().Infof("analytics accepted: %v", string(respBody))
		return nil
	default:
		var errorResponse errorResponse
		if err = json.Unmarshal(respBody, &errorResponse); err != nil {
			auth.Log().Infof("analytics unmarshal error: %d, body: %v", resp.StatusCode, string(respBody))
			return err
		}
		auth.Log().Errorf("analytics not sent. reason: %s, code: %s\n",
			errorResponse.Reason, errorResponse.ErrorCode)
		return fmt.Errorf("analytics not sent. reason: %s, code: %s",
			errorResponse.Reason, errorResponse.ErrorCode)
	}
}
