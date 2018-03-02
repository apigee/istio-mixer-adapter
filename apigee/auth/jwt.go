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

	"path"

	"github.com/apigee/istio-mixer-adapter/apigee/context"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/lestrrat/go-jwx/jwk"
)

const jwksPath = "/jwkPublicKeys"

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
	jwksURL := ctx.CustomerBase()
	jwksURL.Path = path.Join(jwksURL.Path, jwksPath)

	// TODO(robbrit): periodically cache instead of pulling the set each time.
	set, err := jwk.FetchHTTP(jwksURL.String())
	if err != nil {
		return nil, err
	}

	keyID, ok := token.Header["kid"].(string)
	if !ok {
		return nil, errors.New("JWT header missing kid")
	}

	if key := set.LookupKeyID(keyID); len(key) == 1 {
		return key[0].Materialize()
	}

	return nil, fmt.Errorf("jwks doesn't contain key: %s", keyID)
}
