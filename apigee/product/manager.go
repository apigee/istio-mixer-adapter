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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"istio.io/istio/mixer/pkg/adapter"
)

const productsURL = "/products"

var pollInterval = 2 * time.Minute

/*
Usage:
	pp := createManager()
	pp.start()
	products := pp.Products()
	...
	pp.close() // when done
*/

func createManager(baseURL url.URL, log adapter.Logger) *Manager {
	isClosedInt := int32(0)

	return &Manager{
		baseURL:         baseURL,
		log:             log,
		products:        map[string]APIProduct{},
		quitPollingChan: make(chan bool, 1),
		closedChan:      make(chan bool),
		getProductsChan: make(chan bool),
		returnChan:      make(chan map[string]APIProduct),
		updatedChan:     make(chan bool, 1),
		isClosed:        &isClosedInt,
	}
}

// A Manager wraps all things related to a set of API products.
type Manager struct {
	baseURL          url.URL
	log              adapter.Logger
	products         map[string]APIProduct
	isClosed         *int32
	quitPollingChan  chan bool
	closedChan       chan bool
	getProductsChan  chan bool
	returnChan       chan map[string]APIProduct
	updatedChan      chan bool
	refreshTimerChan <-chan time.Time
}

func (p *Manager) start(env adapter.Env) {
	p.retrieve()
	//go p.pollingLoop()
	env.ScheduleDaemon(func() {
		p.pollingLoop()
	})
}

// Products atomically gets a mapping of name => APIProduct.
func (p *Manager) Products() map[string]APIProduct {
	if atomic.LoadInt32(p.isClosed) == int32(1) {
		return nil
	}
	p.getProductsChan <- true
	return <-p.returnChan
}

func (p *Manager) pollingLoop() {
	tick := time.Tick(pollInterval)
	for {
		select {
		case <-p.closedChan:
			return
		case <-p.getProductsChan:
			p.returnChan <- p.products
		case <-tick:
			p.retrieve()
		}
	}
}

// Close shuts down the manager.
func (p *Manager) Close() {
	if p == nil || atomic.SwapInt32(p.isClosed, 1) == int32(1) {
		return
	}
	p.quitPollingChan <- true
	p.closedChan <- true
	close(p.closedChan)
}

// don't call externally. will block until success.
func (p *Manager) retrieve() {
	apiURL := p.baseURL
	apiURL.Path = path.Join(apiURL.Path, productsURL)

	p.pollWithBackoff(p.quitPollingChan, p.pollingClosure(apiURL), func(err error) {
		p.log.Errorf("Error retrieving products: %v", err)
	})
}

func (p *Manager) pollingClosure(apiURL url.URL) func(chan bool) error {
	return func(_ chan bool) error {
		log := p.log

		req, err := http.NewRequest(http.MethodGet, apiURL.String(), nil)
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		log.Infof("retrieving products from: %s", apiURL.String())

		client := http.DefaultClient
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Errorf("Unable to read server response: %v", err)
			return err
		}

		if resp.StatusCode != 200 {
			return log.Errorf("products request failed (%d): %s", resp.StatusCode, string(body))
		}

		var res apiResponse
		err = json.Unmarshal(body, &res)
		if err != nil {
			log.Errorf("unable to unmarshal JSON response '%s': %v", string(body), err)
			return err
		}

		// reformat to map
		for _, product := range res.APIProducts {
			p.products[product.Name] = product
		}

		// don't block, default means there is existing signal
		select {
		case p.updatedChan <- true:
		default:
		}

		return nil
	}
}

func (p *Manager) getTokenReadyChannel() <-chan bool {
	return p.updatedChan
}

func (p *Manager) pollWithBackoff(quit chan bool, toExecute func(chan bool) error, handleError func(error)) {

	backoff := NewExponentialBackoff(200*time.Millisecond, pollInterval, 2, true)
	retry := time.After(0 * time.Millisecond) // start first attempt immediately

	for {
		select {
		case <-quit:
			return
		case <-retry:
			err := toExecute(quit)
			if err == nil {
				return
			}

			if _, ok := err.(quitSignalError); ok {
				return
			}
			handleError(err)

			retry = time.After(backoff.Duration())
		}
	}
}

type quitSignalError error

// Resolve determines the valid products for a given API.
func (p *Manager) Resolve(ac auth.Context, api, path string) []APIProduct {
	validProducts := resolve(p.Products(), ac.APIProducts, ac.Scopes, api, path)
	ac.Log().Infof("Resolve api: %s, path: %s, scopes: %v => %v", api, path, ac.Scopes, validProducts)
	return validProducts
}

// todo: naive impl, optimize
func resolve(pMap map[string]APIProduct, products, scopes []string, api, path string) (result []APIProduct) {

	for _, name := range products {
		apiProduct := pMap[name]
		if !apiProduct.isValidScopes(scopes) {
			continue
		}

		for _, attr := range apiProduct.Attributes {
			if attr.Name != servicesAttr {
				continue
			}

			targets := strings.Split(attr.Value, ",")
			for _, target := range targets { // find target paths
				if strings.TrimSpace(target) != api {
					continue
				}

				if !apiProduct.isValidPath(path) {
					continue
				}

				result = append(result, apiProduct)
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
