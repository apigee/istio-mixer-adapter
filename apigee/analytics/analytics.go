package analytics // import "github.com/apigee/istio-mixer-adapter/apigee/analytics"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	//"time"
	"istio.io/mixer/pkg/adapter"
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"time"
)

const apidPath = "/analytics"
const axRecordType = "APIAnalytics"

func TimeToUnix(t time.Time) int64 {
	return t.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

func SendAnalyticsRecord(env adapter.Env, apidBase url.URL, auth *auth.VerifyApiKeySuccessResponse, record *Record) error {
	log := env.Logger()

	if auth.Organization == "" || auth.Environment == "" {
		return fmt.Errorf("organization and environment are required in auth: %v", auth)
	}

	record.RecordType = axRecordType

	// populate auth context
	record.DeveloperEmail = auth.Developer.Email
	record.DeveloperApp = auth.App.Name
	record.AccessToken = auth.ClientId.ClientSecret
	record.ClientID = auth.ClientId.ClientId
	record.APIProduct = auth.ApiProduct.Name

	apidBase.Path = path.Join(apidBase.Path, apidPath)
	apidBaseString := apidBase.String()

	request := Request{
		Organization: auth.Organization,
		Environment: auth.Environment,
		Records: []Record{ *record },
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(request)

	req, err := http.NewRequest(http.MethodPost, apidBaseString, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	log.Infof("Sending to apid (%s): %s\n", apidBaseString, body)

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
		log.Infof("analytics accepted\n")
		return nil
	default:
		var errorResponse ErrorResponse
		if err = json.Unmarshal(respBody, &errorResponse); err != nil {
			return err
		}
		log.Errorf("analytics not sent. reason: %s, code: %s\n",
			errorResponse.Reason, errorResponse.ErrorCode)
		return fmt.Errorf("analytics not sent. reason: %s, code: %s",
			errorResponse.Reason, errorResponse.ErrorCode)
	}
}

type ErrorResponse struct {
	ErrorCode string `json:"errorCode"`
	Reason    string `json:"reason"`
}
