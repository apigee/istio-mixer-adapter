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

	"github.com/apigee/istio-mixer-adapter/adapter/context"
	"github.com/pkg/errors"
	"istio.io/istio/pkg/cache"
)

const (
	verifyAPIKeyURL              = "/verifyApiKey"
	defaultCacheTTL              = 30 * time.Minute
	defaultCacheEvictionInterval = 10 * time.Second
	defaultMaxCachedEntries      = 10000
)

// keyVerifier encapsulates API key verification logic.
type keyVerifier interface {
	Verify(ctx context.Context, apiKey string) (map[string]interface{}, error)
}

type keyVerifierImpl struct {
	jwtMan *jwtManager
	cache  cache.ExpiringCache
	now    func() time.Time
	client *http.Client
}

type keyVerifierOpts struct {
	CacheTTL              time.Duration
	CacheEvictionInterval time.Duration
	MaxCachedEntries      int
	Client                *http.Client
}

func newVerifier(jwtMan *jwtManager, opts keyVerifierOpts) keyVerifier {
	if opts.CacheTTL == 0 {
		opts.CacheTTL = defaultCacheTTL
	}
	if opts.CacheEvictionInterval == 0 {
		opts.CacheEvictionInterval = defaultCacheEvictionInterval
	}
	if opts.MaxCachedEntries == 0 {
		opts.MaxCachedEntries = defaultMaxCachedEntries
	}
	return &keyVerifierImpl{
		jwtMan: jwtMan,
		cache:  cache.NewLRU(opts.CacheTTL, opts.CacheEvictionInterval, int32(opts.MaxCachedEntries)),
		now:    time.Now,
		client: opts.Client,
	}
}

func (kv *keyVerifierImpl) fetchToken(ctx context.Context, apiKey string) (string, error) {
	verifyRequest := APIKeyRequest{
		APIKey: apiKey,
	}

	apiURL := *ctx.CustomerBase()
	apiURL.Path = path.Join(apiURL.Path, verifyAPIKeyURL)

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(verifyRequest)

	req, err := http.NewRequest(http.MethodPost, apiURL.String(), body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(ctx.Key(), ctx.Secret())

	resp, err := kv.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	apiKeyResp := APIKeyResponse{}
	json.NewDecoder(resp.Body).Decode(&apiKeyResp)

	return apiKeyResp.Token, nil
}

// verify returns the list of claims that an API key has.
func (kv *keyVerifierImpl) Verify(ctx context.Context, apiKey string) (map[string]interface{}, error) {
	var token string

	if existing, ok := kv.cache.Get(apiKey); ok {
		token = existing.(string)
	} else {
		var err error
		token, err = kv.fetchToken(ctx, apiKey)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching token")
		}
	}

	if token == "" {
		return nil, fmt.Errorf("invalid api key")
	}

	// no need to verify our own token, just parse it
	jwt, err := kv.jwtMan.parseJWT(ctx, token, false)
	if err != nil {
		return nil, errors.Wrap(err, "parsing jwt")
	}
	exp, err := parseExp(jwt)
	if err != nil {
		return nil, err
	}

	// A bit hacky, but since SetWithExpiration uses a duration instead of a
	// timestamp which is what the JWT contains, need to do a bit of math.
	kv.cache.SetWithExpiration(apiKey, token, exp.Sub(kv.now()))
	return jwt, nil
}
