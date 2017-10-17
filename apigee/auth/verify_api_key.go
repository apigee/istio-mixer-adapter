package auth // import "github.com/apigee/istio-mixer-adapter/apigee/auth"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
)

func VerifyAPIKey(apidBase, orgName, apiKey, uriPath string) (*VerifyApiKeySuccessResponse, *ErrorResponse, error) {
	parsed, _ := url.Parse(apidBase) // already validated
	parsed.Path = path.Join(parsed.Path, "/verifiers/apikey")
	apidUrl := parsed.String()

	verifyRequestBody := VerifyApiKeyRequest{
		Action:           "verify",
		Key:              apiKey,
		OrganizationName: orgName,
		UriPath:          uriPath,
		// todo: ApiProxyName?
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(verifyRequestBody)

	req, err := http.NewRequest(http.MethodPost, apidUrl, body)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	fmt.Printf("Sending to apid (%s): %s\n", apidUrl, body)

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	buf := bytes.NewBuffer(make([]byte, 0, resp.ContentLength))
	_, err = buf.ReadFrom(resp.Body)
	respBody := buf.Bytes()


	switch resp.StatusCode {
	case 200:
		var success VerifyApiKeySuccessResponse
		if err = json.Unmarshal(respBody, &success); err != nil {
			fmt.Printf("Error unmarshaling: %s\n", string(respBody))
			return nil, nil, err
		}
		if success.Developer.Id != "" {
			fmt.Printf("Returning success: %v\n", success)
			return &success, nil, nil
		}

		var fail ErrorResponse
		err = json.Unmarshal(respBody, &fail)
		fmt.Printf("Returning fail: %v\n", fail)
		return nil, &fail, nil

	default:
		err = fmt.Errorf(string(respBody))
		return nil, nil, err
	}
}
