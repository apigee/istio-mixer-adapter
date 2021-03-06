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
//go:generate $GOPATH/src/github.com/apigee/istio-mixer-adapter/bin/codegen.sh -f adapter/config/config.proto
//go:generate $GOPATH/src/github.com/apigee/istio-mixer-adapter/bin/codegen.sh -t template/analytics/template.proto

package adapter

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/analytics"
	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"github.com/apigee/istio-mixer-adapter/adapter/config"
	"github.com/apigee/istio-mixer-adapter/adapter/product"
	"github.com/apigee/istio-mixer-adapter/adapter/quota"
	"github.com/apigee/istio-mixer-adapter/adapter/util"
	analyticsT "github.com/apigee/istio-mixer-adapter/template/analytics"
	pbtypes "github.com/gogo/protobuf/types"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/pkg/status"
	authT "istio.io/istio/mixer/template/authorization"
)

const (
	jsonClaimsKey             = "json_claims"
	apiKeyAttribute           = "api_key"
	gatewaySource             = "istio"
	tempDirMode               = os.FileMode(0700)
	certPollInterval          = 0 // jwt validation not currently needed
	minSaaSCollectionInterval = 2 * time.Minute

	// if APIGEE_ORG or APIGEE_ENV are set in os env, only those values can be valid in the configuration
	orgEnvKey = "APIGEE_ORG"
	envEnvKey = "APIGEE_ENV"
)

type (
	builder struct {
		handlerConfig *config.Params
	}

	handler struct {
		env            adapter.Env
		apigeeBase     *url.URL
		customerBase   *url.URL
		orgName        string
		envName        string
		key            string
		secret         string
		apiKeyClaimKey string

		productMan   *product.Manager
		authMan      *auth.Manager
		analyticsMan analytics.Manager
		quotaMan     *quota.Manager
	}
)

// make handler implement Context...

func (h *handler) Log() adapter.Logger {
	return h.env.Logger()
}
func (h *handler) ApigeeBase() *url.URL {
	return h.apigeeBase
}
func (h *handler) CustomerBase() *url.URL {
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
	_ analyticsT.HandlerBuilder = &builder{}
	_ authT.HandlerBuilder      = &builder{}

	// Handler
	_ adapter.Handler    = &handler{}
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
		},
		DefaultConfig: &config.Params{
			ClientTimeout: pbtypes.DurationProto(30 * time.Second),
			TempDir:       "/tmp/apigee-istio",
			Products: &config.ParamsProductOptions{
				RefreshRate: pbtypes.DurationProto(2 * time.Minute),
			},
			Analytics: &config.ParamsAnalyticsOptions{
				LegacyEndpoint:     false,
				FileLimit:          1024,
				SendChannelSize:    10,
				CollectionInterval: pbtypes.DurationProto(2 * time.Minute),
			},
			Auth: &config.ParamsAuthOptions{
				ApiKeyCacheDuration: pbtypes.DurationProto(30 * time.Minute),
			},
		},
		NewBuilder: func() adapter.HandlerBuilder { return &builder{} },
	}
}

////////////////// util //////////////////////////

