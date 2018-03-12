package auth

// This file defines the primary entry point for the auth module, which is the
// Authenticate function.

import (
	"fmt"

	"github.com/apigee/istio-mixer-adapter/apigee/context"
	"istio.io/istio/mixer/pkg/adapter"
)

// Start begins the update loop for the auth manager, which will periodically
// refresh JWT credentials.
// call Close() when done
func NewAuthManager(env adapter.Env) *AuthManager {
	jwtMan := newJWTManager()
	// TODO(robbrit): allow options to be configurable.
	v := newVerifier(jwtMan, keyVerifierOpts{})
	am := &AuthManager{
		env:      env,
		jwtMan:   jwtMan,
		verifier: v,
	}
	am.start()
	return am
}

type AuthManager struct {
	env      adapter.Env
	jwtMan   *jwtManager
	verifier keyVerifier
}

func (a *AuthManager) Close() {
	if a != nil {
		a.jwtMan.stop()
	}
}

// Authenticate constructs an Apigee context from an existing context and either
// a set of JWT claims, or an Apigee API key.
func (a *AuthManager) Authenticate(ctx context.Context, apiKey string, claims map[string]interface{}) (Context, error) {

	ctx.Log().Infof("Authenticate: key: %v, claims: %v", apiKey, claims)

	var ac = Context{Context: ctx}
	if claims != nil {
		err := ac.setClaims(claims)
		if ac.ClientID != "" || err != nil {
			return ac, err
		}
	}

	if apiKey == "" {
		return ac, fmt.Errorf("missing api key")
	}

	claims, err := a.verifier.Verify(ctx, apiKey)
	if err != nil {
		return ac, err
	}

	err = ac.setClaims(claims)

	ctx.Log().Infof("Authenticate complete: %v [%v]", ac, err)
	return ac, err
}

func (a *AuthManager) start() {
	a.jwtMan.start(a.env)
}
