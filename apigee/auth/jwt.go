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
	"sync"
	"time"

	"path"

	"github.com/apigee/istio-mixer-adapter/apigee/context"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/lestrrat/go-jwx/jwk"
	"istio.io/istio/mixer/pkg/adapter"
)

const (
	jwksPath     = "/jwkPublicKeys"
	pollInterval = 5 * time.Minute
)

var am = createAuthManager()

// Start begins the update loop for the auth manager, which will periodically
// refresh JWT credentials.
func Start(env adapter.Env) {
	am.start(env)
}

// Stop stops the auth manager's update loop.
func Stop() {
	am.close()
}

type authManager struct {
	closedChan chan bool
	jwkSets    sync.Map
}

func createAuthManager() *authManager {
	return &authManager{
		closedChan: make(chan bool),
		jwkSets:    sync.Map{},
	}
}

func (a *authManager) pollingLoop() {
	tick := time.Tick(pollInterval)
	for {
		select {
		case <-a.closedChan:
			return
		case <-tick:
			a.refresh()
		}
	}
}

func (a *authManager) start(env adapter.Env) {
	a.refresh()
	env.ScheduleDaemon(func() {
		a.pollingLoop()
	})
}

func (a *authManager) close() {
	a.closedChan <- true
}

func (a *authManager) ensureSet(url string) error {
	set, err := jwk.FetchHTTP(url)
	if err != nil {
		return err
	}
	a.jwkSets.Store(url, set)
	return nil
}

func (a *authManager) refresh() {
	a.jwkSets.Range(func(urlI interface{}, setI interface{}) bool {
		if err := a.ensureSet(urlI.(string)); err != nil {
			log.Printf("Error updating jwks set: %s", err)
		}
		return true
	})
}

func (a *authManager) jwtKey(ctx context.Context, token *jwt.Token) (interface{}, error) {
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

func verifyJWT(ctx context.Context, raw string) (jwt.MapClaims, error) {
	ctx.Log().Infof("verifyJWT: %v", raw)
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		return getJWTKey(ctx, token)
	}
	token, err := jwt.Parse(raw, keyFunc)
	if err != nil {
		return nil, err
	}
	claims := token.Claims.(jwt.MapClaims)
	ctx.Log().Infof("claims: %v", claims)
	return claims, nil
}

func getJWTKey(ctx context.Context, token *jwt.Token) (interface{}, error) {
	if am == nil {
		return nil, fmt.Errorf("auth manager not initialized")
	}
	return am.jwtKey(ctx, token)
}
