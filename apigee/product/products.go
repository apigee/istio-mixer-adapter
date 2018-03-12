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

import (
	"net/url"

	"istio.io/istio/mixer/pkg/adapter"
)

const servicesAttr = "istio-services"

// NewManager creates a new product.Manager. Call Close() when done.
func NewManager(baseURL url.URL, log adapter.Logger, env adapter.Env) *Manager {
	pm := createManager(baseURL, log)
	pm.start(env)
	return pm
}
