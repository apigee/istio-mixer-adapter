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

	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"istio.io/istio/mixer/pkg/adapter"
)

const (
	axPath = "/axpublisher/organization/%s/environment/%s"
)

type legacyAnalytics struct {
}

func (oa *legacyAnalytics) Start(env adapter.Env) {}
func (oa *legacyAnalytics) Close()                {}

func (oa *legacyAnalytics) SendRecords(auth *auth.Context, records []Record) error {
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

	auth.Log().Debugf("sending %d analytics records to: %s", len(records), axURL.String())

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	buf := bytes.NewBuffer(make([]byte, 0, resp.ContentLength))
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return err
	}
	respBody := buf.Bytes()

	switch resp.StatusCode {
	case 200:
		auth.Log().Debugf("analytics accepted: %v", string(respBody))
		return nil
	default:
		return fmt.Errorf("analytics rejected. status: %d, body: %s", resp.StatusCode, string(respBody))
	}
}

func buildRequest(auth *auth.Context, records []Record) (*legacyRequest, error) {
	if auth == nil || len(records) == 0 {
		return nil, nil
	}
	if auth.Organization() == "" || auth.Environment() == "" {
		return nil, fmt.Errorf("organization and environment are required in auth: %v", auth)
	}

	EnsureFields(auth, records)

	return &legacyRequest{
		Organization: auth.Organization(),
		Environment:  auth.Environment(),
		Records:      records,
	}, nil
}

type legacyRequest struct {
	Organization string   `json:"organization"`
	Environment  string   `json:"environment"`
	Records      []Record `json:"records"`
}

type legacyResponse struct {
	Accepted int `json:"accepted"`
	Rejected int `json:"rejected"`
}
