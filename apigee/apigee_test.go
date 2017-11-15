package apigee

import (
	"io"
	"log"
	"path/filepath"
	"testing"
	"context"

	"github.com/apigee/istio-mixer-adapter/apigee/testapigeeadapter"
	mixerapi "istio.io/api/mixer/v1"
	"istio.io/istio/mixer/pkg/adapter"
	pkgTmpl "istio.io/istio/mixer/pkg/template"
	"istio.io/istio/mixer/template"
	"istio.io/istio/mixer/test/testenv"
	"os"
	"io/ioutil"
	"github.com/apigee/istio-mixer-adapter/apigee/testutil"
	"net/http/httptest"
	"net/http"
	templat "text/template"
	"time"
	"strconv"
	"path"
	"github.com/apigee/istio-mixer-adapter/apigee/config"
	authT "github.com/apigee/istio-mixer-adapter/template/auth"
	rpc "github.com/googleapis/googleapis/google/rpc"
	"istio.io/istio/mixer/template/logentry"
)

// todo: can we run multiple Mixer environment tests?

func TestConfig(t *testing.T) {

	// wrap adapter builder & handler
	testB := &testBuilder{&builder{}, t}
	oldCreate := createBuilder
	createBuilder = func() adapter.HandlerBuilder { return testB }
	defer func() { createBuilder = oldCreate }()

	// create mock API server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testutil.VerifyApiKeyOr(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		})(w, r)
	}))
	defer ts.Close()

	// create config directory
	configDir, err := ioutil.TempDir("", "testdata")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(configDir)
	writeConfig(t, ts, configDir)
	opConfig, err := filepath.Abs(configDir)
	if err != nil {
		t.Fatalf("fail to get absolute path for dir: %v", err)
	}

	// create Mixer test env
	var args = testenv.Args{
		// Start Mixer server on a free port on loop back interface
		MixerServerAddr:               `127.0.0.1:0`,
		ConfigStoreURL:                `fs://` + opConfig,
		ConfigStore2URL:               `fs://` + opConfig,
		ConfigDefaultNamespace:        "istio-system",
		ConfigIdentityAttribute:       "destination.service",
		ConfigIdentityAttributeDomain: "svc.cluster.local",
	}
	templateInfo := make(map[string]pkgTmpl.Info)
	for k, v := range testapigeeadapter.SupportedTmplInfo {
		templateInfo[k] = v
	}
	for k, v := range template.SupportedTmplInfo {
		templateInfo[k] = v
	}
	env, err := testenv.NewEnv(&args, templateInfo, []adapter.InfoFn{GetInfo})
	if err != nil {
		t.Fatalf("fail to create testenv: %v", err)
	}
	defer safeClose(env)

	// create Mixer client
	client, conn, err := env.CreateMixerClient()
	if err != nil {
		t.Fatalf("fail to create client connection: %v", err)
	}
	defer safeClose(conn)

	// check Validation
	cfg := testB.GetAdapterConfig()
	cfg.ApidBase = ""
	cfg.OrgName = ""
	cfg.EnvName = ""
	cfgErrors := testB.Validate()
	if len(cfgErrors.Multi.Errors) != 3 {
		t.Errorf("expected 3 Validate() errors, got: %v", cfgErrors)
	}

	// set up request attributes
	bag := testenv.GetAttrBag(map[string]interface{}{
		"request.headers": map[string]string{
			"apikey": "xxx",
		},
		"request.time": time.Now(),
	}, args.ConfigIdentityAttribute, args.ConfigIdentityAttributeDomain)

	// call check interface
	checkReq := mixerapi.CheckRequest{
		Attributes: bag,
		DeduplicationId: strconv.Itoa(time.Now().Nanosecond()),
	}
	_, err = client.Check(context.Background(), &checkReq)
	if err != nil {
		t.Errorf("fail to send check to Mixer %v", err)
	}

	// call report interface
	reportReq := mixerapi.ReportRequest{Attributes: []mixerapi.CompressedAttributes{bag}}
	_, err = client.Report(context.Background(), &reportReq)
	if err != nil {
		t.Errorf("fail to send report to Mixer %v", err)
	}
}

type testBuilder struct {
	*builder
	t *testing.T
}

func (b *testBuilder) GetAdapterConfig() *config.Params {
	return b.builder.adapterConfig
}

func (b *testBuilder) Build(context context.Context, env adapter.Env) (th adapter.Handler, err error) {
	h, err := b.builder.Build(context, env)
	if h != nil {
		th = &testHandler{h.(*handler), b.t }
	}
	return th, err
}

type testHandler struct {
	*handler
	t *testing.T
}

