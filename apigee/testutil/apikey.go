package testutil

import (
	"net/http"
	"encoding/json"
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"strings"
	"log"
)

func VerifyApiKeyOr(target http.HandlerFunc) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/verifiers/apikey") {

			verifyApiKeyResponse := auth.VerifyApiKeySuccessResponse{
				Developer: auth.DeveloperDetails{
					Id: "devId",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(verifyApiKeyResponse); err != nil {
				log.Fatalf("VerifyApiKeyOr error encoding: %v", verifyApiKeyResponse)
			}

			return
		}

		target(w, r)
	}
}
