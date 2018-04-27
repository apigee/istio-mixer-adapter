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

package auth

import (
	"encoding/json"
	"log"
	"path"
	"sync"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/context"
	"github.com/lestrrat/go-jwx/jwk"
	"github.com/lestrrat/go-jwx/jws"
	"github.com/lestrrat/go-jwx/jwt"
	"github.com/pkg/errors"
	"istio.io/istio/mixer/pkg/adapter"
)

const (
	jwksPath       = "/jwkPublicKeys"
	pollInterval   = 5 * time.Minute
	acceptableSkew = 10 * time.Second
)

func newJWTManager() *jwtManager {
	return &jwtManager{
		closedChan: make(chan bool),
		jwkSets:    sync.Map{},
	}
}

// An jwtManager handles all of the various JWT authentication functionality.
type jwtManager struct {
	closedChan chan bool
	jwkSets    sync.Map
}

func (a *jwtManager) pollingLoop() {
	tick := time.Tick(pollInterval)
	for {
		select {
		case <-a.closedChan:
			return
		case <-tick:
			if err := a.refresh(); err != nil {
				log.Printf("Error refreshing auth manager: %s", err)
			}
		}
	}
}

func (a *jwtManager) start(env adapter.Env) {
	if err := a.refresh(); err != nil {
		log.Printf("Error refreshing auth manager: %s", err)
	}
	env.ScheduleDaemon(func() {
		a.pollingLoop()
	})
}

func (a *jwtManager) stop() {
	if a != nil {
		a.closedChan <- true
	}
}

func (a *jwtManager) ensureSet(url string) error {
	set, err := jwk.FetchHTTP(url)
	if err != nil {
		return err
	}
	a.jwkSets.Store(url, set)
	return nil
}

func (a *jwtManager) refresh() error {
	var errRet error
	a.jwkSets.Range(func(urlI interface{}, setI interface{}) bool {
		if err := a.ensureSet(urlI.(string)); err != nil {
			errRet = err
		}
		return true
	})
	return errRet
}

func (a *jwtManager) jwkSet(ctx context.Context) (*jwk.Set, error) {
	jwksURL := *ctx.CustomerBase()
	jwksURL.Path = path.Join(jwksURL.Path, jwksPath)
	url := jwksURL.String()
	if _, ok := a.jwkSets.Load(url); !ok {
		if err := a.ensureSet(url); err != nil {
			return nil, err
		}
	}
	set, _ := a.jwkSets.Load(url)
	return set.(*jwk.Set), nil
}

func (a *jwtManager) verifyJWT(ctx context.Context, raw string) (map[string]interface{}, error) {
	set, err := a.jwkSet(ctx)
	if err != nil {
		return nil, err
	}

	// verify against public keys
	_, err = jws.VerifyWithJWKSet([]byte(raw), set, nil)
	if err != nil {
		return nil, err
	}

	// verify fields
	token, err := jwt.ParseString(raw)
	if err != nil {
		return nil, errors.Wrap(err, "invalid jws message")
	}
	err = token.Verify(jwt.WithAcceptableSkew(acceptableSkew))
	if err != nil {
		return nil, errors.Wrap(err, "invalid jws message")
	}

	// get claims
	m, err := jws.ParseString(raw)
	if err != nil {
		return nil, errors.Wrap(err, "invalid jws message")
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(m.Payload(), &claims); err != nil {
		return nil, errors.Wrap(err, "failed to parse claims")
	}

	return claims, nil
}
