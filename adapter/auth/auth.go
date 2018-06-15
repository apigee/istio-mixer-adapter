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

// This file defines the primary entry point for the auth module, which is the
// Authenticate function.

import (
	"fmt"
	"net/http"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/context"
	"github.com/apigee/istio-mixer-adapter/adapter/util"
	"istio.io/istio/mixer/pkg/adapter"
)

// NewManager constructs a new Manager and begins an update loop to
// periodically refresh JWT credentials if options.pollInterval > 0.
// Call Close() when done.
func NewManager(env adapter.Env, options Options) (*Manager, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}
	jwtMan := newJWTManager(options.PollInterval)
	v := newVerifier(jwtMan, keyVerifierOpts{
		Client: options.Client,
	})
	am := &Manager{
		env:      env,
		jwtMan:   jwtMan,
		verifier: v,
	}
	am.start()
	return am, nil
}

// An Manager handles all things related to authentication.
type Manager struct {
	env      adapter.Env
	jwtMan   *jwtManager
	verifier keyVerifier
}

// Close shuts down the Manager.
func (a *Manager) Close() {
	if a != nil {
		a.jwtMan.stop()
	}
}

// Authenticate constructs an Apigee context from an existing context and either
// a set of JWT claims, or an Apigee API key.
func (a *Manager) Authenticate(ctx context.Context, apiKey string, claims map[string]interface{}) (*Context, error) {
	redacts := []interface{}{
		claims["access_token"],
		claims["client_id"],
	}
	redactedClaims := util.SprintfRedacts(redacts, "%#v", claims)
	ctx.Log().Debugf("Authenticate: key: %v, claims: %v", util.Truncate(apiKey, 5), redactedClaims)

	// use JWT claims directly if available
	var ac = &Context{Context: ctx}
	if claims != nil {
		err := ac.setClaims(claims)
		if ac.ClientID != "" || err != nil {
			return ac, err
		}
	}

	if apiKey == "" {
		return ac, &NoAuthInfoError{}
	}

	// use API Key if JWT claims are not available
	ac.APIKey = apiKey
	claims, err := a.verifier.Verify(ctx, apiKey)
	if err != nil {
		return ac, err
	}

	err = ac.setClaims(claims)

	redacts = []interface{}{ac.AccessToken, ac.ClientID}
	redactedAC := util.SprintfRedacts(redacts, "%v", ac)
	if err == nil {
		ctx.Log().Debugf("Authenticate success: %s", redactedAC)
	} else {
		ctx.Log().Debugf("Authenticate error: %s [%v]", redactedAC, err)
	}
	return ac, err
}

func (a *Manager) start() {
	a.jwtMan.start(a.env)
}

// NoAuthInfoError indicates that the error was because of missing auth
type NoAuthInfoError struct {
}

func (e *NoAuthInfoError) Error() string {
	return "missing authentication"
}

// Options allows us to specify options for how this auth manager will run
type Options struct {
	// PollInterval sets refresh rate of JWT credentials, disabled if = 0
	PollInterval time.Duration
	// Client is a configured HTTPClient
	Client *http.Client
}

func (o *Options) validate() error {
	if o.Client == nil {
		return fmt.Errorf("client is required")
	}
	return nil
}
