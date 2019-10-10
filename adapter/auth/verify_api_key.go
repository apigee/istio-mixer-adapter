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

/*
1. When an API Key check comes in, check a LRU cache.
2. If token is cached, initiate background check if token is expired, return cached token.
3. If token is not cached, check bad token cache, return invalid if present.
4. If token is in neither cache, make a synchronous request to Apigee to refresh it. Update good and bad caches.
*/

import (
	"bytes"
	contex "context"
	"encoding/json"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/context"
	"github.com/apigee/istio-mixer-adapter/adapter/util"
	"github.com/pkg/errors"
	"golang.org/x/sync/singleflight"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/pkg/cache"
)

const (
	verifyAPIKeyURL              = "/verifyApiKey"
	defaultCacheTTL              = 30 * time.Minute
	defaultCacheEvictionInterval = 10 * time.Second
	defaultMaxCachedEntries      = 10000
	defaultBadEntryCacheTTL      = 10 * time.Second
	parsedExpClaim               = "__apigeeParsedExp"
)

// keyVerifier encapsulates API key verification logic.
type keyVerifier interface {
	Verify(ctx context.Context, apiKey string) (map[string]interface{}, error)
}

type keyVerifierImpl struct {
	env        adapter.Env
	jwtMan     *jwtManager
	cache      cache.ExpiringCache
	now        func() time.Time
	client     *http.Client
	herdBuster singleflight.Group
	knownBad   cache.ExpiringCache
	checking   sync.Map
}

type keyVerifierOpts struct {
	CacheTTL              time.Duration
	CacheEvictionInterval time.Duration
	MaxCachedEntries      int
	Client                *http.Client
}

func newVerifier(env adapter.Env, jwtMan *jwtManager, opts keyVerifierOpts) keyVerifier {
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
		env:      env,
		jwtMan:   jwtMan,
		cache:    cache.NewLRU(opts.CacheTTL, opts.CacheEvictionInterval, int32(opts.MaxCachedEntries)),
		now:      time.Now,
		client:   opts.Client,
		knownBad: cache.NewLRU(defaultBadEntryCacheTTL, opts.CacheEvictionInterval, 100),
	}
}

func (kv *keyVerifierImpl) fetchToken(ctx context.Context, apiKey string) (map[string]interface{}, error) {
	if _, ok := kv.knownBad.Get(apiKey); ok {
		if kv.env.Logger().DebugEnabled() {
			kv.env.Logger().Debugf("fetchToken: known bad token: %s", util.Truncate(apiKey, 5))
		}
		return nil, ErrBadAuth
	}

	if kv.env.Logger().DebugEnabled() {
		kv.env.Logger().Debugf("fetchToken fetching: %s", util.Truncate(apiKey, 5))
	}
	verifyRequest := APIKeyRequest{
		APIKey: apiKey,
	}

	apiURL := *ctx.CustomerBase()
	apiURL.Path = path.Join(apiURL.Path, verifyAPIKeyURL)

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(verifyRequest)

	req, err := http.NewRequest(http.MethodPost, apiURL.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(ctx.Key(), ctx.Secret())

	resp, err := kv.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	apiKeyResp := APIKeyResponse{}
	json.NewDecoder(resp.Body).Decode(&apiKeyResp)

	token := apiKeyResp.Token
	if token == "" { // bad API Key
		kv.knownBad.Set(apiKey, apiKey)
		kv.cache.Remove(apiKey)
		return nil, ErrBadAuth
	}

	// no need to verify our own token, just parse it
	claims, err := kv.jwtMan.parseJWT(ctx, token, false)
	if err != nil {
		return nil, errors.Wrap(err, "parsing jwt")
	}

	exp, err := parseExp(claims)
	if err != nil {
		return nil, errors.Wrap(err, "bad exp")
	}
	claims[parsedExpClaim] = exp

	kv.cache.Set(apiKey, claims)
	kv.knownBad.Remove(apiKey)

	return claims, nil
}

func (kv *keyVerifierImpl) singleFetchToken(ctx context.Context, apiKey string) (map[string]interface{}, error) {
	// if kv.env.Logger().DebugEnabled() {
	// 	kv.env.Logger().Debugf("singleFetchToken: %s", util.Truncate(apiKey, 5))
	// }

	fetch := func() (interface{}, error) {
		return kv.fetchToken(ctx, apiKey)
	}
	res, err, _ := kv.herdBuster.Do(apiKey, fetch)
	// if kv.env.Logger().DebugEnabled() {
	// 	kv.env.Logger().Debugf("singleFetchToken: %s returning res: %#v, err: %#v", apiKey, res, err)
	// }
	if err != nil {
		return nil, err
	}

	return res.(map[string]interface{}), nil
}

// verify returns the list of claims that an API key has.
func (kv *keyVerifierImpl) Verify(ctx context.Context, apiKey string) (claims map[string]interface{}, err error) {
	if existing, ok := kv.cache.Get(apiKey); ok {
		claims = existing.(map[string]interface{})
	}

	// if token is expired, initiate a background refresh
	if claims != nil {
		exp := claims[parsedExpClaim].(time.Time)
		ttl := exp.Sub(kv.now())
		if ttl <= 0 { // refresh if possible
			if _, ok := kv.checking.Load(apiKey); !ok { // one refresh per apiKey at a time
				kv.checking.Store(apiKey, apiKey)

				// make the call with a backoff
				// will only call once and cancel loop if successful
				looper := util.Looper{
					Env:     kv.env,
					Backoff: util.DefaultExponentialBackoff(),
				}
				c, cancel := contex.WithCancel(contex.Background())
				work := func(c contex.Context) error {
					claims, err = kv.singleFetchToken(ctx, apiKey)
					if err != nil && err != ErrBadAuth {
						return err
					}
					cancel()
					kv.checking.Delete(apiKey)
					return nil
				}
				looper.Start(c, work, time.Minute, func(err error) error {
					kv.env.Logger().Errorf("Error refreshing token: %s", err)
					return nil
				})
			}
		}
		return claims, nil
	}

	// not found, force new request
	return kv.singleFetchToken(ctx, apiKey)
}
