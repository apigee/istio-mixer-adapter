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
)

const (
	apiProductListClaim  = "api_product_list"
	audienceClaim        = "audience"
	clientIDClaim        = "client_id"
	applicationNameClaim = "application_name"
	scopesClaim          = "scopes"
	expClaim             = "exp"
)

var (
	// AllValidClaims is a list of the claims expected from a JWT token
	AllValidClaims = []string{
		apiProductListClaim, audienceClaim, clientIDClaim, applicationNameClaim, scopesClaim, expClaim,
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
}

func parseExp(claims map[string]interface{}) (time.Time, error) {
	// JSON decodes this struct to either float64 or string, so we won't
	// need to check anything else.
	switch exp := claims["exp"].(type) {
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
	return time.Time{}, fmt.Errorf("unknown type for time %s: %T", claims["exp"], claims["exp"])
}

// does nothing if claims is empty
func (a *Context) setClaims(claims map[string]interface{}) error {
	if claims[clientIDClaim] == nil {
		return nil
	}
	a.Log().Infof("setClaims: %v", claims)

	products, err := parseArrayOfStrings(claims[apiProductListClaim])
	if err != nil {
		return fmt.Errorf("unable to interpret api_product_list: %v", claims[apiProductListClaim])
	}

	scopes, err := parseArrayOfStrings(claims[scopesClaim])
	if err != nil {
		return fmt.Errorf("unable to interpret scopes: %v", claims[scopesClaim])
	}

	exp, err := parseExp(claims)
	if err != nil {
		return err
	}

	var ok bool
	if a.ClientID, ok = claims[clientIDClaim].(string); !ok {
		return fmt.Errorf("unable to interpret client_id: %v", claims[clientIDClaim])
	}
	if a.Application, ok = claims[applicationNameClaim].(string); !ok {
		return fmt.Errorf("unable to interpret application_name: %v", claims[applicationNameClaim])
	}
	a.APIProducts = products
	a.Scopes = scopes
	a.Expires = exp

	return nil
}

func parseArrayOfStrings(obj interface{}) (results []string, err error) {
	if arr, ok := obj.([]interface{}); ok {
		for _, unk := range arr {
			if obj, ok := unk.(string); ok {
				results = append(results, obj)
			} else {
				err = fmt.Errorf("unable to interpret: %v", unk)
				break
			}
		}
		return results, err
	} else if str, ok := obj.(string); ok {
		err = json.Unmarshal([]byte(str), &results)
		return
	}
	return
}

// todo: add developerEmail
/*
jwt claims:
{
 api_product_list: [
  "EdgeMicroTestProduct"
 ],
 audience: "microgateway",
 jti: "29e2320b-787c-4625-8599-acc5e05c68d0",
 iss: "https://theganyo1-eval-test.apigee.net/edgemicro-auth/token",
 access_token: "8E7Az3ZgPHKrgzcQA54qAzXT3Z1G",
 client_id: "yBQ5eXZA8rSoipYEi1Rmn0Z8RKtkGI4H",
 nbf: 1516387728,
 iat: 1516387728,
 application_name: "61cd4d83-06b5-4270-a9ee-cf9255ef45c3",
 scopes: [
  "scope1",
  "scope2"
 ],
 exp: 1516388028
}
*/
