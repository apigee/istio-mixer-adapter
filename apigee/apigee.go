package apigee // import "github.com/apigee/istio-mixer-adapter/apigee"

import (
	"context"
	"fmt"
	"net/url"
	"time"

	rpc "github.com/googleapis/googleapis/google/rpc"
	"github.com/apigee/istio-mixer-adapter/apigee/analytics"
	"github.com/apigee/istio-mixer-adapter/apigee/config"
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	authT "github.com/apigee/istio-mixer-adapter/template/auth"
	analyticsT "github.com/apigee/istio-mixer-adapter/template/analytics"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/template/logentry"
	"istio.io/istio/mixer/template/quota"
	"net"
)

////////////////// GetInfo //////////////////////////

// returns the adapter.Info metadata during mixer initialization
func GetInfo() adapter.Info {
	return adapter.Info{
		Name:        "apigee",
		Impl:        "istio.io/istio/mixer/adapter/apigee",
		Description: "Apigee adapter",
		SupportedTemplates: []string{
			logentry.TemplateName,
			quota.TemplateName,
			authT.TemplateName,
			analyticsT.TemplateName,
		},
		NewBuilder: createBuilder,
		DefaultConfig: &config.Params{
			ApidBase: "http://apid:9000/",
		},
	}
}

var createBuilder = func() adapter.HandlerBuilder {
	return &builder{}
}

////////////////// Builder //////////////////////////

type builder struct {
	adapterConfig *config.Params
}

// force interface checks at compile time
var _ adapter.HandlerBuilder = (*builder)(nil)
var _ authT.HandlerBuilder = (*builder)(nil)
var _ logentry.HandlerBuilder = (*builder)(nil)
var _ quota.HandlerBuilder = (*builder)(nil)
var _ analyticsT.HandlerBuilder = (*builder)(nil)

// adapter.HandlerBuilder
func (b *builder) SetAdapterConfig(cfg adapter.Config) {
	b.adapterConfig = cfg.(*config.Params)
}

// adapter.HandlerBuilder
func (b *builder) Build(context context.Context, env adapter.Env) (adapter.Handler, error) {

	apidBase, err := url.Parse(b.adapterConfig.ApidBase)
	if err != nil {
		return nil, err
	}

	return &handler{
		apidBase:    *apidBase,
		orgName:     b.adapterConfig.OrgName,
		envName:     b.adapterConfig.EnvName,
		env:         env,
	}, nil
}

// adapter.HandlerBuilder
func (b *builder) Validate() (ce *adapter.ConfigErrors) {

	fmt.Printf("Validate: %v\n", b.adapterConfig)

	if b.adapterConfig.ApidBase == "" {
		ce = ce.Append("apid_base", fmt.Errorf("apid_base is required"))
	}

	if _, err := url.Parse(b.adapterConfig.ApidBase); err != nil {
		ce = ce.Append("apid_base", fmt.Errorf("apid_base must be a valid url: %v", err))
	}

	if b.adapterConfig.OrgName == "" {
		ce = ce.Append("org_name", fmt.Errorf("org_name is required"))
	}

	if b.adapterConfig.EnvName == "" {
		ce = ce.Append("env_name", fmt.Errorf("env_name is required"))
	}

	return ce
}

func (*builder) SetAuthTypes(t map[string]*authT.Type) {}
func (*builder) SetLogEntryTypes(t map[string]*logentry.Type) {}
func (*builder) SetQuotaTypes(map[string]*quota.Type) {}
func (*builder) SetAnalyticsTypes(map[string]*analyticsT.Type) {}

////////////////// Handler //////////////////////////

type handler struct {
	apidBase    url.URL
	orgName     string
	envName     string
	env         adapter.Env
}

// force interface checks at compile time
var _ adapter.Handler = (*handler)(nil)
var _ authT.Handler = (*handler)(nil)
var _ quota.Handler = (*handler)(nil)
var _ logentry.Handler = (*handler)(nil)
var _ analyticsT.Handler = (*handler)(nil)

// adapter.Handler
func (h *handler) Close() error { return nil }

func (h *handler) HandleLogEntry(ctx context.Context, logEntries []*logentry.Instance) error {
	log := h.env.Logger()
	for _, logEntry := range logEntries {
		log.Infof("HandleLogEntry: %v\n", logEntry)
	}
	return nil
}

