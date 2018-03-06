package auth

// This file defines the primary entry point for the auth module, which is the
// Authenticate function.

import (
	"fmt"

	"github.com/apigee/istio-mixer-adapter/apigee/context"
)

var verifier = newVerifier()

// Authenticate constructs an Apigee context from an existing context and either
// a set of JWT claims, or an Apigee API key.
func Authenticate(ctx context.Context, apiKey string, claims map[string]interface{}) (Context, error) {

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

	claims, err := verifier.Verify(ctx, apiKey)
	if err != nil {
		return ac, err
	}

	err = ac.setClaims(claims)

	ctx.Log().Infof("Authenticate complete: %v [%v]", ac, err)
	return ac, err
}
