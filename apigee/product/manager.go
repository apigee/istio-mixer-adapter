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
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"sync/atomic"
	"time"

	"istio.io/istio/mixer/pkg/adapter"
)

const productsURL = "/products"

var pollInterval = 2 * time.Minute

/*
Usage:
	pp := createProductManager()
	pp.start()
	products := pp.getProducts()
	...
	pp.close() // when done
*/

func createProductManager(baseURL url.URL, log adapter.Logger) *productManager {
	isClosedInt := int32(0)

	return &productManager{
		baseURL:         baseURL,
		log:             log,
		products:        map[string]Details{},
		quitPollingChan: make(chan bool, 1),
		closedChan:      make(chan bool),
		getProductsChan: make(chan bool),
		returnChan:      make(chan map[string]Details),
		updatedChan:     make(chan bool, 1),
		isClosed:        &isClosedInt,
	}
}

type productManager struct {
	baseURL          url.URL
	log              adapter.Logger
	products         map[string]Details
	isClosed         *int32
	quitPollingChan  chan bool
	closedChan       chan bool
	getProductsChan  chan bool
	returnChan       chan map[string]Details
	updatedChan      chan bool
	refreshTimerChan <-chan time.Time
}

func (p *productManager) start(env adapter.Env) {
	p.retrieve()
	//go p.pollingLoop()
	env.ScheduleDaemon(func() {
		p.pollingLoop()
	})
}

// returns name => Details
func (p *productManager) getProducts() map[string]Details {
	p.log.Errorf("getProducts()")
	if atomic.LoadInt32(p.isClosed) == int32(1) {
		return nil
	}
	p.getProductsChan <- true
	return <-p.returnChan
}

func (p *productManager) pollingLoop() {
	tick := time.Tick(pollInterval)
	for {
		select {
		case <-p.closedChan:
			p.log.Errorf("closedChan")
			return
		case <-p.getProductsChan:
			p.log.Errorf("getProductsChan")
			p.returnChan <- p.products
		case <-tick:
			p.log.Errorf("tick")
			p.retrieve()
		}
	}
}

func (p *productManager) close() {
	if atomic.SwapInt32(p.isClosed, 1) == int32(1) {
		return
	}
	p.quitPollingChan <- true
	p.closedChan <- true
	close(p.closedChan)
}

// don't call externally. will block until success.
func (p *productManager) retrieve() {
	apiURL := p.baseURL
	apiURL.Path = path.Join(apiURL.Path, productsURL)

	p.pollWithBackoff(p.quitPollingChan, p.getPollingClosure(apiURL), func(err error) {
		p.log.Errorf("Error retrieving products: %v", err)
	})
}

func (p *productManager) getPollingClosure(apiURL url.URL) func(chan bool) error {
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
		resp.Body.Close()
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

func (p *productManager) getTokenReadyChannel() <-chan bool {
	return p.updatedChan
}

func (p *productManager) pollWithBackoff(quit chan bool, toExecute func(chan bool) error, handleError func(error)) {

	backoff := NewExponentialBackoff(200*time.Millisecond, pollInterval, 2, true)
	retry := time.After(0 * time.Millisecond) // start first attempt immediately

	for {
		select {
		case <-quit:
			p.log.Infof("quit signal, returning")
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
