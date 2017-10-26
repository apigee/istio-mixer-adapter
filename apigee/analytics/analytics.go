package analytics // import "github.com/apigee/istio-mixer-adapter/apigee/analytics"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"
	"istio.io/mixer/pkg/adapter"
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
)

func SendAnalyticsRecord(env adapter.Env, apidBase, orgName, envName string, record map[string]interface{}) error {
	log := env.Logger()

	log.Infof("SendAnalyticsRecord\n")

	if apidBase == "" || orgName == "" || envName == "" {
		return fmt.Errorf("apidBase, orgName, envName are required")
	}

	annotateWithAuthFields(env, apidBase, orgName, envName, record)

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
			record[k] = t.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
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

	log.Infof("Sending to apid (%s): %s\n", apidUrl, body)

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


func annotateWithAuthFields(env adapter.Env, apidBase, orgName, envName string, record map[string]interface{}) {
	log := env.Logger()

	// todo: hack - perform authentication here as Istio is not able to pass auth context
	apiKey := ""
	if v := record["apikey"]; v != nil {
		apiKey = v.(string)
		delete(record, "apikey")
	}

	proxy := ""
	if v := record["apigeeproxy"]; v != nil {
		proxy = v.(string)
		delete(record, "apigeeproxy")
	}

	path := "/"
	if v := record["request_uri"]; v != nil {
		path = v.(string)
		if path == "" {
			path = "/"
		}
	}

	verifyApiKeyRequest := auth.VerifyApiKeyRequest{
		Key:              apiKey,
		OrganizationName: orgName,
		UriPath:          path,
		ApiProxyName:	  proxy,
		EnvironmentName:  envName,
	}

	success, fail, err := auth.VerifyAPIKey(env, apidBase, verifyApiKeyRequest)
	if err != nil {
		log.Warningf("annotateWithAuthFields error: %v\n", err)
		return
	}
	if fail != nil {
		log.Warningf("annotateWithAuthFields fail: %v\n", fail.ResponseMessage)
		return
	}

	record["apiproxy"] = proxy
	record["apiRevision"] = "" // todo: is this necessary? available?
	record["developerEmail"] = success.Developer.Email
	record["developerApp"] = success.App.Name
	record["accessToken"] = success.ClientId.ClientSecret
	record["clientID"] = success.ClientId.ClientId
	record["apiProduct"] = success.ApiProduct.Name
}

type ErrorResponse struct {
	ErrorCode string `json:"errorCode"`
	Reason    string `json:"reason"`
}
