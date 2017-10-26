package testutil

import (
	"net/http"
	"encoding/json"
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"strings"
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

			verifyApiKeyResponse := auth.VerifyApiKeySuccessResponse{
				Developer: auth.DeveloperDetails{
					Id: "devId",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(verifyApiKeyResponse)

			return
		}

		target(w, r)
	}
}