func (h *handler) HandleAnalytics(ctx context.Context, instances []*analyticsT.Instance) error {
	log := h.env.Logger()

	for _, inst := range instances {
		log.Infof("HandleAnalytics: %v\n", inst)

		clientIP := ""
		rawIP := inst.ClientIp.([]uint8)
		if len(rawIP) == net.IPv4len || len(rawIP) == net.IPv6len {
			ip := net.IP(rawIP)
			if !ip.IsUnspecified() {
				clientIP = ip.String()
			}
		}

		record := &analytics.Record{
			ClientReceivedStartTimestamp: analytics.TimeToUnix(inst.ClientReceivedStartTimestamp),
			ClientReceivedEndTimestamp:   analytics.TimeToUnix(inst.ClientReceivedStartTimestamp),
			ClientSentStartTimestamp:     analytics.TimeToUnix(inst.ClientSentStartTimestamp),
			ClientSentEndTimestamp:       analytics.TimeToUnix(inst.ClientSentEndTimestamp),
			TargetReceivedStartTimestamp: analytics.TimeToUnix(inst.TargetReceivedStartTimestamp),
			TargetReceivedEndTimestamp:   analytics.TimeToUnix(inst.TargetReceivedEndTimestamp),
			TargetSentStartTimestamp:     analytics.TimeToUnix(inst.TargetSentStartTimestamp),
			TargetSentEndTimestamp:       analytics.TimeToUnix(inst.TargetSentEndTimestamp),
			APIProxy:                     inst.Apigeeproxy,
			APIProxyRevision:			  int(inst.ApigeeproxyRevision), // todo: is this available?
			RequestURI:                   inst.RequestUri,
			RequestPath:                  inst.RequestPath,
			RequestVerb:                  inst.RequestVerb,
			ClientIP:                     clientIP,
			UserAgent:                    inst.Useragent,
			ResponseStatusCode:           int(inst.ResponseStatusCode),
		}

		verifyApiKeyRequest := auth.VerifyApiKeyRequest{
			Key:              inst.Apikey,
			OrganizationName: h.orgName,
			UriPath:          inst.RequestPath,
			ApiProxyName:	  inst.Apigeeproxy,
			EnvironmentName:  h.envName,
		}
		// todo: ignoring fail & err results for now
		success, _, _ := auth.VerifyAPIKey(h.env, h.apidBase, verifyApiKeyRequest)
		if success != nil {
			// todo: org isn't being returned by apid, why?
			success.Organization = h.orgName
			return analytics.SendAnalyticsRecord(h.env, h.apidBase, success, record)
		}
	}

	return nil
}

func (*handler) HandleQuota(ctx context.Context, _ *quota.Instance, args adapter.QuotaArgs) (adapter.QuotaResult, error) {
	return adapter.QuotaResult{
		ValidDuration: 1000000000 * time.Second,
		Amount:        args.QuotaAmount,
	}, nil
}

func (h *handler) HandleAuth(ctx context.Context, inst *authT.Instance) (adapter.CheckResult, error) {
	log := h.env.Logger()
	log.Infof("HandleAuth: %v\n", inst)

	if inst.Apikey == "" {
		return adapter.CheckResult{
			Status: rpc.Status{
				Code:    int32(rpc.UNAUTHENTICATED),
				Message: "Unauthorized",
			},
		}, nil
	}

	verifyApiKeyRequest := auth.VerifyApiKeyRequest{
		Key:              inst.Apikey,
		OrganizationName: h.orgName,
		UriPath:          inst.Uripath,
		ApiProxyName:	  inst.Apigeeproxy,
		EnvironmentName:  h.envName,
	}

	//log.Errorf("sending: %v, %v", h.apidBase, verifyApiKeyRequest)

	success, fail, err := auth.VerifyAPIKey(h.env, h.apidBase, verifyApiKeyRequest)
	if err != nil {
		log.Errorf("apid err: %v\n", err)
		return adapter.CheckResult{
			Status: rpc.Status{
				Code:    int32(rpc.PERMISSION_DENIED),
				Message: err.Error(),
			},
		}, err
	}

	if success != nil {
		log.Infof("auth success!\n")
		return adapter.CheckResult{
			Status: rpc.Status{Code: int32(rpc.OK)},
		}, nil
	}

	log.Infof("auth fail: %v\n", fail.ResponseMessage)
	return adapter.CheckResult{
		Status: rpc.Status{
			Code:    int32(rpc.PERMISSION_DENIED),
			Message: fail.ResponseMessage,
		},
	}, nil
}
