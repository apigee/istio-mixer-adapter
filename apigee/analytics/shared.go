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
	"fmt"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
)

func buildRequest(auth *auth.Context, records []Record) (*request, error) {
	if auth == nil || len(records) == 0 {
		return nil, nil
	}
	if auth.Organization() == "" || auth.Environment() == "" {
		return nil, fmt.Errorf("organization and environment are required in auth: %v", auth)
	}

	for i := range records {
		records[i].RecordType = axRecordType

		// populate from auth context
		records[i].DeveloperEmail = auth.DeveloperEmail
		records[i].DeveloperApp = auth.Application
		records[i].AccessToken = auth.AccessToken
		records[i].ClientID = auth.ClientID

		// todo: select best APIProduct based on path, otherwise arbitrary
		if len(auth.APIProducts) > 0 {
			records[i].APIProduct = auth.APIProducts[0]
		}
	}

	return &request{
		Organization: auth.Organization(),
		Environment:  auth.Environment(),
		Records:      records,
	}, nil
}
