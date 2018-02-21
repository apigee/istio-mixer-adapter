// build the protos
//go:generate $GOPATH/src/github.com/apigee/istio-mixer-adapter/bin/codegen.sh -f apigee/config/config.proto
//go:generate $GOPATH/src/github.com/apigee/istio-mixer-adapter/bin/codegen.sh -t template/analytics/template.proto

package apigee

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"github.com/apigee/istio-mixer-adapter/apigee/analytics"
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/config"
	"github.com/apigee/istio-mixer-adapter/apigee/quota"
	analyticsT "github.com/apigee/istio-mixer-adapter/template/analytics"
	quotaT "istio.io/istio/mixer/template/quota"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/pkg/status"
	"istio.io/istio/mixer/template/apikey"
	"istio.io/istio/mixer/template/logentry"
)

type (
	builder struct {
		adapterConfig *config.Params
	}

	handler struct {
		log          adapter.Logger
		apigeeBase   url.URL
		customerBase url.URL
		orgName      string
		envName      string
		key          string
		secret       string
	}
)

// Have handler implement Context

func (h *handler) Log() adapter.Logger {
	return h.log
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
	_ logentry.HandlerBuilder   = &builder{}
	_ quotaT.HandlerBuilder     = &builder{}
	_ analyticsT.HandlerBuilder = &builder{}
	_ apikey.HandlerBuilder     = &builder{}

	// Handler
	_ adapter.Handler    = &handler{}
	_ quotaT.Handler     = &handler{}
	_ logentry.Handler   = &handler{}
	_ analyticsT.Handler = &handler{}
	_ apikey.Handler     = &handler{}
)

////////////////// GetInfo //////////////////////////

