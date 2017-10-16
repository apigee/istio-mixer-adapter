package apigee

import (
	"context"
	"fmt"
	"net/url"
	"time"

	rpc "github.com/googleapis/googleapis/google/rpc"
	apidAnalytics "github.com/apigee/istio-mixer-adapter/apigee/analytics"
	"github.com/apigee/istio-mixer-adapter/apigee/config"
	"github.com/apigee/istio-mixer-adapter/template/auth"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/template/logentry"
	//"istio.io/mixer/template/metric"
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
			auth.TemplateName,
			//metric.TemplateName,
		},
		NewBuilder:    func() adapter.HandlerBuilder { return &builder{} },
		DefaultConfig: &config.Params{},
	}
}

////////////////// Builder //////////////////////////

type builder struct {
	adapterConfig *config.Params
	//metricTypes   map[string]*metric.Type
}

// force interface checks at compile time
var _ adapter.HandlerBuilder = (*builder)(nil)
var _ auth.HandlerBuilder = (*builder)(nil)
var _ logentry.HandlerBuilder = (*builder)(nil)
var _ quota.HandlerBuilder = (*builder)(nil)
//var _ metric.HandlerBuilder = (*builder)(nil)

// adapter.HandlerBuilder
func (b *builder) SetAdapterConfig(cfg adapter.Config) {
	b.adapterConfig = cfg.(*config.Params)
}

// adapter.HandlerBuilder
func (b *builder) Build(context context.Context, env adapter.Env) (adapter.Handler, error) {
	return &handler{
		apidBase:    b.adapterConfig.ApidBase,
		orgName:     b.adapterConfig.OrgName,
		//metricTypes: b.metricTypes,
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

//func (b *builder) SetMetricTypes(t map[string]*metric.Type) {
//	fmt.Printf("SetMetricTypes: %v\n", t)
//	b.metricTypes = t
//}

func (*builder) SetAuthTypes(t map[string]*auth.Type)    {
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
	//metricTypes map[string]*metric.Type
}

// force interface checks at compile time
var _ adapter.Handler = (*handler)(nil)
var _ auth.Handler = (*handler)(nil)
var _ quota.Handler = (*handler)(nil)
var _ logentry.Handler = (*handler)(nil)
//var _ metric.Handler = (*handler)(nil)

// adapter.Handler
func (h *handler) Close() error { return nil }

func (h *handler) HandleLogEntry(ctx context.Context, logs []*logentry.Instance) error {
	fmt.Println("HandleLogEntry")

	for _, log := range logs {
		// todo
		err := apidAnalytics.SendAnalyticsRecord(h.apidBase, h.orgName, "test", log.Variables)
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

func (h *handler) HandleAuth(ctx context.Context, inst *auth.Instance) (adapter.CheckResult, error) {
	fmt.Printf("HandleAuth: %v\n", inst)

	if inst.AuthFailMessage == "" {
		fmt.Println("auth success!")
		return adapter.CheckResult{
			Status: rpc.Status{Code: int32(rpc.OK)},
		}, nil
	}

	fmt.Printf("auth fail: %v\n", inst.AuthFailMessage)
	return adapter.CheckResult{
		Status: rpc.Status{
			Code:    int32(rpc.PERMISSION_DENIED),
			Message: inst.AuthFailMessage,
		},
	}, nil
}

//func (h *handler) HandleMetric(ctx context.Context, insts []*metric.Instance) error {
//	fmt.Println("HandleMetric")
//	//for _, inst := range insts {
//	//	if _, ok := h.metricTypes[inst.Name]; !ok {
//	//		h.env.Logger().Errorf("Cannot find Type for instance %s", inst.Name)
//	//		continue
//	//	}
//	//	h.env.Logger().Infof(`HandleMetric invoke for :
//	//	Instance Name  :'%s'
//	//	Instance Value : %v,
//	//	Type           : %v`, inst.Name, *inst, *h.metricTypes[inst.Name])
//	//}
//	//
//	return nil
//}

