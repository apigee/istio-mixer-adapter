package apigee

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

func (*builder) SetAuthTypes(t map[string]*authT.Type)    {
	fmt.Printf("SetAuthTypes: %v\n", t)
}

func (*builder) SetLogEntryTypes(t map[string]*logentry.Type) {
	fmt.Printf("SetLogEntryTypes: %v\n", t)
}
func (*builder) SetQuotaTypes(map[string]*quota.Type)       {}

////////////////// Handler //////////////////////////

type handler struct {
	apidBase    string
	orgName     string
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
	fmt.Println("HandleLogEntry")

	for _, logEntry := range logEntries {

		h.annotateWithAuthFields(logEntry)

		err := analytics.SendAnalyticsRecord(h.apidBase, h.orgName, "test", logEntry.Variables)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *handler) annotateWithAuthFields(logEntry *logentry.Instance) {

	// todo: hack - perform authentication here as Istio is not able to pass auth context
	apiKey := ""
	if v := logEntry.Variables["apikey"]; v != nil {
		apiKey = v.(string)
		delete(logEntry.Variables, "apikey")
	}

	path := "/"
	if v := logEntry.Variables["request_uri"]; v != nil {
		path = v.(string)
		if path == "" {
			path = "/"
		}
	}

	success, fail, err := auth.VerifyAPIKey(h.apidBase, h.orgName, apiKey, path)
	if err != nil {
		fmt.Printf("annotateWithAuthFields error: %v\n", err)
		return
	}
	if fail != nil {
		fmt.Printf("annotateWithAuthFields fail: %v\n", fail.ResponseMessage)
		return
	}

	// todo: verify
	app := ""
	if len(success.Developer.Apps) > 0 {
		app = success.Developer.Apps[0]
	}

	// todo: verify
	proxy := ""
	if len(success.ApiProduct.Apiproxies) > 0 {
		proxy = success.ApiProduct.Apiproxies[0]
	}

	logEntry.Variables["apiproxy"] = proxy
	logEntry.Variables["apiRevision"] = "" // todo: is this available?
	logEntry.Variables["developerEmail"] = success.Developer.Email
	logEntry.Variables["developerApp"] = app
	logEntry.Variables["accessToken"] = success.ClientId.ClientSecret
	logEntry.Variables["clientID"] = success.ClientId.ClientId
	logEntry.Variables["apiProduct"] = success.ApiProduct.Name
}

func (*handler) HandleQuota(ctx context.Context, _ *quota.Instance, args adapter.QuotaArgs) (adapter.QuotaResult, error) {
	return adapter.QuotaResult{
		ValidDuration: 1000000000 * time.Second,
		Amount:        args.QuotaAmount,
	}, nil
}

func (h *handler) HandleAuth(ctx context.Context, inst *authT.Instance) (adapter.CheckResult, error) {
	fmt.Printf("HandleAuth: %v\n", inst)

	success, fail, err := auth.VerifyAPIKey(h.apidBase, h.orgName, inst.Apikey, inst.Uripath)
	if err != nil {
		fmt.Printf("apid err: %v\n", err)
		return adapter.CheckResult{
			Status: rpc.Status{
				Code:    int32(rpc.PERMISSION_DENIED),
				Message: err.Error(),
			},
		}, err
	}

	if success != nil {
		fmt.Println("auth success!")
		return adapter.CheckResult{
			Status: rpc.Status{Code: int32(rpc.OK)},
		}, nil
	}

	fmt.Printf("auth fail: %v\n", fail.ResponseMessage)
	return adapter.CheckResult{
		Status: rpc.Status{
			Code:    int32(rpc.PERMISSION_DENIED),
			Message: fail.ResponseMessage,
		},
	}, nil
}
