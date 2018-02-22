package apigee

import (
	"context"
	"github.com/apigee/istio-mixer-adapter/apigee/config"
	"github.com/apigee/istio-mixer-adapter/template/analytics"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/pkg/adapter/test"
	"istio.io/istio/mixer/pkg/status"
	"istio.io/istio/mixer/template/apikey"
	"istio.io/istio/mixer/template/quota"
	"istio.io/istio/mixer/template/authorization"
	"testing"

	rpc "istio.io/gogo-genproto/googleapis/google/rpc"
)

func TestValidateBuild(t *testing.T) {
	b := GetInfo().NewBuilder().(*builder)

	b.SetAdapterConfig(GetInfo().DefaultConfig)
	b.SetAdapterConfig(&config.Params{
		ApigeeBase: "https://edgemicroservices.apigee.net/edgemicro/",
		CustomerBase: "http://theganyo1-eval-test.apigee.net/edgemicro-auth",
		OrgName: "theganyo1-eval",
		EnvName: "test",
		Key: "key",
		Secret: "secret",
	})

	if err := b.Validate(); err != nil {
		t.Errorf("Validate() resulted in unexpected error: %v", err)
	}

	// invoke the empty set methods for coverage
	b.SetAnalyticsTypes(map[string]*analytics.Type{})
	b.SetApiKeyTypes(map[string]*apikey.Type{})
	b.SetQuotaTypes(map[string]*quota.Type{})

	// check build
	if _, err := b.Build(context.Background(), test.NewEnv(t)); err != nil {
		t.Errorf("Build() resulted in unexpected error: %v", err)
	}
}

func TestHandleAnalytics(t *testing.T) {
	ctx := context.Background()

	h := &handler{
		env: test.NewEnv(t),
	}

	err := h.HandleAnalytics(ctx, nil)
	if err != nil {
		t.Errorf("HandleAnalytics(ctx, nil) resulted in an unexpected error: %v", err)
	}

	if err := h.Close(); err != nil {
		t.Errorf("Close() returned an unexpected error")
	}
}

func TestHandleApiKey(t *testing.T) {
	ctx := context.Background()

	h := &handler{
		env: test.NewEnv(t),
	}

	inst := &apikey.Instance{}

	got, err := h.HandleApiKey(ctx, inst)
	if err != nil {
		t.Errorf("HandleApiKey(ctx, nil) resulted in an unexpected error: %v", err)
	}
	//if !status.IsOK(got.Status) {
	//	t.Errorf("HandleApiKey(ctx, nil) => %#v, want %#v", got.Status, status.OK)
	//}
	if got.Status.Code != int32(rpc.PERMISSION_DENIED) {
		t.Errorf("HandleApiKey(ctx, nil) => %#v, want %#v", got.Status, status.OK)
	}

	if err := h.Close(); err != nil {
		t.Errorf("Close() returned an unexpected error")
	}
}

func TestHandleAuthorization(t *testing.T) {
	ctx := context.Background()

	h := &handler{
		env: test.NewEnv(t),
	}

	inst := &authorization.Instance{}

	got, err := h.HandleAuthorization(ctx, inst)
	if err != nil {
		t.Errorf("HandleAuthorization(ctx, nil) resulted in an unexpected error: %v", err)
	}

	if got.Status.Code != int32(rpc.PERMISSION_DENIED) {
		t.Errorf("HandleAuthorization(ctx, nil) => %#v, want %#v", got.Status, status.OK)
	}

	if err := h.Close(); err != nil {
		t.Errorf("Close() returned an unexpected error")
	}
}

func TestHandleQuota(t *testing.T) {
	ctx := context.Background()

	h := &handler{
		env: test.NewEnv(t),
	}

	inst := &quota.Instance{
		Name: "",
		Dimensions: map[string]interface{}{
			"": "",
		},
	}

	got, err := h.HandleQuota(ctx, inst, adapter.QuotaArgs{})
	if err != nil {
		t.Errorf("HandleQuota(ctx, nil) resulted in an unexpected error: %v", err)
	}
	if !status.IsOK(got.Status) {
		t.Errorf("HandleQuota(ctx, nil) => %#v, want %#v", got.Status, status.OK)
	}

	if err := h.Close(); err != nil {
		t.Errorf("Close() returned an unexpected error")
	}
}

//func TestResolveProducts(t *testing.T) {
//
//	ac := auth.Context{}
//	api := ""
//	path := ""
//	var got []auth.ApiProductDetails
//
//	got = resolveProducts(ac, api, path)
//}