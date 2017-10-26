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
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/template/logentry"
	"istio.io/mixer/template/quota"
)

////////////////// GetInfo //////////////////////////

// returns the adapter.Info metadata during mixer initialization
func GetInfo() adapter.Info {
	return adapter.Info{
		Name:        "apigee",
		Impl:        "istio.io/mixer/adapter/apigee",
		Description: "Apigee adapter",
		SupportedTemplates: []string{
			logentry.TemplateName,
			quota.TemplateName,
			authT.TemplateName,
		},
		NewBuilder:    func() adapter.HandlerBuilder { return &builder{} },
		DefaultConfig: &config.Params{},
	}
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

// adapter.HandlerBuilder
func (b *builder) SetAdapterConfig(cfg adapter.Config) {
	b.adapterConfig = cfg.(*config.Params)
}

// adapter.HandlerBuilder
func (b *builder) Build(context context.Context, env adapter.Env) (adapter.Handler, error) {
	return &handler{
		apidBase:    b.adapterConfig.ApidBase,
		orgName:     b.adapterConfig.OrgName,
		envName:     b.adapterConfig.EnvName,
		env:         env,
	}, nil
}

// adapter.HandlerBuilder
func (b *builder) Validate() (ce *adapter.ConfigErrors) {

	fmt.Printf("Validate: %v\n", b.adapterConfig)

	if b.adapterConfig.ApidBase == "" {
		b.adapterConfig.ApidBase = "http://apid:9000/"
	}
	if _, err := url.Parse(b.adapterConfig.ApidBase); err != nil {
		ce = ce.Append("apid_base", err)
	}

	if b.adapterConfig.OrgName == "" {
		ce = ce.Append("org_name", fmt.Errorf("org_name is required"))
	}

	return ce
}

func (*builder) SetAuthTypes(t map[string]*authT.Type) {}
func (*builder) SetLogEntryTypes(t map[string]*logentry.Type) {}
func (*builder) SetQuotaTypes(map[string]*quota.Type) {}

////////////////// Handler //////////////////////////

type handler struct {
	apidBase    string
	orgName     string
	envName     string
	env         adapter.Env
}

// force interface checks at compile time
var _ adapter.Handler = (*handler)(nil)
var _ authT.Handler = (*handler)(nil)
var _ quota.Handler = (*handler)(nil)
var _ logentry.Handler = (*handler)(nil)

// adapter.Handler
func (h *handler) Close() error { return nil }

func (h *handler) HandleLogEntry(ctx context.Context, logEntries []*logentry.Instance) error {

	log := h.env.Logger()
	log.Infof("HandleLogEntry\n")

	for _, logEntry := range logEntries {

		err := analytics.SendAnalyticsRecord(h.env, h.apidBase, h.orgName, h.envName, logEntry.Variables)
		if err != nil {
			return err
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

	verifyApiKeyRequest := auth.VerifyApiKeyRequest{
		Key:              inst.Apikey,
		OrganizationName: h.orgName,
		UriPath:          inst.Uripath,
		ApiProxyName:	  inst.Apigeeproxy,
		EnvironmentName:  h.envName,
	}

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
