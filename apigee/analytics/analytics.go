package analytics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"net/http"
	"time"
	"path"
)

const (
	axPath       = "/axpublisher/organization/%s/environment/%s"
	axRecordType = "APIAnalytics"
)

func TimeToUnix(t time.Time) int64 {
	return t.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

func SendRecord(auth auth.Context, record *Record) error {
	if auth.Organization() == "" || auth.Environment() == "" {
		return fmt.Errorf("organization and environment are required in auth: %v", auth)
	}

	record.RecordType = axRecordType

	// populate from auth context
	record.DeveloperEmail = auth.DeveloperEmail
	record.DeveloperApp = auth.Application
	record.AccessToken = auth.AccessToken
	record.ClientID = auth.ClientID

	// todo: seriously?
	if len(auth.APIProducts) > 0 {
		record.APIProduct = auth.APIProducts[0]
	}

	axURL := auth.ApigeeBase()
	axURL.Path = path.Join(axURL.Path, fmt.Sprintf(axPath, auth.Organization(), auth.Environment()))

	request := Request{
		Organization: auth.Organization(),
		Environment:  auth.Environment(),
		Records:      []Record{*record},
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(request)

	req, err := http.NewRequest(http.MethodPost, axURL.String(), body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	auth.Log().Infof("Sending to (%s): %s\n", axURL.String(), body)

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
		auth.Log().Infof("analytics accepted\n")
		return nil
	default:
		var errorResponse ErrorResponse
		if err = json.Unmarshal(respBody, &errorResponse); err != nil {
			return err
		}
		auth.Log().Errorf("analytics not sent. reason: %s, code: %s\n",
			errorResponse.Reason, errorResponse.ErrorCode)
		return fmt.Errorf("analytics not sent. reason: %s, code: %s",
			errorResponse.Reason, errorResponse.ErrorCode)
	}
}

type ErrorResponse struct {
	ErrorCode string `json:"errorCode"`
	Reason    string `json:"reason"`
}
