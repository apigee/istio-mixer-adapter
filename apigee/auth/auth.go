package auth

// This file defines the primary entry point for the auth module, which is the
// Authenticate function.

import (
	"github.com/apigee/istio-mixer-adapter/apigee/context"
	"github.com/apigee/istio-mixer-adapter/apigee/util"
	"istio.io/istio/mixer/pkg/adapter"
)

// NewManager constructs a new Manager and begins the update loop, which
// will periodically refresh JWT credentials. Call Close() when done.
func NewManager(env adapter.Env) *Manager {
	jwtMan := newJWTManager()
	// TODO(robbrit): allow options to be configurable.
	v := newVerifier(jwtMan, keyVerifierOpts{})
	am := &Manager{
		env:      env,
		jwtMan:   jwtMan,
		verifier: v,
	}
	am.start()
	return am
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
	ctx.Log().Infof("Authenticate: key: %v, claims: %v", util.Truncate(apiKey, 5), redactedClaims)

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

	claims, err := a.verifier.Verify(ctx, apiKey)
	if err != nil {
		return ac, err
	}

	err = ac.setClaims(claims)

	redacts = []interface{}{ac.AccessToken, ac.ClientID}
	redactedAC := util.SprintfRedacts(redacts, "%v", ac)
	if err == nil {
		ctx.Log().Infof("Authenticate success: %s", redactedAC)
	} else {
		ctx.Log().Infof("Authenticate error: %s [%v]", redactedAC, err)
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
