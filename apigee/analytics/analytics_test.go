package analytics

import (
	"testing"
	"net/http/httptest"
	"net/http"
	"encoding/json"
	"time"
	"github.com/apigee/istio-mixer-adapter/apigee/testutil"
	"net/url"
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"strings"
)

func TestAnalyticsSubmit(t *testing.T) {

	startTime := time.Now()
	authResponse := &auth.VerifyApiKeySuccessResponse{
		Organization: "orgName",
		Environment: "envName",
		ClientId: auth.ClientIdDetails{
			ClientSecret: "AccessToken",
		},
	}
	axRecord := &Record{
		ClientReceivedStartTimestamp: TimeToUnix(startTime),
	}

	ts := makeTestServer(authResponse, axRecord, t)
	defer ts.Close()
	apidBase, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	err = SendAnalyticsRecord(testutil.MakeMockEnv(), *apidBase, authResponse, axRecord)
	if err != nil {
		t.Error(err)
	}
}

func TestBadApidBase(t *testing.T) {

	authResponse := &auth.VerifyApiKeySuccessResponse{
		Organization: "orgName",
		Environment: "envName",
		ClientId: auth.ClientIdDetails{
			ClientSecret: "AccessToken",
		},
	}
	axRecord := &Record{}
	ts := makeTestServer(authResponse, axRecord, t)
	defer ts.Close()
	apidBase := url.URL{}
	err := SendAnalyticsRecord(testutil.MakeMockEnv(), apidBase, authResponse, axRecord)
	if err == nil {
		t.Errorf("should get bad apid base error")
	}
}

func TestMissingOrg(t *testing.T) {

	authResponse := &auth.VerifyApiKeySuccessResponse{
		Organization: "",
		Environment: "envName",
		ClientId: auth.ClientIdDetails{
			ClientSecret: "AccessToken",
		},
	}
	axRecord := &Record{}
	ts := makeTestServer(authResponse, axRecord, t)
	defer ts.Close()
	apidBase, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	err = SendAnalyticsRecord(testutil.MakeMockEnv(), *apidBase, authResponse, axRecord)
	if err == nil || !strings.Contains(err.Error(), "organization") {
		t.Errorf("should get missing organization error, got: %s", err)
	}
}

func TestMissingEnv(t *testing.T) {

	authResponse := &auth.VerifyApiKeySuccessResponse{
		Organization: "orgName",
		Environment: "",
		ClientId: auth.ClientIdDetails{
			ClientSecret: "AccessToken",
		},
	}
	axRecord := &Record{}
	ts := makeTestServer(authResponse, axRecord, t)
	defer ts.Close()
	apidBase, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	err = SendAnalyticsRecord(testutil.MakeMockEnv(), *apidBase, authResponse, axRecord)
	if err == nil || !strings.Contains(err.Error(), "environment") {
		t.Errorf("should get missing environment error, got: %s", err)
	}
}

func makeTestServer(auth *auth.VerifyApiKeySuccessResponse, rec *Record, t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var axRequest Request
		err := decoder.Decode(&axRequest)
		if err != nil {
			t.Error(err)
		}
		defer r.Body.Close()

		if axRequest.Organization != auth.Organization {
			t.Errorf("bad orgName")
		}
		if axRequest.Environment != auth.Environment {
			t.Errorf("bad envName")
		}
		if len(axRequest.Records) != 1 {
			t.Errorf("record missing")
			return
		}

		axRecord := axRequest.Records[0]
		if axRecord.AccessToken == "" {
			t.Errorf("access_token missing")
		}
		if axRecord.ClientReceivedStartTimestamp != rec.ClientReceivedStartTimestamp {
			t.Errorf("client_received_start_timestamp expected: %v, got: %v",
				rec.ClientReceivedStartTimestamp, axRecord.ClientReceivedStartTimestamp)
		}

		w.WriteHeader(200)
	}))

}