// timeToUnix converts a time to a UNIX timestamp in milliseconds.
func timeToUnix(t time.Time) int64 {
	return t.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

// toDuration converts protobuf Duration to time.Duration.
func toDuration(durationProto *pbtypes.Duration) time.Duration {
	duration, err := pbtypes.DurationFromProto(durationProto)
	if err != nil {
		panic(fmt.Sprintf("invalid Duration proto: %v", err))
	}
	return duration
}

////////////////// adapter.Builder //////////////////////////

// Implements adapter.HandlerBuilder
func (b *builder) SetAdapterConfig(cfg adapter.Config) {
	b.handlerConfig = cfg.(*config.Params)
}

// Implements adapter.HandlerBuilder
func (b *builder) Build(context context.Context, env adapter.Env) (adapter.Handler, error) {
	redacts := []interface{}{
		b.handlerConfig.Key,
		b.handlerConfig.Secret,
	}
	redactedConfig := util.SprintfRedacts(redacts, "%#v", *b.handlerConfig)
	env.Logger().Infof("Handler config: %#v", redactedConfig)

	envOrg := os.Getenv(orgEnvKey)
	if envOrg != "" && envOrg != b.handlerConfig.OrgName {
		return nil, fmt.Errorf("invalid tenant org: %s", b.handlerConfig.OrgName)
	}

	envEnv := os.Getenv(envEnvKey)
	if envEnv != "" && envEnv != b.handlerConfig.EnvName {
		return nil, fmt.Errorf("invalid tenant env: %s", b.handlerConfig.EnvName)
	}

	// apigeeBase not required for hybrid, ignore if the hybrid config is set
	var apigeeBase *url.URL
	hybridConfigFile := b.handlerConfig.HybridConfig
	if hybridConfigFile == "" {
		var err error
		if apigeeBase, err = url.Parse(b.handlerConfig.ApigeeBase); err != nil {
			return nil, err
		}
	}

	customerBase, err := url.Parse(b.handlerConfig.CustomerBase)
	if err != nil {
		return nil, err
	}

	analyticsDir := filepath.Join(b.handlerConfig.TempDir, "analytics")
	if err := os.MkdirAll(analyticsDir, tempDirMode); err != nil {
		return nil, err
	}

	if b.handlerConfig.ClientTimeout == nil || toDuration(b.handlerConfig.ClientTimeout) < time.Second {
		return nil, fmt.Errorf("ClientTimeout must be > 1")
	}
	tr := http.DefaultTransport
	if b.handlerConfig.AllowUnverifiedSSLCert {
		tr = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: b.handlerConfig.AllowUnverifiedSSLCert},
		}
	}
	httpClient := &http.Client{
		Timeout:   toDuration(b.handlerConfig.ClientTimeout),
		Transport: tr,
	}

	productMan, err := product.NewManager(env, product.Options{
		Client:      httpClient,
		BaseURL:     customerBase,
		RefreshRate: toDuration(b.handlerConfig.Products.RefreshRate),
		Key:         b.handlerConfig.Key,
		Secret:      b.handlerConfig.Secret,
	})
	if err != nil {
		return nil, err
	}

	authMan, err := auth.NewManager(env, auth.Options{
		PollInterval:        certPollInterval,
		Client:              httpClient,
		APIKeyCacheDuration: toDuration(b.handlerConfig.Auth.ApiKeyCacheDuration),
	})
	if err != nil {
		return nil, err
	}

	quotaMan, err := quota.NewManager(env, quota.Options{
		BaseURL: customerBase,
		Client:  httpClient,
		Key:     b.handlerConfig.Key,
		Secret:  b.handlerConfig.Secret,
	})
	if err != nil {
		return nil, err
	}

	analyticsMan, err := analytics.NewManager(env, analytics.Options{
		LegacyEndpoint:     b.handlerConfig.Analytics.LegacyEndpoint,
		BufferPath:         analyticsDir,
		StagingFileLimit:   int(b.handlerConfig.Analytics.FileLimit),
		BaseURL:            apigeeBase,
		Key:                b.handlerConfig.Key,
		Secret:             b.handlerConfig.Secret,
		Client:             httpClient,
		SendChannelSize:    int(b.handlerConfig.Analytics.SendChannelSize),
		HybridConfigFile:   hybridConfigFile,
		CollectionInterval: toDuration(b.handlerConfig.Analytics.CollectionInterval),
	})
	env.Logger().Infof("new manager: %#v", analyticsMan)
	if err != nil {
		return nil, err
	}

	h := &handler{
		env:            env,
		apigeeBase:     apigeeBase,
		customerBase:   customerBase,
		orgName:        b.handlerConfig.OrgName,
		envName:        b.handlerConfig.EnvName,
		key:            b.handlerConfig.Key,
		secret:         b.handlerConfig.Secret,
		productMan:     productMan,
		authMan:        authMan,
		analyticsMan:   analyticsMan,
		quotaMan:       quotaMan,
		apiKeyClaimKey: b.handlerConfig.Auth.ApiKeyClaim,
	}

	return h, nil
}