// GetInfo returns the adapter.Info associated with this implementation.
func GetInfo() adapter.Info {
	return adapter.Info{
		Name:        "apigee",
		Impl:        "istio.io/istio/mixer/adapter/apigee",
		Description: "Apigee adapter",
		SupportedTemplates: []string{
			logentry.TemplateName,
			quotaT.TemplateName,
			analyticsT.TemplateName,
			apikey.TemplateName,
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

	return &handler{
		log:          env.Logger(),
		apigeeBase:   *apigeeBase,
		customerBase: *customerBase,
		orgName:      b.adapterConfig.OrgName,
		envName:      b.adapterConfig.EnvName,
		key:          b.adapterConfig.Key,
		secret:       b.adapterConfig.Secret,
	}, nil
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

func (*builder) SetLogEntryTypes(t map[string]*logentry.Type)  {}
func (*builder) SetQuotaTypes(map[string]*quotaT.Type)         {}
func (*builder) SetAnalyticsTypes(map[string]*analyticsT.Type) {}
func (*builder) SetApiKeyTypes(map[string]*apikey.Type)        {}

////////////////// adapter.Handler //////////////////////////

// Implements adapter.Handler
func (h *handler) Close() error { return nil }

func (h *handler) HandleLogEntry(ctx context.Context, logEntries []*logentry.Instance) error {
	// stubbed if we want to support
	//for _, logEntry := range logEntries {
	//	h.log.Infof("HandleLogEntry: %v\n", logEntry)
	//}
	return nil
}

// identify if auth available, call apid
// important: This assumes that the Auth is the same for all records!
func (h *handler) HandleAnalytics(ctx context.Context, instances []*analyticsT.Instance) error {

	var authContext *auth.Context
	var records []analytics.Record

	for _, inst := range instances {
		h.log.Infof("HandleAnalytics: %v\n", inst)

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

		// todo: this is nonsense, are we dealing with map[string]string or map[string]interface{}?
		var claims map[string]interface{}
		for k,v := range inst.ApiClaims {
			claims[k] = v
		}

		if authContext == nil {
			ac, err := auth.Authenticate(h, inst.ApiKey, claims)
			if err != nil {
				return err
			}
			authContext = &ac
		}

		records = append(records, record)
	}

	return analytics.SendRecords(authContext, records)
}

var productNameToDetails map[string]auth.ApiProductDetails

// todo: naive impl, optimize
// todo: paths can be wildcards
// todo: auth scopes
func resolveProducts(ac auth.Context, api, path string) []auth.ApiProductDetails {
	var result []auth.ApiProductDetails
	for _, name := range ac.APIProducts { // find product by name
		apiProduct := productNameToDetails[name]

		for _, attr := range apiProduct.Attributes { // find target services
			if attr.Name == "istio-services" {
				apiProductTargets := strings.Split(attr.Value, ",")
				for _, apiProductTarget := range apiProductTargets { // find target paths
					if apiProductTarget == api {
						validPaths := apiProduct.Resources
						for _, p := range validPaths {
							if p == path { // todo: probably need to do a substring match
								result = append(result, apiProduct)
							}
						}
					}
				}
			}
		}
	}
	return result
}

/*
if no jwt, get jwt from api key
for each product
	get product def: (products -> lookup by id)
	get services: attributes.name = "istio-services", "value" = comma-delimited service names
	for each service, if matches target service (api)
		for each apiResources, if matches request path
			get quota
		apply quota
	end
end

*/
// jwt: application_name -> product list
// jwt: application_name -> authorized scopes
// products -> lookup by id
// 		quota stuff
// 		apiResources (valid paths)
// 		required scopes

// 1. Authenticate & authorize path
// 2. Check quota
//		Get products
//		For each product, check paths
//		For each product w/ a matching path, apply quota?

// Istio doesn't understand our Quotas, so it cannot be allowed to cache
func (h *handler) HandleQuota(ctx context.Context, inst *quotaT.Instance, args adapter.QuotaArgs) (adapter.QuotaResult, error) {
	h.log.Infof("HandleQuota: %v args: %v\n", inst, args)

	// todo: skip < 0 to eliminate Istio prefetch returns (if it does that?)
	if args.QuotaAmount <= 0 {
		return adapter.QuotaResult{}, nil
	}

	path := inst.Dimensions["path"].(string)
	if path == "" {
		return adapter.QuotaResult{}, fmt.Errorf("path attribute required")
	}
	apiKey := inst.Dimensions["api_key"].(string)
	api := inst.Dimensions["api"].(string)

	h.log.Infof("api: %v, key: %v, path: %v", api, apiKey, path)

	// not sure about actual format
	claims, ok := inst.Dimensions["api_claims"].(map[string]interface{})
	if !ok {
		return adapter.QuotaResult{}, fmt.Errorf("wrong claims type: %v\n", inst.Dimensions["api_claims"])
	}

	authContext, err := auth.Authenticate(h, apiKey, claims)
	if err != nil {
		return adapter.QuotaResult{}, err
	}

	h.log.Infof("auth: %v", authContext)

	// checks auths, paths, scopes for relevant products
	products := resolveProducts(authContext, api, path)

	if len(products) == 0 { // no quotas, allow
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
	for _, product := range products {
		if product.QuotaLimit != "" {
			result, err := quota.Apply(authContext, product, args)
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

func (h *handler) HandleApiKey(ctx context.Context, inst *apikey.Instance) (adapter.CheckResult, error) {
	h.log.Infof("HandleApiKey: %v\n", inst)

	if inst.ApiKey == "" {
		h.log.Infof("missing api key")
		return adapter.CheckResult{
			Status: status.WithPermissionDenied("Unauthorized"),
		}, nil
	}

	authContext, err := auth.Authenticate(h, inst.ApiKey, nil)
	if err != nil {
		h.log.Errorf("authenticate err: %v", err)
		return adapter.CheckResult{
			Status: status.WithPermissionDenied(err.Error()),
		}, err
	}

	// todo: need to do better response for fail
	if authContext.ClientID == "" {
		h.log.Infof("authenticate failed")
		return adapter.CheckResult{
			Status: status.WithPermissionDenied("denied"),
		}, nil
	}

	return adapter.CheckResult{
		Status: status.OK,
	}, nil
}
