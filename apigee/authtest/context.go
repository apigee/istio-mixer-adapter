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

package authtest

import (
	"fmt"
	"net/url"
)

// Context implements the context.Context interface and is to be used in tests.
type Context struct {
	apigeeBase   *url.URL
	customerBase *url.URL
	orgName      string
	envName      string
	key          string
	secret       string
}

// NewContext constructs a new test context.
func NewContext(base string) *Context {
	u, err := url.Parse(base)
	if err != nil {
		panic(fmt.Sprintf("Could not parse URL: %s", base))
	}
	return &Context{
		apigeeBase:   u,
		customerBase: u,
	}
}

// ApigeeBase gets a URL base to send HTTP requests to.
func (c *Context) ApigeeBase() *url.URL { return c.apigeeBase }

// CustomerBase gets a URL base to send HTTP requests to.
func (c *Context) CustomerBase() *url.URL { return c.customerBase }

// Organization gets this context's organization.
func (c *Context) Organization() string { return c.orgName }

// Environment gets this context's environment.
func (c *Context) Environment() string { return c.envName }

// Key gets this context's API key.
func (c *Context) Key() string { return c.key }

// Secret gets this context's API secret.
func (c *Context) Secret() string { return c.secret }

// SetOrganization sets this context's organization.
func (c *Context) SetOrganization(o string) { c.orgName = o }

// SetEnvironment sets this context's environment.
func (c *Context) SetEnvironment(e string) { c.envName = e }
