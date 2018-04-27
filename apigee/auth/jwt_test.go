package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/authtest"
	"github.com/dgrijalva/jwt-go"
	"github.com/lestrrat/go-jwx/jwk"
	"istio.io/istio/mixer/pkg/adapter/test"
)

func goodJWTRequest(privateKey *rsa.PrivateKey, t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key, err := jwk.New(&privateKey.PublicKey)
		if err != nil {
			t.Fatal(err)
		}
		key.Set("kid", "1")
		key.Set("alg", jwt.SigningMethodRS256.Alg())

		type JWKS struct {
			Keys []jwk.Key `json:"keys"`
		}

		jwks := JWKS{
			Keys: []jwk.Key{
				key,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}
}

func badJWTRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(401)
	json.NewEncoder(w).Encode(badKeyResponse)
}

func TestJWTCaching(t *testing.T) {
	env := test.NewEnv(t)
	jwtMan := newJWTManager()
	jwtMan.start(env)
	defer jwtMan.stop()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwt, err := generateJWT(privateKey)
	if err != nil {
		t.Fatal(err)
	}

	good := goodJWTRequest(privateKey, t)
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !called {
			called = true
			good(w, r)
		} else {
			badJWTRequest(w, r)
		}
	}))
	defer ts.Close()

	for i := 0; i < 5; i++ {
		ctx := authtest.NewContext(ts.URL, test.NewEnv(t))

		// Do a first request and confirm that things look good.
		_, err = jwtMan.verifyJWT(ctx, jwt)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Refresh, should fail
	err = jwtMan.refresh()
	if err == nil {
		t.Errorf("Expected refresh to fail")
	}
}

func TestGoodAndBadJWT(t *testing.T) {
	env := test.NewEnv(t)
	jwtMan := newJWTManager()
	jwtMan.start(env)
	defer jwtMan.stop()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		goodJWTRequest(privateKey, t)(w, r)
	}))
	defer ts.Close()

	ctx := authtest.NewContext(ts.URL, test.NewEnv(t))

	// A good JWT request
	jwt, err := generateJWT(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	_, err = jwtMan.verifyJWT(ctx, jwt)
	if err != nil {
		t.Errorf("good JWT should not get error: %v", err)
	}

	// expired JWT
	jwt, err = generateExpiredJWT(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	_, err = jwtMan.verifyJWT(ctx, jwt)
	if err == nil {
		t.Errorf("expired JWT should get error")
	}

	// near future JWT
	jwt, err = generateFutureJWT(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	_, err = jwtMan.verifyJWT(ctx, jwt)
	if err != nil {
		t.Errorf("near future JWT should not get error")
	}

	// wrong key
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwt, err = generateJWT(wrongKey)
	if err != nil {
		t.Fatal(err)
	}
	_, err = jwtMan.verifyJWT(ctx, jwt)
	if err == nil {
		t.Errorf("JWT with wrong key should get error")
	}
}

func generateJWT(privateKey *rsa.PrivateKey) (string, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"api_product_list": []string{
			"EdgeMicroTestProduct",
		},
		"audience":         "microgateway",
		"jti":              "29e2320b-787c-4625-8599-acc5e05c68d0",
		"iss":              "https://theganyo1-eval-test.apigee.net/edgemicro-auth/token",
		"access_token":     "8E7Az3ZgPHKrgzcQA54qAzXT3Z1G",
		"client_id":        "yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H",
		"application_name": "61cd4d83-06b5-4270-a9ee-cf9255ef45c3",
		"scopes": []string{
			"scope1",
			"scope2",
		},
		"nbf": (time.Now().Add(-10 * time.Minute)).Unix(),
		"iat": time.Now().Unix(),
		"exp": (time.Now().Add(50 * time.Millisecond)).Unix(),
	})
	token.Header["kid"] = "1"

	return token.SignedString(privateKey)
}

func generateExpiredJWT(privateKey *rsa.PrivateKey) (string, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"jti": "29e2320b-787c-4625-8599-acc5e05c68d0",
		"iss": "https://theganyo1-eval-test.apigee.net/edgemicro-auth/token",
		"nbf": (time.Now().Add(-10 * time.Minute)).Unix(),
		"iat": (time.Now().Add(-10 * time.Minute)).Unix(),
		"exp": (time.Now().Add(-1 * time.Minute)).Unix(),
	})
	token.Header["kid"] = "1"

	return token.SignedString(privateKey)
}

func generateFutureJWT(privateKey *rsa.PrivateKey) (string, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"jti": "29e2320b-787c-4625-8599-acc5e05c68d0",
		"iss": "https://theganyo1-eval-test.apigee.net/edgemicro-auth/token",
		"nbf": (time.Now().Add(5 * time.Second)).Unix(),
		"iat": (time.Now().Add(5 * time.Second)).Unix(),
		"exp": (time.Now().Add(2 * time.Minute)).Unix(),
	})
	token.Header["kid"] = "1"

	return token.SignedString(privateKey)
}
