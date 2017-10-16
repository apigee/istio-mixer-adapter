package auth // import "github.com/apigee/istio-mixer-adapter/apigee/auth"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
)

func VerifyAPIKey(apidBase, orgName, apikey, uriPath string) (success VerifyApiKeySuccessResponse, fail ErrorResponse, err error) {
	parsed, _ := url.Parse(apidBase) // already validated
	parsed.Path = path.Join(parsed.Path, "/verifiers/apikey")
	apidUrl := parsed.String()

	verifyRequestBody := VerifyApiKeyRequest{
		Action:           "verify",
		Key:              apikey,
		OrganizationName: orgName,
		UriPath:          uriPath,
		// todo: ApiProxyName?
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(verifyRequestBody)

	var req *http.Request
	req, err = http.NewRequest(http.MethodPost, apidUrl, body)
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	fmt.Printf("Sending to apid (%s): %s\n", apidUrl, body)

	client := http.DefaultClient
	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	buf := bytes.NewBuffer(make([]byte, 0, resp.ContentLength))
	_, err = buf.ReadFrom(resp.Body)
	respBody := buf.Bytes()

	switch resp.StatusCode {

	case 200:
		if err = json.Unmarshal(respBody, &success); err != nil {
			return
		}
		if success.Developer.Id != "" {
			fmt.Printf("Returning success: %v\n", success)
			return
		}

		err = json.Unmarshal(respBody, &fail)
		fmt.Printf("Returning fail: %v\n", fail)
		return

	default:
		err = fmt.Errorf(string(respBody))
		return
	}
}