// Implements adapter.HandlerBuilder
func (b *builder) Validate() (errs *adapter.ConfigErrors) {

	if os.Getenv(orgEnvKey) != "" && os.Getenv(orgEnvKey) != b.handlerConfig.OrgName {
		errs = errs.Append("org_name", fmt.Errorf("org is restricted, %s is invalid", b.handlerConfig.OrgName))
	}

	if os.Getenv(envEnvKey) != "" && os.Getenv(envEnvKey) != b.handlerConfig.EnvName {
		errs = errs.Append("env_name", fmt.Errorf("env is restricted, %s is invalid", b.handlerConfig.EnvName))
	}

	if b.handlerConfig.HybridConfig == "" {
		if b.handlerConfig.ApigeeBase == "" {
			errs = errs.Append("apigee_base or hybrid_config", fmt.Errorf("required"))
		} else if _, err := url.ParseRequestURI(b.handlerConfig.ApigeeBase); err != nil {
			errs = errs.Append("apigee_base", fmt.Errorf("must be a valid url: %v", err))
		}

		if b.handlerConfig.Analytics != nil && b.handlerConfig.Analytics.CollectionInterval != nil {
			ci := toDuration(b.handlerConfig.Analytics.CollectionInterval)
			if ci < minSaaSCollectionInterval {
				errs = errs.Append("analytics/collection_interval", fmt.Errorf("must be a greater than: %s", minSaaSCollectionInterval))
			}
		}

	} else { // isHybrid
		info, err := os.Stat(b.handlerConfig.HybridConfig)
		if os.IsNotExist(err) || info.IsDir() {
			errs = errs.Append("hybrid_config", fmt.Errorf("%s is not a valid file", b.handlerConfig.HybridConfig))
		}
	}

	if b.handlerConfig.CustomerBase == "" {
		errs = errs.Append("customer_base", fmt.Errorf("required"))
	} else if _, err := url.ParseRequestURI(b.handlerConfig.CustomerBase); err != nil {
		errs = errs.Append("customer_base", fmt.Errorf("must be a valid url: %v", err))
	}

	if b.handlerConfig.OrgName == "" {
		errs = errs.Append("org_name", fmt.Errorf("required"))
	}

	if b.handlerConfig.EnvName == "" {
		errs = errs.Append("env_name", fmt.Errorf("required"))
	}

	if b.handlerConfig.Key == "" {
		errs = errs.Append("key", fmt.Errorf("required"))
	}

	if b.handlerConfig.Secret == "" {
		errs = errs.Append("secret", fmt.Errorf("required"))
	}

	return errs
}

func (*builder) SetAnalyticsTypes(map[string]*analyticsT.Type) {}
func (*builder) SetAuthorizationTypes(map[string]*authT.Type)  {}

////////////////// adapter.Handler //////////////////////////

// Implements adapter.Handler
func (h *handler) Close() error {
	h.productMan.Close()
	h.quotaMan.Close()
	h.authMan.Close()
	h.analyticsMan.Close()
	return nil
}

// Handle processing and delivery of Analytics to Apigee
func (h *handler) HandleAnalytics(ctx context.Context, instances []*analyticsT.Instance) error {
	if len(instances) == 0 {
		return nil
	}
	h.Log().Debugf("HandleAnalytics: %d instances", len(instances))

	var authContext *auth.Context
	var records = make([]analytics.Record, 0, len(instances))

	for _, inst := range instances {
		record := analytics.Record{
			ClientReceivedStartTimestamp: timeToUnix(inst.ClientReceivedStartTimestamp),
			ClientReceivedEndTimestamp:   timeToUnix(inst.ClientReceivedStartTimestamp),
			ClientSentStartTimestamp:     timeToUnix(inst.ClientSentStartTimestamp),
			ClientSentEndTimestamp:       timeToUnix(inst.ClientSentEndTimestamp),
			TargetReceivedStartTimestamp: timeToUnix(inst.TargetReceivedStartTimestamp),
			TargetReceivedEndTimestamp:   timeToUnix(inst.TargetReceivedEndTimestamp),
			TargetSentStartTimestamp:     timeToUnix(inst.TargetSentStartTimestamp),
			TargetSentEndTimestamp:       timeToUnix(inst.TargetSentEndTimestamp),
			APIProxy:                     inst.ApiProxy,
			RequestURI:                   inst.RequestUri,
			RequestVerb:                  inst.RequestVerb,
			ClientIP:                     inst.ClientIp.String(),
			UserAgent:                    inst.Useragent,
			ResponseStatusCode:           int(inst.ResponseStatusCode),
			GatewaySource:                gatewaySource,
		}

		// Apigee expects RequestURI to include query parameters. Istio's request.path matches this.
		// However, Apigee expects RequestPath exclude query parameters and there is no corresponding
		// Istio attribute. Thus, we need to drop the query params from request.path for RequestPath.
		splits := strings.Split(inst.RequestPath, "?")
		if len(splits) > 0 {
			record.RequestPath = splits[0]
		} else {
			record.RequestPath = "/"
		}

		// important: This assumes that the Auth is the same for all records!
		if authContext == nil {
			ac, _ := h.authMan.Authenticate(h, inst.ApiKey, h.resolveClaims(inst.ApiClaims), h.apiKeyClaimKey)
			// ignore error, take whatever we have
			authContext = ac
		}

		records = append(records, record)
	}

	return h.analyticsMan.SendRecords(authContext, records)
}

