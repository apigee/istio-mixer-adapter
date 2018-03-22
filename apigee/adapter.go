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

// build the protos
//go:generate $GOPATH/src/github.com/apigee/istio-mixer-adapter/bin/codegen.sh -f apigee/config/config.proto
//go:generate $GOPATH/src/github.com/apigee/istio-mixer-adapter/bin/codegen.sh -t template/analytics/template.proto

package apigee

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/apigee/istio-mixer-adapter/apigee/analytics"
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/config"
	"github.com/apigee/istio-mixer-adapter/apigee/product"
	"github.com/apigee/istio-mixer-adapter/apigee/quota"
	analyticsT "github.com/apigee/istio-mixer-adapter/template/analytics"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/pkg/status"
	authT "istio.io/istio/mixer/template/authorization"
	quotaT "istio.io/istio/mixer/template/quota"
)

const (
	encodedClaimsKey   = "encoded_claims"
	apiClaimsAttribute = "api_claims"
	apiKeyAttribute    = "api_key"
	apiNameAttribute   = "api"
	pathAttribute      = "path"
)

type (
	builder struct {
		adapterConfig *config.Params
	}

	handler struct {
		env          adapter.Env
		apigeeBase   url.URL
		customerBase url.URL
		orgName      string
		envName      string
		key          string
		secret       string

		productMan   *product.Manager
		authMan      *auth.Manager
		analyticsMan *analytics.Manager
	}
)

// make handler implement Context...

func (h *handler) Log() adapter.Logger {
	return h.env.Logger()
}
func (h *handler) ApigeeBase() url.URL {
	return h.apigeeBase
}
func (h *handler) CustomerBase() url.URL {
	return h.customerBase
}
func (h *handler) Organization() string {
	return h.orgName
}
func (h *handler) Environment() string {
	return h.envName
}
func (h *handler) Key() string {
	return h.key
}
func (h *handler) Secret() string {
	return h.secret
}

// Ensure required interfaces are implemented.
var (
	// Builder
	_ adapter.HandlerBuilder    = &builder{}
	_ quotaT.HandlerBuilder     = &builder{}
	_ analyticsT.HandlerBuilder = &builder{}
	_ authT.HandlerBuilder      = &builder{}

	// Handler
	_ adapter.Handler    = &handler{}
	_ quotaT.Handler     = &handler{}
	_ analyticsT.Handler = &handler{}
	_ authT.Handler      = &handler{}
)

////////////////// GetInfo //////////////////////////

// GetInfo returns the adapter.Info associated with this implementation.
func GetInfo() adapter.Info {
	return adapter.Info{
		Name:        "apigee",
		Impl:        "istio.io/istio/mixer/adapter/apigee",
		Description: "Apigee adapter",
		SupportedTemplates: []string{
			analyticsT.TemplateName,
			authT.TemplateName,
			quotaT.TemplateName,
		},
		DefaultConfig: &config.Params{},
		NewBuilder:    func() adapter.HandlerBuilder { return &builder{} },
	}
}

////////////////// adapter.Builder //////////////////////////

// Implements adapter.HandlerBuilder
func (b *builder) SetAdapterConfig(cfg adapter.Config) {
	b.adapterConfig = cfg.(*config.Params)
}

// Implements adapter.HandlerBuilder
func (b *builder) Build(context context.Context, env adapter.Env) (adapter.Handler, error) {

	apigeeBase, err := url.Parse(b.adapterConfig.ApigeeBase)
	if err != nil {
		return nil, err
	}

	customerBase, err := url.Parse(b.adapterConfig.CustomerBase)
	if err != nil {
		return nil, err
	}

	pMan := product.NewManager(*customerBase, env.Logger(), env)
	aMan := auth.NewManager(env)
	anMan := analytics.NewManager(env, analytics.Options{
		BufferPath: b.adapterConfig.BufferPath,
	})

	h := &handler{
		env:          env,
		apigeeBase:   *apigeeBase,
		customerBase: *customerBase,
		orgName:      b.adapterConfig.OrgName,
		envName:      b.adapterConfig.EnvName,
		key:          b.adapterConfig.Key,
		secret:       b.adapterConfig.Secret,
		productMan:   pMan,
		authMan:      aMan,
		analyticsMan: anMan,
	}

	return h, nil
}

