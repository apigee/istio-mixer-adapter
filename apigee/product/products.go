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
	"net/url"
	"regexp"
	"strings"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"istio.io/istio/mixer/pkg/adapter"
)

const servicesAttr = "istio-services"

var pm *productManager

func Start(baseURL url.URL, log adapter.Logger, env adapter.Env) {
	pm = createProductManager(baseURL, log)
	pm.start(env)
}

func Stop() {
	pm.close()
	pm = nil
}

func Resolve(ac auth.Context, api, path string) []APIProduct {
	validProducts := resolve(pm.getProducts(), ac.APIProducts, ac.Scopes, api, path)
	ac.Log().Infof("Resolve api: %s, path: %s, scopes: %v => %v", api, path, ac.Scopes, validProducts)
	return validProducts
}

// todo: naive impl, optimize
func resolve(pMap map[string]APIProduct, products, scopes []string, api, path string) (result []APIProduct) {

	for _, name := range products {
		apiProduct := pMap[name]
		if apiProduct.isValidScopes(scopes) {
			for _, attr := range apiProduct.Attributes {
				if attr.Name == servicesAttr {
					targets := strings.Split(attr.Value, ",")
					for _, target := range targets { // find target paths
						if strings.TrimSpace(target) == api {
							if apiProduct.isValidPath(path) {
								result = append(result, apiProduct)
							}
						}
					}
				}
			}
		}
	}
	return result
}

// true if valid path for API Product
func (d *APIProduct) isValidPath(requestPath string) bool {
	for _, resource := range d.Resources {
		reg, err := makeResourceRegex(resource)
		if err == nil && reg.MatchString(requestPath) {
			return true
		}
	}
	return false
}

// true if any intersect of scopes
func (d *APIProduct) isValidScopes(scopes []string) bool {
	for _, ds := range d.Scopes {
		for _, s := range scopes {
			if ds == s {
				return true
			}
		}
	}
	return false
}

// - A single slash by itself matches any path
// - * is valid anywhere and matches within a segment (between slashes)
// - ** is valid only at the end and matches anything to EOL
func makeResourceRegex(resource string) (*regexp.Regexp, error) {

	if resource == "/" {
		return regexp.Compile(".*")
	}

	// only allow ** as suffix
	doubleStarIndex := strings.Index(resource, "**")
	if doubleStarIndex >= 0 && doubleStarIndex != len(resource)-2 {
		return nil, fmt.Errorf("bad resource specification")
	}

	// remove ** suffix if exists
	pattern := resource
	if doubleStarIndex >= 0 {
		pattern = pattern[:len(pattern)-2]
	}

	// let * = any non-slash
	pattern = strings.Replace(pattern, "*", "[^/]*", -1)

	// if ** suffix, allow anything at end
	if doubleStarIndex >= 0 {
		pattern = pattern + ".*"
	}

	return regexp.Compile("^" + pattern + "$")
}
