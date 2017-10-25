package analytics // import "github.com/apigee/istio-mixer-adapter/apigee/analytics"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"
)

func SendAnalyticsRecord(apidBase, orgName, envName string, record map[string]interface{}) error {

	fmt.Println("SendAnalyticsRecord")

	parsed, _ := url.Parse(apidBase) // already validated
	parsed.Path = path.Join(parsed.Path, "/analytics")
	apidUrl := parsed.String()

	record["recordType"] = "APIAnalytics"
	records := []map[string]interface{}{
		record,
	}

	// convert times to ms ints
	for k, v := range record {
		if t, ok := v.(time.Time); ok {
			record[k] = t.UnixNano() / (int64(time.Millisecond)/int64(time.Nanosecond))
		}
	}

	request := map[string]interface{}{
		"organization": orgName,
		"environment":  envName,
		"records":      records,
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(request)

	req, err := http.NewRequest(http.MethodPost, apidUrl, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	fmt.Printf("Sending to apid (%s): %s\n", apidUrl, body)

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	buf := bytes.NewBuffer(make([]byte, 0, resp.ContentLength))
	_, err = buf.ReadFrom(resp.Body)
	respBody := buf.Bytes()

	switch resp.StatusCode {
	case 200:
		fmt.Println("analytics accepted")
		return nil
	default:
		var errorResponse ErrorResponse
		if err = json.Unmarshal(respBody, &errorResponse); err != nil {
			return err
		}
		fmt.Printf("analytics not sent. reason: %s, code: %s",
			errorResponse.Reason, errorResponse.ErrorCode)
		return fmt.Errorf("analytics not sent. reason: %s, code: %s",
			errorResponse.Reason, errorResponse.ErrorCode)
	}
}

type ErrorResponse struct {
	ErrorCode string `json:"errorCode"`
	Reason    string `json:"reason"`
}

/*
type Request struct {
	Organization string   `json:"organization"`
	Environment  string   `json:"environment"`
	Records      []Record `json:"records"`
}

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
*/