// Implements adapter.HandlerBuilder
func (b *builder) Validate() (errs *adapter.ConfigErrors) {

	if b.adapterConfig.ApigeeBase == "" {
		errs = errs.Append("apigee_base", fmt.Errorf("required"))
	} else if _, err := url.Parse(b.adapterConfig.ApigeeBase); err != nil {
		errs = errs.Append("apigee_base", fmt.Errorf("must be a valid url: %v", err))
	}

	if b.adapterConfig.CustomerBase == "" {
		errs = errs.Append("customer_base", fmt.Errorf("required"))
	} else if _, err := url.Parse(b.adapterConfig.CustomerBase); err != nil {
		errs = errs.Append("customer_base", fmt.Errorf("must be a valid url: %v", err))
	}

	if b.adapterConfig.OrgName == "" {
		errs = errs.Append("org_name", fmt.Errorf("required"))
	}

	if b.adapterConfig.EnvName == "" {
		errs = errs.Append("env_name", fmt.Errorf("required"))
	}

	if b.adapterConfig.Key == "" {
		errs = errs.Append("key", fmt.Errorf("required"))
	}

	if b.adapterConfig.Secret == "" {
		errs = errs.Append("secret", fmt.Errorf("required"))
	}

	return errs
}

func (*builder) SetAnalyticsTypes(map[string]*analyticsT.Type) {}
func (*builder) SetAuthorizationTypes(map[string]*authT.Type)  {}
func (*builder) SetQuotaTypes(map[string]*quotaT.Type)         {}

////////////////// adapter.Handler //////////////////////////

// Implements adapter.Handler
func (h *handler) Close() error {
	h.productMan.Close()
	h.authMan.Close()
	h.analyticsMan.Close()
	return nil
}

// Handle processing and delivery of Analytics to Apigee
func (h *handler) HandleAnalytics(ctx context.Context, instances []*analyticsT.Instance) error {

	var authContext *auth.Context
	var records []analytics.Record

	for _, inst := range instances {
		h.Log().Infof("HandleAnalytics: %#v", inst)

		record := analytics.Record{
			ClientReceivedStartTimestamp: analytics.TimeToUnix(inst.ClientReceivedStartTimestamp),
			ClientReceivedEndTimestamp:   analytics.TimeToUnix(inst.ClientReceivedStartTimestamp),
			ClientSentStartTimestamp:     analytics.TimeToUnix(inst.ClientSentStartTimestamp),
			ClientSentEndTimestamp:       analytics.TimeToUnix(inst.ClientSentEndTimestamp),
			TargetReceivedStartTimestamp: analytics.TimeToUnix(inst.TargetReceivedStartTimestamp),
			TargetReceivedEndTimestamp:   analytics.TimeToUnix(inst.TargetReceivedEndTimestamp),
			TargetSentStartTimestamp:     analytics.TimeToUnix(inst.TargetSentStartTimestamp),
			TargetSentEndTimestamp:       analytics.TimeToUnix(inst.TargetSentEndTimestamp),
			APIProxy:                     inst.ApiProxy,
			RequestURI:                   inst.RequestUri,
			RequestPath:                  inst.RequestPath,
			RequestVerb:                  inst.RequestVerb,
			ClientIP:                     inst.ClientIp.String(),
			UserAgent:                    inst.Useragent,
			ResponseStatusCode:           int(inst.ResponseStatusCode),
		}

		// important: This assumes that the Auth is the same for all records!
		if authContext == nil {
			ac, _ := h.authMan.Authenticate(h, inst.ApiKey, resolveClaims(h.Log(), inst.ApiClaims))
			// ignore error, take whatever we have
			authContext = &ac
		}

		records = append(records, record)
	}

	return h.analyticsMan.SendRecords(authContext, records)
}

// Handle Authentication and Authorization - NEVER RETURN ERROR!
func (h *handler) HandleAuthorization(ctx context.Context, inst *authT.Instance) (adapter.CheckResult, error) {
	h.Log().Infof("HandleAuthorization: Subject: %#v, Action: %#v", inst.Subject, inst.Action)

	// Mixer template says this is map[string]interface{}, but won't allow non-string values...
	// so, we'll have to take the entire properties value as the claims since we can't nest a map
	// also, need to convert to map[string]string for processing by resolveClaims()
	c := map[string]string{}
	for k, v := range inst.Subject.Properties {
		if vstr, ok := v.(string); ok {
			c[k] = vstr
		}
	}
	claims := resolveClaims(h.Log(), c)

	var apiKey string
	if k, ok := inst.Subject.Properties[apiKeyAttribute]; ok {
		apiKey = k.(string)
	}

	authContext, err := h.authMan.Authenticate(h, apiKey, claims)
	if err != nil {
		if _, ok := err.(*auth.NoAuthInfoError); ok {
			h.Log().Infof("authenticate err: %v", err)
			return adapter.CheckResult{
				Status: status.WithPermissionDenied(err.Error()),
			}, nil
		}
		h.Log().Errorf("authenticate err: %v", err)
		return adapter.CheckResult{
			Status: status.WithPermissionDenied(err.Error()),
		}, nil
	}

	if authContext.ClientID == "" {
		h.Log().Infof("authenticate failed")
		return adapter.CheckResult{
			Status: status.WithPermissionDenied("not authenticated"),
		}, nil
	}

	products := h.productMan.Resolve(authContext, inst.Action.Service, inst.Action.Path)
	if len(products) > 0 {
		return adapter.CheckResult{
			Status: status.OK,
		}, nil
	}

	return adapter.CheckResult{
		Status: status.WithPermissionDenied("not authorized"),
	}, nil
}

