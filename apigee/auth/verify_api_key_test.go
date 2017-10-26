package auth

import (
	"testing"
	"net/http/httptest"
	"net/http"
	"encoding/json"
	"github.com/apigee/istio-mixer-adapter/apigee/testutil"
)

func TestVerifyAPIKeyValid(t *testing.T) {

	verifyApiKeyRequest := VerifyApiKeyRequest{
		Action: 		  "verify",
		Key:              "apiKey",
		OrganizationName: "orgName",
		UriPath:          "path",
		ApiProxyName:	  "proxy",
		EnvironmentName:  "envName",
		ValidateAgainstApiProxiesAndEnvs: true,
	}

	verifyApiKeyResponse := VerifyApiKeySuccessResponse{
		Developer: DeveloperDetails{
			Id: "devid",
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var req VerifyApiKeyRequest
		err := decoder.Decode(&req)
		if err != nil {
			t.Error(err)
		}
		defer r.Body.Close()

		if verifyApiKeyRequest != req {
			t.Errorf("expected: %v, got: %v", verifyApiKeyRequest, req)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(verifyApiKeyResponse)
	}))
	defer ts.Close()

	success, fail, err := VerifyAPIKey(testutil.MakeMockEnv(), ts.URL, verifyApiKeyRequest)

	if err != nil {
		t.Error(err)
	}

	if fail != nil {
		t.Errorf("failed: %v", fail)
	}

	if success == nil {
		t.Error("success should not be nil")
	}

	if success.Developer.Id != verifyApiKeyResponse.Developer.Id {
		t.Errorf("expected dev id: %v, got: %v", verifyApiKeyResponse.Developer.Id, success.Developer.Id)
	}
}

func TestVerifyAPIKeyFail(t *testing.T) {

	res := ErrorResponse{
		ResponseMessage: "API Key verify failed for (apiKey, orgName)",
		ResponseCode: "oauth.v2.InvalidApiKey",
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	}))
	defer ts.Close()

	success, fail, err := VerifyAPIKey(testutil.MakeMockEnv(), ts.URL, VerifyApiKeyRequest{})

	if err != nil {
		t.Error(err)
	}

	if success != nil {
		t.Errorf("success should be nil, is: %v", success)
	}

	if *fail != res {
		t.Errorf("expected fail: %v, got: %v", &res, fail)
	}
}

func TestVerifyAPIKeyError(t *testing.T) {

	success, fail, err := VerifyAPIKey(testutil.MakeMockEnv(),"http://localhost", VerifyApiKeyRequest{})

	if err == nil {
		t.Errorf("error should not be nil")
	}

	if success != nil {
		t.Errorf("success should be nil, is: %v", success)
	}

	if fail != nil {
		t.Errorf("fail should be nil, is: %v", fail)
	}
}
