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
	"fmt"
	"net/url"

	"github.com/apigee/istio-mixer-adapter/apigee/analytics"
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/config"
	"github.com/apigee/istio-mixer-adapter/apigee/product"
	"github.com/apigee/istio-mixer-adapter/apigee/quota"
	analyticsT "github.com/apigee/istio-mixer-adapter/template/analytics"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/pkg/status"
	"istio.io/istio/mixer/template/apikey"
	authT "istio.io/istio/mixer/template/authorization"
	quotaT "istio.io/istio/mixer/template/quota"
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
		analyticsMan analytics.Manager
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
	_ apikey.HandlerBuilder     = &builder{}
	_ authT.HandlerBuilder      = &builder{}

	// Handler
	_ adapter.Handler    = &handler{}
	_ quotaT.Handler     = &handler{}
	_ analyticsT.Handler = &handler{}
	_ apikey.Handler     = &handler{}
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
			apikey.TemplateName,
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
	anMan := analytics.NewManager(env)

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
func (*builder) SetApiKeyTypes(map[string]*apikey.Type)        {}
func (*builder) SetAuthorizationTypes(map[string]*authT.Type)  {}
func (*builder) SetQuotaTypes(map[string]*quotaT.Type)         {}

////////////////// adapter.Handler //////////////////////////

// Implements adapter.Handler
func (h *handler) Close() error {
	h.productMan.Close()
	h.authMan.Close()
	if h.analyticsMan != nil {
		h.analyticsMan.Close()
	}
	return nil
}

// important: This assumes that the Auth is the same for all records!
func (h *handler) HandleAnalytics(ctx context.Context, instances []*analyticsT.Instance) error {

	var authContext *auth.Context
	var records []analytics.Record

	for _, inst := range instances {
		h.Log().Infof("HandleAnalytics: %v\n", inst)

		record := analytics.Record{
			ClientReceivedStartTimestamp: inst.ClientReceivedStartTimestamp.Unix(),
			ClientReceivedEndTimestamp:   inst.ClientReceivedStartTimestamp.Unix(),
			ClientSentStartTimestamp:     inst.ClientSentStartTimestamp.Unix(),
			ClientSentEndTimestamp:       inst.ClientSentEndTimestamp.Unix(),
			TargetReceivedStartTimestamp: inst.TargetReceivedStartTimestamp.Unix(),
			TargetReceivedEndTimestamp:   inst.TargetReceivedEndTimestamp.Unix(),
			TargetSentStartTimestamp:     inst.TargetSentStartTimestamp.Unix(),
			TargetSentEndTimestamp:       inst.TargetSentEndTimestamp.Unix(),
			APIProxy:                     inst.ApiProxy,
			RequestURI:                   inst.RequestUri,
			RequestPath:                  inst.RequestPath,
			RequestVerb:                  inst.RequestVerb,
			ClientIP:                     inst.ClientIp.String(),
			UserAgent:                    inst.Useragent,
			ResponseStatusCode:           int(inst.ResponseStatusCode),
		}

		if authContext == nil {
			ac, _ := h.authMan.Authenticate(h, inst.ApiKey, convertClaims(inst.ApiClaims))
			// ignore error, take whatever we have
			authContext = &ac
		}

		records = append(records, record)
	}

	return h.analyticsMan.SendRecords(authContext, records)
}

func (h *handler) HandleApiKey(ctx context.Context, inst *apikey.Instance) (adapter.CheckResult, error) {
	h.Log().Infof("HandleApiKey: %v\n", inst)

	if inst.ApiKey == "" || inst.Api == "" || inst.ApiOperation == "" {
		h.Log().Infof("missing properties: %v", inst)
		return adapter.CheckResult{
			Status: status.WithPermissionDenied("missing authentication"),
		}, nil
	}

	authContext, err := h.authMan.Authenticate(h, inst.ApiKey, nil)
	if err != nil {
		h.Log().Errorf("authenticate err: %v", err)
		return adapter.CheckResult{
			Status: status.WithPermissionDenied(err.Error()),
		}, err
	}

	if authContext.ClientID == "" {
		h.Log().Infof("authenticate failed")
		return adapter.CheckResult{
			Status: status.WithPermissionDenied("authentication failed"),
		}, nil
	}

	return h.authorize(authContext, inst.Api, inst.ApiOperation)
}

