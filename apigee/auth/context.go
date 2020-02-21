// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/context"
	"github.com/pkg/errors"
)

const (
	apiProductListClaim  = "api_product_list"
	audienceClaim        = "audience"
	clientIDClaim        = "client_id"
	applicationNameClaim = "application_name"
	scopesClaim          = "scopes"
	expClaim             = "exp"
	developerEmailClaim  = "application_developeremail"
	accessTokenClaim     = "access_token"
)

var (
	// AllValidClaims is a list of the claims expected from a JWT token
	AllValidClaims = []string{
		apiProductListClaim, audienceClaim, clientIDClaim, applicationNameClaim,
		scopesClaim, expClaim, developerEmailClaim,
	}
)

// A Context wraps all the various information that is needed to make requests
// through the Apigee adapter.
type Context struct {
	context.Context
	ClientID       string
	AccessToken    string
	Application    string
	APIProducts    []string
	Expires        time.Time
	DeveloperEmail string
	Scopes         []string
	APIKey         string
}

func parseExp(claims map[string]interface{}) (time.Time, error) {
	// JSON decodes this struct to either float64 or string, so we won't
	// need to check anything else.
	switch exp := claims[expClaim].(type) {
	case float64:
		return time.Unix(int64(exp), 0), nil
	case string:
		var expi int64
		var err error
		if expi, err = strconv.ParseInt(exp, 10, 64); err != nil {
			return time.Time{}, err
		}
		return time.Unix(expi, 0), nil
	}
	return time.Time{}, fmt.Errorf("unknown type %T for exp %v", claims[expClaim], claims[expClaim])
}

// if claims can't be processed, returns error and sets no fields
func (a *Context) setClaims(claims map[string]interface{}) error {
	if claims[apiProductListClaim] == nil {
		return fmt.Errorf("api_product_list claim is required")
	}

	products, err := parseArrayOfStrings(claims[apiProductListClaim])
	if err != nil {
		return errors.Wrapf(err, "unable to interpret api_product_list: %v", claims[apiProductListClaim])
	}

	scopes, err := parseArrayOfStrings(claims[scopesClaim])
	if err != nil {
		return errors.Wrapf(err, "unable to interpret scopes: %v", claims[scopesClaim])
	}

	exp, err := parseExp(claims)
	if err != nil {
		return err
	}

	var ok bool
	if _, ok = claims[clientIDClaim].(string); !ok {
		return errors.Wrapf(err, "unable to interpret %s: %v", clientIDClaim, claims[clientIDClaim])
	}
	if _, ok = claims[applicationNameClaim].(string); !ok {
		return errors.Wrapf(err, "unable to interpret %s: %v", applicationNameClaim, claims[applicationNameClaim])
	}
	a.ClientID = claims[clientIDClaim].(string)
	a.Application = claims[applicationNameClaim].(string)
	a.APIProducts = products
	a.Scopes = scopes
	a.Expires = exp
	a.DeveloperEmail, _ = claims[developerEmailClaim].(string)
	a.AccessToken, _ = claims[accessTokenClaim].(string)

	return nil
}

func (a *Context) isAuthenticated() bool {
	return a.ClientID != ""
}

func parseArrayOfStrings(obj interface{}) (results []string, err error) {
	if obj == nil {
		// nil is ok
	} else if arr, ok := obj.([]string); ok {
		results = arr
	} else if arr, ok := obj.([]interface{}); ok {
		for _, unk := range arr {
			if obj, ok := unk.(string); ok {
				results = append(results, obj)
			} else {
				err = fmt.Errorf("unable to interpret: %v", unk)
				break
			}
		}
	} else if str, ok := obj.(string); ok {
		err = json.Unmarshal([]byte(str), &results)
	} else {
		err = fmt.Errorf("unable to interpret: %v", obj)
	}
	return
}
