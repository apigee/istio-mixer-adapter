package auth

import (
	"fmt"
	"testing"

	"github.com/apigee/istio-mixer-adapter/apigee/context"
	adaptertest "istio.io/istio/mixer/pkg/adapter/test"
)

type testVerifier struct {
	goodAPIKey string
	claims     map[string]interface{}
}

var testJWTClaims = map[string]interface{}{
	"client_id":        "hi",
	"application_name": "taco",
	"exp":              14.0,
	"api_product_list": []string{"superapp"},
	"scopes":           []string{"scope"},
}

func (tv *testVerifier) Verify(ctx context.Context, apiKey string) (map[string]interface{}, error) {
	if apiKey != tv.goodAPIKey {
		return nil, fmt.Errorf("invalid auth key")
	}
	// Just return some dummy value.
	return testJWTClaims, nil
}

func TestAuthenticate(t *testing.T) {
	goodAPIKey := "good"

	for _, test := range []struct {
		desc      string
		apiKey    string
		claims    map[string]interface{}
		wantError bool
	}{
		{"with valid JWT", "", testJWTClaims, false},
		{"with invalid JWT", "", map[string]interface{}{
			"client_id": "bad",
		}, true},
		{"with valid API key", "good", nil, false},
		{"with invalid API key", "bad", nil, true},
	} {
		t.Log(test.desc)

		verifier = &testVerifier{
			goodAPIKey: goodAPIKey,
		}

		_, err := Authenticate(&testContext{
			log: adaptertest.NewEnv(t),
		}, test.apiKey, test.claims)
		if err != nil {
			if !test.wantError {
				t.Errorf("unexpected error: %s", err)
			}
			continue
		}
		if test.wantError {
			t.Errorf("wanted error, got none")
			continue
		}
	}
}
