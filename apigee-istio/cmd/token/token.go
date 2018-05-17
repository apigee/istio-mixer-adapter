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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee-istio/shared"
	"github.com/lestrrat/go-jwx/jwk"
	"github.com/lestrrat/go-jwx/jws"
	"github.com/lestrrat/go-jwx/jwt"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	tokenURLFormat         = "%s/token" // customerProxyURL
	certsURLFormat         = "%s/certs" // customerProxyURL
	clientCredentialsGrant = "client_credentials"
)

type token struct {
	*shared.RootArgs
	id     string
	secret string
	file   string
}

// Cmd returns base command
func Cmd(rootArgs *shared.RootArgs, printf, fatalf shared.FormatFn) *cobra.Command {
	t := &token{RootArgs: rootArgs}

	c := &cobra.Command{
		Use:   "token",
		Short: "JWT Token Utilities",
		Long:  "JWT Token Utilities",
	}

	c.AddCommand(cmdCreateToken(t, printf, fatalf))
	c.AddCommand(cmdInspectToken(t, printf, fatalf))
	//c.AddCommand(cmdRotateKey(t, printf, fatalf))

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

func cmdInspectToken(t *token, printf, fatalf shared.FormatFn) *cobra.Command {
	c := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect a JWT token",
		Long:  "Inspect a JWT token",
		Args:  cobra.NoArgs,

		Run: func(cmd *cobra.Command, _ []string) {
			err := t.inspectToken(printf, fatalf)
			if err != nil {
				fatalf("error inspecting token: %v", err)
			}
		},
	}

	c.Flags().StringVarP(&t.file, "file", "f", "", "token file (default: use stdin)")

	return c
}

func cmdRotateKey(t *token, printf, fatalf shared.FormatFn) *cobra.Command {
	c := &cobra.Command{
		Use:   "rotate-key",
		Short: "rotate private/public key",
		Long:  "rotate private/public key",
		Args:  cobra.NoArgs,

		Run: func(cmd *cobra.Command, _ []string) {
			err := t.rotateKey(printf, fatalf)
			if err != nil {
				fatalf("error creating token: %v", err)
			}
		},
	}

	c.Flags().StringVarP(&t.id, "kid", "k", "1", "new key id")

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

func (t *token) inspectToken(printf, fatalf shared.FormatFn) error {
	// Print JWT
	var file = os.Stdin
	if t.file != "" {
		var err error
		file, err = os.Open(t.file)
		if err != nil {
			fatalf("error opening file: %v", err)
		}
	}

	jwtBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return errors.Wrap(err, "error reading jwt token")
	}
	token, err := jwt.ParseBytes(jwtBytes)
	if err != nil {
		return errors.Wrap(err, "error parsing jwt token")
	}
	jsonBytes, err := token.MarshalJSON()
	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonBytes, "", "\t")
	if err != nil {
		return errors.Wrap(err, "error printing jwt token")
	}
	printf(prettyJSON.String())

	// verify JWT
	printf("\nverifying...")

	url := fmt.Sprintf(certsURLFormat, t.CustomerProxyURL)
	jwkSet, err := jwk.FetchHTTP(url)
	if err != nil {
		fatalf("error fetching certs: %v", err)
	}
	_, err = jws.VerifyWithJWKSet(jwtBytes, jwkSet, nil)
	if err != nil {
		fatalf("certificate error: %v", err)
	}
	tokenURL := fmt.Sprintf(tokenURLFormat, t.CustomerProxyURL)
	err = token.Verify(
		jwt.WithAcceptableSkew(time.Minute),
		jwt.WithAudience("istio"),
		jwt.WithIssuer(tokenURL),
	)
	if err != nil {
		fatalf("verification error: %v", err)
	}

	printf("token ok.")
	return nil
}

func (t *token) rotateKey(printf, fatalf shared.FormatFn) error {
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
