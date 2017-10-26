package analytics

import (
	"testing"
	"net/http/httptest"
	"net/http"
	"encoding/json"
	"time"
	"github.com/apigee/istio-mixer-adapter/apigee/testutil"
)

func TestAnalyticsSubmit(t *testing.T) {

	orgName := "orgName"
	envName := "envName"
	record := map[string]interface{}{}
	record["access_token"] = "AccessToken"
	startTime := time.Now()
	record["client_received_start_timestamp"] = startTime

	// Structs copied from apidAnalytics plugin...
	type Record struct {
		ClientReceivedStartTimestamp int64  `json:"client_received_start_timestamp"`
		ClientReceivedEndTimestamp   int64  `json:"client_received_end_timestamp"`
		ClientSentStartTimestamp     int64  `json:"client_sent_start_timestamp"`
		ClientSentEndTimestamp       int64  `json:"client_sent_end_timestamp"`
		TargetReceivedStartTimestamp int64  `json:"target_received_start_timestamp,omitempty"`
		TargetReceivedEndTimestamp   int64  `json:"target_received_end_timestamp,omitempty"`
		TargetSentStartTimestamp     int64  `json:"target_sent_start_timestamp,omitempty"`
		TargetSentEndTimestamp       int64  `json:"target_sent_end_timestamp,omitempty"`
		RecordType                   string `json:"recordType"`
		APIProxy                     string `json:"apiproxy"`
		RequestURI                   string `json:"request_uri"`
		RequestPath                  string `json:"request_path"`
		RequestVerb                  string `json:"request_verb"`
		ClientIP                     string `json:"client_ip,omitempty"`
		UserAgent                    string `json:"useragent"`
		APIProxyRevision             int    `json:"apiproxy_revision"`
		ResponseStatusCode           int    `json:"response_status_code"`
		DeveloperEmail               string `json:"developer_email,omitempty"`
		DeveloperApp                 string `json:"developer_app,omitempty"`
		AccessToken                  string `json:"access_token,omitempty"`
		ClientID                     string `json:"client_id,omitempty"`
		APIProduct                   string `json:"api_product,omitempty"`
	}
	type Request struct {
		Organization string   `json:"organization"`
		Environment  string   `json:"environment"`
		Records      []Record `json:"records"`
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		testutil.VerifyApiKeyOr(func(w http.ResponseWriter, r *http.Request) {

			decoder := json.NewDecoder(r.Body)
			var req Request
			err := decoder.Decode(&req)
			if err != nil {
				t.Error(err)
			}
			defer r.Body.Close()

			if req.Organization != orgName {
				t.Errorf("bad orgName")
			}
			if req.Environment != envName {
				t.Errorf("bad envName")
			}
			if len(req.Records) != 1 {
				t.Errorf("records missing")
				return
			}

			rec := req.Records[0]
			if rec.AccessToken == "" {
				t.Errorf("access_token missing")
			}
			msTime := startTime.UnixNano() / (int64(time.Millisecond)/int64(time.Nanosecond))
			if rec.ClientReceivedStartTimestamp != msTime {
				t.Errorf("client_received_start_timestamp expected: %v, got: %v",
					msTime, rec.ClientReceivedStartTimestamp)
			}

			w.WriteHeader(200)
		})
	}))
	defer ts.Close()

	err := SendAnalyticsRecord(testutil.MakeMockEnv(), ts.URL, orgName, envName, record)

	if err != nil {
		t.Error(err)
		return
	}
}

func TestBadApidBase(t *testing.T) {

	apidBase := "notanurl"
	orgName := "orgName"
	envName := "envName"
	record := map[string]interface{}{}

	err := SendAnalyticsRecord(testutil.MakeMockEnv(), apidBase, orgName, envName, record)

	if err == nil {
		t.Fail()
	}
}

func TestMissingApidBase(t *testing.T) {

	apidBase := ""
	orgName := "orgName"
	envName := "envName"
	record := map[string]interface{}{}

	err := SendAnalyticsRecord(testutil.MakeMockEnv(), apidBase, orgName, envName, record)

	if err == nil {
		t.Fail()
	}
}

func TestMissingOrg(t *testing.T) {

	apidBase := "http://localhost"
	orgName := ""
	envName := "envName"
	record := map[string]interface{}{}

	err := SendAnalyticsRecord(testutil.MakeMockEnv(), apidBase, orgName, envName, record)

	if err == nil {
		t.Fail()
	}
}

func TestMissingEnv(t *testing.T) {

	apidBase := "http://localhost"
	orgName := "orgName"
	envName := ""
	record := map[string]interface{}{}

	err := SendAnalyticsRecord(testutil.MakeMockEnv(), apidBase, orgName, envName, record)

	if err == nil {
		t.Fail()
	}
}
