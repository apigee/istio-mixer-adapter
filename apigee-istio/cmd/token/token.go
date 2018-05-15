// Copyright 2017 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package token

import (
	"encoding/json"
	"fmt"

	"net/http"

	"bytes"

	"github.com/apigee/istio-mixer-adapter/apigee-istio/shared"
	"github.com/spf13/cobra"
)

const (
	tokenURLFormat         = "%s/token" // customerProxyURL
	clientCredentialsGrant = "client_credentials"
)

type token struct {
	*shared.RootArgs
	id     string
	secret string
}

func Cmd(rootArgs *shared.RootArgs, printf, fatalf shared.FormatFn) *cobra.Command {
	t := &token{RootArgs: rootArgs}

	c := &cobra.Command{
		Use:   "token",
		Short: "OAuth Token Utilities",
		Long:  "OAuth Token Utilities",
	}

	c.AddCommand(cmdCreateToken(t, printf, fatalf))
	//c.AddCommand(cmdVerifyToken(t, printf, fatalf))
	//c.AddCommand(cmdDecodeToken(t, printf, fatalf))

	return c
}

func cmdCreateToken(t *token, printf, fatalf shared.FormatFn) *cobra.Command {
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a new OAuth token",
		Long:  "Create a new OAuth token",
		Args:  cobra.NoArgs,

		Run: func(cmd *cobra.Command, _ []string) {
			err := t.createToken(printf, fatalf)
			if err != nil {
				fatalf("error creating token: %v", err)
			}
		},
	}

	c.Flags().StringVarP(&t.id, "id", "i", "", "client id")
	c.Flags().StringVarP(&t.secret, "secret", "s", "", "client secret")

	c.MarkFlagRequired("id")
	c.MarkFlagRequired("secret")

	return c
}

func (t *token) createToken(printf, fatalf shared.FormatFn) error {

	tokenReq := &tokenRequest{
		ClientID:     t.id,
		ClientSecret: t.secret,
		GrantType:    clientCredentialsGrant,
	}
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(tokenReq)

	tokenURL := fmt.Sprintf(tokenURLFormat, t.CustomerProxyURL)
	req, err := http.NewRequest(http.MethodPost, tokenURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	var tokenRes tokenResponse
	resp, err := t.Client.Do(req, &tokenRes)
	if err != nil {
		return fmt.Errorf("error creating token: %v", err)
	}
	defer resp.Body.Close()

	printf(tokenRes.Token)
	return nil
}

type tokenRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
}

type tokenResponse struct {
	Token string `json:"token"`
}
