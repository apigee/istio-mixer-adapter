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

package product

type apiResponse struct {
	APIProducts []Details `json:"apiProducts"`
}

type Details struct {
	Attributes     []Attribute `json:"attributes,omitempty"`
	CreatedAt      string      `json:"createdAt,omitempty"`
	CreatedBy      string      `json:"createdBy,omitempty"`
	Description    string      `json:"description,omitempty"`
	DisplayName    string      `json:"displayName,omitempty"`
	Environments   []string    `json:"environments,omitempty"`
	LastModifiedAt string      `json:"lastModifiedAt,omitempty"`
	LastModifiedBy string      `json:"lastModifiedBy,omitempty"`
	Name           string      `json:"name,omitempty"`
	QuotaLimit     string      `json:"quota,omitempty"`
	QuotaInterval  int64       `json:"quotaInterval,omitempty"`
	QuotaTimeUnit  string      `json:"quotaTimeUnit,omitempty"`
	Resources      []string    `json:"apiResources"`
	Scopes         []string    `json:"scopes"`
}

type Attribute struct {
	Kind  string `json:"kind,omitempty"`
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}