// static values for HandleAuthorization...
var quotaArgs = adapter.QuotaArgs{QuotaAmount: 1}
var checkResultNil = adapter.CheckResult{}
var checkResultOk = adapter.CheckResult{Status: status.OK}
var checkResultOkWithQuotas = adapter.CheckResult{
	Status:        status.OK,
	ValidUseCount: 1, // call adapter each time to ensure quotas are applied
}
var checkResultQuotaExceeded = adapter.CheckResult{
	Status:        status.WithResourceExhausted("quota exceeded"),
	ValidUseCount: 1, // call adapter each time to ensure quotas are applied
}
var checkResultNotAuthorized = adapter.CheckResult{Status: status.WithPermissionDenied("permission denied")}
var checkResultMissingAuth = adapter.CheckResult{Status: status.WithUnauthenticated("missing authentication")}

// Handle Authentication, Authorization, and Quotas
func (h *handler) HandleAuthorization(ctx context.Context, inst *authT.Instance) (adapter.CheckResult, error) {
	if h.Log().DebugEnabled() {
		redacts := []interface{}{
			inst.Subject.Properties[apiKeyAttribute],
			inst.Subject.Properties[jsonClaimsKey],
		}
		redactedSub := util.SprintfRedacts(redacts, "%#v", *inst.Subject)
		h.Log().Debugf("HandleAuthorization: Subject: %s, Action: %#v", redactedSub, *inst.Action)
	}

	claims := h.resolveClaimsInterface(inst.Subject.Properties)
	apiKey, _ := inst.Subject.Properties[apiKeyAttribute].(string)
	authContext, err := h.authMan.Authenticate(h, apiKey, claims, h.apiKeyClaimKey)
	if err != nil {
		h.Log().Debugf("authenticate err: %v", err)
		switch err {
		case auth.ErrNoAuth:
			return checkResultMissingAuth, nil
		case auth.ErrBadAuth:
			return checkResultNotAuthorized, nil
		default:
			return adapter.CheckResult{
				Status: status.WithPermissionDenied(err.Error()),
			}, nil
		}
	}

	products := h.productMan.Resolve(authContext, inst.Action.Service, inst.Action.Path)
	if len(products) == 0 {
		return checkResultNotAuthorized, nil
	}

	var anyQuotas, exceeded bool
	var anyError error
	// apply to all matching products
	for _, p := range products {
		if p.QuotaLimitInt > 0 {
			anyQuotas = true
			result, err := h.quotaMan.Apply(authContext, p, quotaArgs)
			if err != nil {
				anyError = err
			} else if result.Exceeded > 0 {
				exceeded = true
			}
		}
	}
	if anyError != nil {
		h.Log().Debugf("authenticate err: %v", anyError)
		return checkResultNil, anyError
	}
	if exceeded {
		h.Log().Debugf("quota exceeded: %v", err)
		return checkResultQuotaExceeded, nil
	}

	h.Log().Debugf("request authorized")

	var okResult adapter.CheckResult
	if anyQuotas {
		okResult = checkResultOkWithQuotas
	} else {
		okResult = checkResultOk
	}
	return okResult, nil
}

// resolveClaims ensures that jwt auth claims are properly populated from an
// incoming map of potential claims values--including extraneous filtering.
func (h *handler) resolveClaims(claimsIn map[string]string) map[string]interface{} {
	var claims = map[string]interface{}{}

	if encoded, ok := claimsIn[jsonClaimsKey]; ok && encoded != "" {
		err := json.Unmarshal([]byte(encoded), &claims)
		if err != nil {
			h.Log().Errorf("error resolving %s claims: %v, data: %v", jsonClaimsKey, err, encoded)
		}
	}

	for _, k := range auth.AllValidClaims {
		if v, ok := claimsIn[k]; ok {
			claims[k] = v
		}
	}

	if claimsIn[h.apiKeyClaimKey] != "" {
		claims[h.apiKeyClaimKey] = claimsIn[h.apiKeyClaimKey]
	}

	return claims
}

// convert map[string]interface{} to string[string]string so we can call real resolveClaims
func (h *handler) resolveClaimsInterface(claimsIn map[string]interface{}) map[string]interface{} {
	c := make(map[string]string, len(claimsIn))
	for k, v := range claimsIn {
		if s, ok := v.(string); ok {
			c[k] = s
		}
	}
	return h.resolveClaims(c)
}
