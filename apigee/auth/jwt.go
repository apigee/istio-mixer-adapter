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
	"errors"
	"fmt"
	"log"
	"path"
	"sync"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/context"
	"github.com/dgrijalva/jwt-go"
	"github.com/lestrrat/go-jwx/jwk"
	"istio.io/istio/mixer/pkg/adapter"
)

const (
	jwksPath     = "/jwkPublicKeys"
	pollInterval = 5 * time.Minute
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

func (a *jwtManager) jwtKey(ctx context.Context, token *jwt.Token) (interface{}, error) {
	jwksURL := ctx.CustomerBase()
	jwksURL.Path = path.Join(jwksURL.Path, jwksPath)

	keyID, ok := token.Header["kid"].(string)
	if !ok {
		return nil, errors.New("JWT header missing kid")
	}

	url := jwksURL.String()
	if _, ok := a.jwkSets.Load(url); !ok {
		if err := a.ensureSet(url); err != nil {
			return nil, err
		}
	}
	set, _ := a.jwkSets.Load(url)

	if key := set.(*jwk.Set).LookupKeyID(keyID); len(key) == 1 {
		return key[0].Materialize()
	}

	return nil, fmt.Errorf("jwks doesn't contain key: %s", keyID)
}

func (a *jwtManager) verifyJWT(ctx context.Context, raw string) (jwt.MapClaims, error) {
	ctx.Log().Infof("verifyJWT: %v", raw)
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		return a.getJWTKey(ctx, token)
	}
	token, err := jwt.Parse(raw, keyFunc)
	if err != nil {
		return nil, fmt.Errorf("jwt.Parse(): %s", err)
	}
	return token.Claims.(jwt.MapClaims), nil
}

func (a *jwtManager) getJWTKey(ctx context.Context, token *jwt.Token) (interface{}, error) {
	if a == nil {
		return nil, fmt.Errorf("auth manager not initialized")
	}
	return a.jwtKey(ctx, token)
}