func (h *handler) HandleAuthorization(ctx context.Context, inst *authT.Instance) (adapter.CheckResult, error) {
	h.Log().Infof("HandleAuthorization: %v\n", inst)

	if inst.Subject == nil || inst.Subject.Properties == nil || inst.Action.Service == "" || inst.Action.Path == "" {
		h.Log().Infof("missing properties: %v", inst)
		return adapter.CheckResult{
			Status: status.WithPermissionDenied("missing authentication"),
		}, nil
	}

	claims, ok := inst.Subject.Properties["claims"].(map[string]string)
	if !ok {
		return adapter.CheckResult{}, fmt.Errorf("wrong claims type: %v", inst.Subject.Properties["claims"])
	}

	authContext, err := h.authMan.Authenticate(h, "", convertClaims(claims))
	if err != nil {
		h.Log().Errorf("authenticate err: %v", err)
		return adapter.CheckResult{
			Status: status.WithPermissionDenied(err.Error()),
		}, err
	}

	if authContext.ClientID == "" {
		h.Log().Infof("authenticate failed")
		return adapter.CheckResult{
			Status: status.WithPermissionDenied("not authenticated"),
		}, nil
	}

	return h.authorize(authContext, inst.Action.Service, inst.Action.Path)
}

// authorize: check service, path, scopes
func (h *handler) authorize(authContext auth.Context, service, path string) (adapter.CheckResult, error) {

	products := h.productMan.Resolve(authContext, service, path)
	if len(products) > 0 {
		return adapter.CheckResult{
			Status: status.OK,
		}, nil
	}

	return adapter.CheckResult{
		Status: status.WithPermissionDenied("not authorized"),
	}, nil
}

// Istio doesn't understand our Quotas, so it cannot be allowed to cache
func (h *handler) HandleQuota(ctx context.Context, inst *quotaT.Instance, args adapter.QuotaArgs) (adapter.QuotaResult, error) {
	h.Log().Infof("HandleQuota: %v args: %v\n", inst, args)

	// skip < 0 to eliminate Istio prefetch returns
	if args.QuotaAmount <= 0 {
		return adapter.QuotaResult{}, nil
	}

	path := inst.Dimensions["path"].(string)
	if path == "" {
		return adapter.QuotaResult{}, fmt.Errorf("path attribute required")
	}
	apiKey := inst.Dimensions["api_key"].(string)
	api := inst.Dimensions["api"].(string)

	h.Log().Infof("api: %v, key: %v, path: %v", api, apiKey, path)

	// not sure about actual format
	claims, ok := inst.Dimensions["api_claims"].(map[string]string)
	if !ok {
		return adapter.QuotaResult{}, fmt.Errorf("wrong claims type: %v", inst.Dimensions["api_claims"])
	}

	authContext, err := h.authMan.Authenticate(h, apiKey, convertClaims(claims))
	if err != nil {
		return adapter.QuotaResult{}, err
	}

	h.Log().Infof("auth: %v", authContext)

	// get relevant products
	prods := h.productMan.Resolve(authContext, api, path)

	if len(prods) == 0 { // no quotas, allow
		return adapter.QuotaResult{
			Amount:        args.QuotaAmount,
			ValidDuration: 0,
		}, nil
	}

	// todo: support args.DeduplicationID
	// todo: converting our quotas to Istio is weird, anything better?
	// todo: set QuotaAmount to 1 to eliminate Istio prefetch (also renders BestEffort meaningless)
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
		return adapter.QuotaResult{}, anyErr
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

func convertClaims(claims map[string]string) map[string]interface{} {
	var claimsOut map[string]interface{}
	for k, v := range claims {
		claimsOut[k] = v
	}
	return claimsOut
}