// Handle Quota checks - NEVER RETURN ERROR!
func (h *handler) HandleQuota(ctx context.Context, inst *quotaT.Instance, args adapter.QuotaArgs) (adapter.QuotaResult, error) {
	h.Log().Infof("HandleQuota: %#v args: %v", inst, args)

	// skip < 0 to eliminate Istio prefetch returns
	if args.QuotaAmount <= 0 {
		return adapter.QuotaResult{}, nil
	}

	path := inst.Dimensions[pathAttribute].(string)
	if path == "" {
		h.Log().Errorf("path attribute required")
		return adapter.QuotaResult{}, nil
	}
	apiKey := inst.Dimensions[apiKeyAttribute].(string)
	api := inst.Dimensions[apiNameAttribute].(string)

	h.Log().Infof("api: %v, key: %v, path: %v", api, apiKey, path)

	claims, ok := inst.Dimensions[apiClaimsAttribute].(map[string]string)
	if !ok {
		h.Log().Errorf("wrong claims type: %v", inst.Dimensions[apiClaimsAttribute])
		return adapter.QuotaResult{}, nil
	}

	authContext, err := h.authMan.Authenticate(h, apiKey, resolveClaims(h.Log(), claims))
	if err != nil {
		h.Log().Errorf("auth error: %v", err)
		return adapter.QuotaResult{}, nil
	}

	// get relevant products
	prods := h.productMan.Resolve(authContext, api, path)

	if len(prods) == 0 { // no quotas, allow
		return adapter.QuotaResult{
			Amount:        args.QuotaAmount,
			ValidDuration: 0,
		}, nil
	}

	// todo: support args.DeduplicationID
	// set QuotaAmount to 1 to eliminate Istio prefetch (also renders BestEffort meaningless)
	args.QuotaAmount = 1
	var exceeded int64
	var anyErr error
	for _, p := range prods {
		if p.QuotaLimit != "" {
			result, err := quota.Apply(authContext, p, args)
			if err != nil {
				anyErr = err
			} else if result.Exceeded > 0 {
				exceeded = result.Exceeded
			}
		}
	}
	if anyErr != nil {
		h.Log().Errorf("quota error: %v", anyErr)
		return adapter.QuotaResult{}, nil
	}
	if exceeded > 0 {
		return adapter.QuotaResult{
			Status:        status.OK,
			ValidDuration: 0,
			Amount:        0,
		}, nil
	}

	return adapter.QuotaResult{
		Status:        status.OK,
		ValidDuration: 0,
		Amount:        args.QuotaAmount,
	}, nil
}

// resolveClaims ensures that jwt auth claims are properly populated from an
// incoming map of potential claims values--including extraneous filtering.
// For future compatibility with Istio, also checks for "encoded_claims" - a
// base64 string value containing all claims in a JSON format. This is used
// as the request.auth.claims attribute has not yet been defined in Mixer.
// see: https://github.com/istio/istio/issues/3194
func resolveClaims(log adapter.Logger, claimsIn map[string]string) map[string]interface{} {
	var claims = map[string]interface{}{}
	for _, k := range auth.AllValidClaims {
		if v, ok := claimsIn[k]; ok {
			claims[k] = v
		}
	}
	if len(claims) > 0 {
		return claims
	}

	if encoded, ok := claimsIn[encodedClaimsKey]; ok {
		if encoded == "" {
			return claims
		}

		if len(encoded)%4 != 0 {
			// Encoded base64 must always have a length divisble by 4, or DecodeString
			// will fail. Pad with "=" to make it happy.
			encoded += strings.Repeat("=", 4-(len(encoded)%4))
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err == nil {
			err = json.Unmarshal(decoded, &claims)
		}

		if err != nil {
			log.Errorf("error resolving claims: %v, data: %v", err, encoded)
			return claims
		}

		err = json.Unmarshal(decoded, &claims)
	}

	return claims
}