func (h *testHandler) HandleAuth(ctx context.Context, inst *authT.Instance) (cr adapter.CheckResult, err error) {

	if inst.Apikey != "xxx" {
		h.t.Errorf("Unexpected value in HandleAuth. Expected: %v, got: %v", "xxx", inst.Apikey)
	}

	inst.Apikey = ""
	cr, err = h.handler.HandleAuth(ctx, inst)
	if err != nil {
		h.t.Errorf("Failed unauthenticated call to HandleAuth: %v", err)
	}
	if cr.Status.Code != int32(rpc.UNAUTHENTICATED) {
		h.t.Errorf("HandleAuth unauthenticated code incorrect: %v", cr.Status.Code)
	}
	if cr.Status.Message != "Unauthorized" {
		h.t.Errorf("HandleAuth unauthenticated message incorrect: %v", cr.Status.Message)
	}

	inst.Apikey = "fail"
	cr, err = h.handler.HandleAuth(ctx, inst)
	if err != nil {
		h.t.Errorf("Failed fail call to HandleAuth: %v", err)
	}
	if cr.Status.Code != int32(rpc.PERMISSION_DENIED) {
		h.t.Errorf("HandleAuth fail code incorrect: %v", cr.Status.Code)
	}
	if cr.Status.Message != "fail" {
		h.t.Errorf("HandleAuth fail message incorrect: %v", cr.Status.Message)
	}

	inst.Apikey = "error"
	cr, err = h.handler.HandleAuth(ctx, inst)
	if err == nil {
		h.t.Error("HandleAuth error response missing error")
	}
	if cr.Status.Code != int32(rpc.PERMISSION_DENIED) {
		h.t.Errorf("HandleAuth error code incorrect: %v", cr.Status.Code)
	}

	inst.Apikey = "success"
	cr, err = h.handler.HandleAuth(ctx, inst)
	if err != nil {
		h.t.Errorf("Failed success call to HandleAuth: %v", err)
	}
	if cr.Status.Code != int32(rpc.OK) {
		h.t.Errorf("HandleAuth success code incorrect: %v", cr.Status.Code)
	}

	return
}

func (h *testHandler) HandleLogEntry(ctx context.Context, logEntries []*logentry.Instance) error {

	if len(logEntries) != 1 {
		h.t.Errorf("HandleLogEntry expected 1 row, got: %v", logEntries)
	} else {
		t := logEntries[0].Variables["client_received_start_timestamp"]
		if t == nil {
			h.t.Errorf("HandleLogEntry expected valid client_received_start_timestamp, got: %v", logEntries[0].Variables)
		} else if time.Now().Sub(t.(time.Time)) > time.Second {
			h.t.Errorf("Unexpected time. Expected near: %v, got: %v", time.Now(), t)
		}
	}

	err := h.handler.HandleLogEntry(ctx, logEntries)
	if err != nil {
		h.t.Errorf("Failed call to HandleLogEntry: %v", err)
	}

	return err
}

func safeClose(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func writeConfig(t *testing.T, ts *httptest.Server, configDir string) {
	// copy attributes.yaml
	data, err := ioutil.ReadFile("../testdata/operatorconfig/attributes.yaml")
	if err != nil {
		t.Fatalf("fail to link file: %v", err)
	}
	err = ioutil.WriteFile(path.Join(configDir, "attributes.yaml"), data, 0644)
	if err != nil {
		t.Fatalf("fail to link file: %v", err)
	}

	configFilePath := filepath.Join(configDir, "config.yaml")

	// create config.yaml
	tmpl, err := templat.New("x").Parse(configYaml)
	configFile, err := os.Create(configFilePath)
	if err != nil {
		t.Fatalf("fail to create file %s: %v", configFilePath, err)
	}
	defer configFile.Close()
	if err := tmpl.Execute(configFile, ts); err != nil {
		t.Fatalf("fail to execute template: %v", err)
	}
}

//
// Keep in sync with testdata/operatorconfig/config.yaml
//
var configYaml = `
# handler configuration for adapter 'apigee'
apiVersion: "config.istio.io/v1alpha2"
kind: apigee
metadata:
  name: apigee-handler
  namespace: istio-system
spec:
  apid_base: {{.URL}}
  org_name: edgex01
  env_name: test
---
# instance configuration for template 'auth'
apiVersion: "config.istio.io/v1alpha2"
kind: auth
metadata:
  name: helloworld
  namespace: istio-system
spec:
  apikey: request.headers["apikey"] | ""
  uripath: request.path | "/"
  apigeeproxy: '"helloworld"'
---
# instance configuration for template 'logentry'
apiVersion: "config.istio.io/v1alpha2"
kind: analytics
metadata:
  name: helloworld
  namespace: istio-system
spec:
  apikey: request.headers["apikey"] | "" # HACK
  apigeeproxy: '"helloworld"' # HACK
  apigeeproxy_revision: "0"" # HACK
  response_status_code: response.code | 0
  client_ip: source.ip | ip("0.0.0.0")
  request_verb: request.method | ""
  request_uri: request.path | ""
  request_path: request.path | ""
  useragent: request.useragent | ""
  client_received_start_timestamp: request.time # HACK - no ability to provide default ts values!
  client_received_end_timestamp: request.time
  target_sent_start_timestamp: request.time
  target_sent_end_timestamp: request.time
  target_received_start_timestamp: response.time
  target_received_end_timestamp: response.time
  client_sent_start_timestamp: response.time
  client_sent_end_timestamp: response.time
---
# rule to dispatch to handler 'apigee'
apiVersion: "config.istio.io/v1alpha2"
kind: rule
metadata:
  name: helloworld
  namespace: istio-system
spec:
  match: "true"
  actions:
  - handler: apigee-handler.apigee
    instances:
    - helloworld.auth
    - helloworld.analytics

`