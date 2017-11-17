package testutil

import (
	"net/http"
	"encoding/json"
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"strings"
	"log"
	"fmt"
)

func VerifyApiKeyOr(target http.HandlerFunc) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/verifiers/apikey") {

			decoder := json.NewDecoder(r.Body)
			var req auth.VerifyApiKeyRequest
			err := decoder.Decode(&req)
			if err != nil {
				fmt.Printf("VerifyApiKeyOr error: %v", err)
				panic(err)
			}
			defer r.Body.Close()

			w.Header().Set("Content-Type", "application/json")

			switch req.Key {
			case "fail" :
				errResponse := auth.ErrorResponse{
					ResponseCode: "fail",
					ResponseMessage: "fail",
					StatusCode: 200,
					Kind: "",
				}
				if err := json.NewEncoder(w).Encode(errResponse); err != nil {
					log.Fatalf("VerifyApiKeyOr error encoding: %v", errResponse)
				}
			case "error":
				w.WriteHeader(500)
			default:
				verifyApiKeyResponse := auth.VerifyApiKeySuccessResponse{
					Developer: auth.DeveloperDetails{
						Id: "devId",
					},
					Environment: req.EnvironmentName,
				}
				if err := json.NewEncoder(w).Encode(verifyApiKeyResponse); err != nil {
					log.Fatalf("VerifyApiKeyOr error encoding: %v", verifyApiKeyResponse)
				}
			}

			return
		}

		target(w, r)
	}
}
