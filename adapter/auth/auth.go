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
func (m *Manager) Close() {
	if m != nil {
		m.jwtMan.stop()
	}
}

// Authenticate constructs an Apigee context from an existing context and either
// a set of JWT claims, or an Apigee API key.
// The following logic applies:
// 1. If JWT w/ API Key - use API Key in claims
// 2. API Key - use API Key
// 3. Has JWT token - use JWT claims
// If any method is provided but fails, the next available one(s) will be attempted. If all provided methods fail,
// the request will be rejected.
func (m *Manager) Authenticate(ctx context.Context, apiKey string,
	claims map[string]interface{}, apiKeyClaimKey string) (*Context, error) {
	log := ctx.Log()

	redacts := []interface{}{
		claims["access_token"],
		claims["client_id"],
		claims[apiKeyClaimKey],
	}
	redactedClaims := util.SprintfRedacts(redacts, "%#v", claims)
	log.Debugf("Authenticate: key: %v, claims: %v", util.Truncate(apiKey, 5), redactedClaims)

	var authContext = &Context{Context: ctx}

	// use API Key in JWT if available
	var err error
	var verifiedClaims map[string]interface{}
	if claims[apiKeyClaimKey] != nil {
		if apiKey, ok := claims[apiKeyClaimKey].(string); ok {
			verifiedClaims, err = m.verifier.Verify(ctx, apiKey)
			if err == nil {
				log.Debugf("using api key from jwt claim %s", apiKeyClaimKey)
				authContext.APIKey = apiKey
			}
		}
	}

	// else, use API Key if available
	if verifiedClaims == nil && apiKey != "" {
		verifiedClaims, err = m.verifier.Verify(ctx, apiKey)
		if err == nil {
			log.Debugf("using api key from request")
			authContext.APIKey = apiKey
		}
	}

	// else, use JWT claims
	if verifiedClaims == nil && claims != nil {
		verifiedClaims = claims
	}

	err = authContext.setClaims(verifiedClaims)

	if log.DebugEnabled() {
		redacts = []interface{}{authContext.APIKey, authContext.AccessToken, authContext.ClientID}
		redactedAC := util.SprintfRedacts(redacts, "%v", authContext)
		if err == nil {
			log.Debugf("Authenticate success: %s", redactedAC)
		} else {
			log.Debugf("Authenticate error: %s [%v]", redactedAC, err)
		}
	}

	if verifiedClaims[apiProductListClaim] == nil {
		err = &NoAuthInfoError{}
	}

	return authContext, err
}

func (m *Manager) start() {
	m.jwtMan.start(m.env)
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
