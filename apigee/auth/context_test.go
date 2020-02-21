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
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestParseExp(t *testing.T) {
	now := time.Unix(time.Now().Unix(), 0)

	claims := map[string]interface{}{
		expClaim: float64(now.Unix()),
	}
	exp, err := parseExp(claims)
	if err != nil {
		t.Errorf("parseExp: %v", err)
	}
	if exp != now {
		t.Errorf("parseExp float got: %v, want: %v", exp, now)
	}

	claims[expClaim] = strconv.FormatInt(time.Now().Unix(), 10)
	exp, err = parseExp(claims)
	if err != nil {
		t.Errorf("parseExp: %v", err)
	}
	if exp != now {
		t.Errorf("parseExp string got: %v, want: %v", exp, now)
	}

	claims[expClaim] = "badexp"
	_, err = parseExp(claims)
	if err == nil {
		t.Error("parseExp should have gotten an error")
	}
}

func TestSetClaims(t *testing.T) {
	c := Context{}
	now := time.Unix(time.Now().Unix(), 0)
	claims := map[string]interface{}{
		apiProductListClaim: time.Now(),
		audienceClaim:       "aud",
		//clientIDClaim:        nil,
		applicationNameClaim: "app",
		scopesClaim:          nil,
		expClaim:             float64(now.Unix()),
		developerEmailClaim:  "email",
	}
	err := c.setClaims(claims)
	if err == nil {
		t.Errorf("setClaims without client_id should get error")
	}

	claims[clientIDClaim] = "clientID"
	err = c.setClaims(claims)
	if err == nil {
		t.Errorf("bad product list should error")
	}

	productsWant := []string{"product 1", "product 2"}
	claims[apiProductListClaim] = `["product 1", "product 2"]`
	err = c.setClaims(claims)
	if err != nil {
		t.Fatalf("valid setClaims, got: %v", err)
	}
	if !reflect.DeepEqual(c.APIProducts, productsWant) {
		t.Errorf("apiProducts want: %s, got: %v", productsWant, c.APIProducts)
	}

	claimsWant := []string{"scope1", "scope2"}
	claims[scopesClaim] = []interface{}{"scope1", "scope2"}
	err = c.setClaims(claims)
	if err != nil {
		t.Fatalf("valid setClaims, got: %v", err)
	}
	if !reflect.DeepEqual(claimsWant, c.Scopes) {
		t.Errorf("claims want: %s, got: %v", claimsWant, claims[scopesClaim])
	}

	//if c.A != "" {
	//	t.Errorf("nil ClientID should be empty, got: %v", c.ClientID)
	//}

	//ClientID       string
	//AccessToken    string
	//Application    string
	//APIProducts    []string
	//Expires        time.Time
	//DeveloperEmail string
	//Scopes         []string
	//APIKey         string
}
