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
	"fmt"
	"net/http"
	"net/url"
	"time"

	"istio.io/istio/mixer/pkg/adapter"
)

// ServicesAttr is the name of the Product attribute that lists the Istio services it binds to (comma delim)
const ServicesAttr = "istio-services"

// NewManager creates a new product.Manager. Call Close() when done.
func NewManager(env adapter.Env, options Options) (*Manager, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}
	pm := createManager(options, env.Logger())
	pm.start(env)
	return pm, nil
}

// Options allows us to specify options for how this product manager will run.
type Options struct {
	// Client is a configured HTTPClient
	Client *http.Client
	// BaseURL of the Apigee customer proxy
	BaseURL *url.URL
	// RefreshRate determines how often the products are refreshed
	RefreshRate time.Duration
}

func (o *Options) validate() error {
	if o.Client == nil ||
		o.BaseURL == nil ||
		o.RefreshRate <= 0 {
		return fmt.Errorf("all products options are required")
	}
	return nil
}
