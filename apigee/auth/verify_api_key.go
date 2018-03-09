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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/context"
	"istio.io/istio/pkg/cache"
)

const (
	verifyAPIKeyURL = "/verifyApiKey"
	// TODO(robbrit): Make these cache values configurable.
	defaultCacheTTL       = 30 * time.Minute
	cacheEvictionInterval = 10 * time.Second
	maxCachedEntries      = 10000
)

// keyVerifier encapsulates API key verification logic.
type keyVerifier interface {
	Verify(ctx context.Context, apiKey string) (map[string]interface{}, error)
}

type keyVerifierImpl struct {
	jwtMan *jwtManager
	cache  cache.ExpiringCache
}

func newVerifier(jwtMan *jwtManager) keyVerifier {
	return &keyVerifierImpl{
		jwtMan: jwtMan,
		cache:  cache.NewLRU(defaultCacheTTL, cacheEvictionInterval, maxCachedEntries),
	}
}

func (kv *keyVerifierImpl) fetchToken(ctx context.Context, apiKey string) (string, error) {
	if existing, ok := kv.cache.Get(apiKey); ok {
		token, ok := existing.(string)
		if !ok {
			// Whelp, somebody put something in they shouldn't have.
			return "", fmt.Errorf("cached value for %s is of invalid type %T", apiKey, existing)
		}
		return token, nil
	}
	verifyRequest := apiKeyRequest{
		APIKey: apiKey,
	}

	apiURL := ctx.CustomerBase()
	apiURL.Path = path.Join(apiURL.Path, verifyAPIKeyURL)

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(verifyRequest)

	req, err := http.NewRequest(http.MethodPost, apiURL.String(), body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	apiKeyResp := apiKeyResponse{}
	json.NewDecoder(resp.Body).Decode(&apiKeyResp)

	kv.cache.Set(apiKey, apiKeyResp.Token)
	return apiKeyResp.Token, nil
}

// verify returns the list of claims that an API key has.
func (kv *keyVerifierImpl) Verify(ctx context.Context, apiKey string) (map[string]interface{}, error) {
	ctx.Log().Infof("keyVerifierImpl.Verify(): %v", apiKey)

	token, err := kv.fetchToken(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, fmt.Errorf("invalid api key")
	}

	// TODO(robbrit): Do we need to clear the cache on error here?
	return kv.jwtMan.verifyJWT(ctx, token)
}
