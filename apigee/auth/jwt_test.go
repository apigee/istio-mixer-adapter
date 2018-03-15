package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apigee/istio-mixer-adapter/apigee/authtest"
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
