package auth

import (
	"errors"
	"fmt"

	"github.com/dgrijalva/jwt-go"
	"github.com/lestrrat/go-jwx/jwk"
	"github.com/apigee/istio-mixer-adapter/apigee/context"
	"path"
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

	// TODO: cache response

	jwksURL := ctx.CustomerBase()
	jwksURL.Path = path.Join(jwksURL.Path, jwksPath)

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
