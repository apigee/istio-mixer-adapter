package auth // import "github.com/apigee/istio-mixer-adapter/apigee/auth"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"istio.io/istio/mixer/pkg/adapter"
)

func VerifyAPIKey(env adapter.Env, apidBase url.URL, verifyRequest VerifyApiKeyRequest) (*VerifyApiKeySuccessResponse, *ErrorResponse, error) {
	log := env.Logger()

	// clients don't need to set
	verifyRequest.Action = "verify"
	verifyRequest.ValidateAgainstApiProxiesAndEnvs = true

	apidBase.Path = path.Join(apidBase.Path, "/verifiers/apikey")
	apidBaseString := apidBase.String()

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(verifyRequest)

	req, err := http.NewRequest(http.MethodPost, apidBaseString, body)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	log.Infof("Sending to apid (%s): %s\n", apidBaseString, body)

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
			log.Errorf("Error unmarshaling: %s\n", string(respBody))
			return nil, nil, err
		}
		if success.Developer.Id != "" {
			log.Infof("Returning success: %v\n", success)
			return &success, nil, nil
		}

		var fail ErrorResponse
		err = json.Unmarshal(respBody, &fail)
		log.Infof("Returning fail: %v\n", fail)
		return nil, &fail, nil

	default:
		err = fmt.Errorf(string(respBody))
		return nil, nil, err
	}
}

/*
Example fail:
{
   "response_message" : "API Key verify failed for (4XxBlHNPEhEhiX23I305iayNfNdxPmGJx, edgex01)",
   "response_code" : "oauth.v2.InvalidApiKey"
}

Example success:
{
   "apiProduct" : {
      "name" : "apigee-remote-proxy",
      "created_at" : "2017-09-20 23:05:09.234+00:00",
      "displayName" : "apigee-remote-proxy",
      "apiproxies" : [
         "apigee-remote-proxy"
      ],
      "created_by" : "theganyo@google.com",
      "id" : "db90a25a-15c8-42ad-96c1-63ed9682b5a9",
      "attributes" : [
         {
            "name" : "access",
            "value" : "public"
         }
      ],
      "lastmodified_by" : "theganyo@google.com",
      "lastmodified_at" : "2017-09-20 23:05:09.234+00:00",
      "environments" : [
         "prod",
         "test"
      ]
   },
   "clientId" : {
      "clientSecret" : "4IKfx8uASxfX4v7K",
      "status" : "APPROVED",
      "clientId" : "00A0RcOti8kEtstbt5knxbRXFpIUGOMP"
   },
   "app" : {
      "lastmodified_by" : "theganyo@google.com",
      "lastmodified_at" : "2017-09-20 23:05:59.125+00:00",
      "appFamily" : "default",
      "id" : "ae053aee-f12d-4591-84ef-2e6ae0d4205d",
      "created_by" : "theganyo@google.com",
      "attributes" : [
         {
            "name" : "DisplayName",
            "value" : "apigee-remote-proxy"
         },
         {
            "name" : "Notes"
         }
      ],
      "name" : "apigee-remote-proxy",
      "created_at" : "2017-09-20 23:05:59.125+00:00",
      "status" : "APPROVED"
   },
   "developer" : {
      "created_by" : "theganyo@google.com",
      "id" : "590f33bf-f05c-48c1-bb93-183759bd9ee1",
      "userName" : "remoteproxy",
      "created_at" : "2017-09-20 23:03:52.327+00:00",
      "status" : "ACTIVE",
      "lastName" : "proxy",
      "lastmodified_by" : "theganyo@google.com",
      "firstName" : "remote",
      "lastmodified_at" : "2017-09-20 23:03:52.327+00:00",
      "email" : "remoteproxy@apigee.com"
   },
   "company" : {}
}
 */