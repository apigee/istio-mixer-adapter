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

	"github.com/apigee/istio-mixer-adapter/apigee-istio/cmd/provision"
	"github.com/apigee/istio-mixer-adapter/apigee-istio/shared"
	"github.com/lestrrat/go-jwx/jwk"
	"github.com/lestrrat/go-jwx/jws"
	"github.com/lestrrat/go-jwx/jwt"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	tokenURLFormat         = "%s/token"  // customerProxyURL
	certsURLFormat         = "%s/certs"  // customerProxyURL
	rotateURLFormat        = "%s/rotate" // customerProxyURL
	clientCredentialsGrant = "client_credentials"
)

type token struct {
	*shared.RootArgs
	clientID              string
	clientSecret          string
	file                  string
	keyID                 string
	certExpirationInYears int
	certKeyStrength       int
}

// Cmd returns base command
func Cmd(rootArgs *shared.RootArgs, printf, fatalf shared.FormatFn) *cobra.Command {
	t := &token{RootArgs: rootArgs}

	c := &cobra.Command{
		Use:   "token",
		Short: "JWT Token Utilities",
		Long:  "JWT Token Utilities",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return rootArgs.Resolve(true)
		},
	}

	c.AddCommand(cmdCreateToken(t, printf, fatalf))
	c.AddCommand(cmdInspectToken(t, printf, fatalf))
	c.AddCommand(cmdRotateCert(t, printf, fatalf))

	return c
}

func cmdCreateToken(t *token, printf, fatalf shared.FormatFn) *cobra.Command {
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a new OAuth token",
		Long:  "Create a new OAuth token",
		Args:  cobra.NoArgs,

		Run: func(cmd *cobra.Command, _ []string) {
			_, err := t.createToken(printf, fatalf)
			if err != nil {
				fatalf("error creating token: %v", err)
			}
		},
	}

	c.Flags().StringVarP(&t.clientID, "id", "i", "", "client id")
	c.Flags().StringVarP(&t.clientSecret, "secret", "s", "", "client secret")

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

func cmdRotateCert(t *token, printf, fatalf shared.FormatFn) *cobra.Command {
	c := &cobra.Command{
		Use:   "rotate-cert",
		Short: "rotate JWT certificate",
		Long:  "Deploys a new private and public key while maintaining the current public key for existing tokens.",
		Args:  cobra.NoArgs,

		Run: func(cmd *cobra.Command, _ []string) {
			t.rotateCert(printf, fatalf)
		},
	}

	c.Flags().StringVarP(&t.keyID, "kid", "", "1", "new key id")
	c.Flags().IntVarP(&t.certExpirationInYears, "years", "", 1,
		"number of years before the cert expires")
	c.Flags().IntVarP(&t.certKeyStrength, "strength", "", 2048,
		"key strength")

	c.Flags().StringVarP(&t.clientID, "key", "k", "", "istio provision key")
	c.Flags().StringVarP(&t.clientSecret, "secret", "s", "", "istio provision secret")

	c.MarkFlagRequired("key")
	c.MarkFlagRequired("secret")

	return c
}

func (t *token) createToken(printf, fatalf shared.FormatFn) (string, error) {
	tokenReq := &tokenRequest{
		ClientID:     t.clientID,
		ClientSecret: t.clientSecret,
		GrantType:    clientCredentialsGrant,
	}
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(tokenReq)

	tokenURL := fmt.Sprintf(tokenURLFormat, t.CustomerProxyURL)
	req, err := http.NewRequest(http.MethodPost, tokenURL, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	var tokenRes tokenResponse
	resp, err := t.Client.Do(req, &tokenRes)
	if err != nil {
		return "", fmt.Errorf("error creating token: %v", err)
	}
	defer resp.Body.Close()

	printf(tokenRes.Token)
	return tokenRes.Token, nil
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

// RotateKey is called by `token rotate-cert`
func (t *token) rotateCert(printf, fatalf shared.FormatFn) {
	var verbosef = shared.NoPrintf
	if t.Verbose {
		verbosef = printf
	}

	verbosef("generating a new key and cert...")
	cert, privateKey, err := provision.GenKeyCert(t.certKeyStrength, t.certExpirationInYears)
	if err != nil {
		fatalf("error generating new cert: %v", err)
	}

	rotateReq := rotateRequest{
		PrivateKey:  privateKey,
		Certificate: cert,
		KeyID:       t.keyID,
	}

	verbosef("rotating certificate...")

	body := new(bytes.Buffer)
	err = json.NewEncoder(body).Encode(rotateReq)
	if err != nil {
		fatalf("encoding error: %v", err)
	}

	rotateURL := fmt.Sprintf(rotateURLFormat, t.CustomerProxyURL)
	req, err := http.NewRequest(http.MethodPost, rotateURL, body)
	if err != nil {
		fatalf("unable to create request: %v", err)
	}
	req.SetBasicAuth(t.clientID, t.clientSecret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := t.Client.Do(req, nil)
	if err != nil {
		if resp.StatusCode == 401 {
			fatalf("authentication failed, check your key and secret")
		}
		fatalf("rotation request error: %v", err)
	}
	defer resp.Body.Close()

	verbosef("new certificate:\n%s", cert)
	verbosef("new private key:\n%s", privateKey)

	printf("certificate successfully rotated")
}

type rotateRequest struct {
	PrivateKey  string `json:"private_key"`
	Certificate string `json:"certificate"`
	KeyID       string `json:"kid"`
}

type tokenRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
}

type tokenResponse struct {
	Token string `json:"token"`
}
