package apigee

import (
	"io"
	"log"
	"path/filepath"
	"testing"

	"github.com/apigee/istio-mixer-adapter/apigee/testapigeeadapter"
	"golang.org/x/net/context"
	mixerapi "istio.io/api/mixer/v1"
	"istio.io/mixer/pkg/adapter"
	pkgTmpl "istio.io/mixer/pkg/template"
	"istio.io/mixer/template"
	"istio.io/mixer/test/testenv"
	"os"
	"io/ioutil"
	"github.com/apigee/istio-mixer-adapter/apigee/testutil"
	"net/http/httptest"
	"net/http"
	templat "text/template"
	"time"
	"strconv"
	"path"
)

// todo: can we run multiple Mixer environment tests?

func TestConfig(t *testing.T) {

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
	if err := tmpl.Execute(configFile, ts); err != nil {
		t.Fatalf("fail to execute template: %v", err)
	}
	configFile.Close()

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
	env, err := testenv.NewEnv(&args, templateInfo, []adapter.InfoFn{GetInfo}) // <- inject mock GetInfo here
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

	// call check interface
	bag := testenv.GetAttrBag(map[string]interface{}{
		"request.headers": map[string]string{
			"apikey": "xxx",
		},
	}, args.ConfigIdentityAttribute, args.ConfigIdentityAttributeDomain)
	checkReq := mixerapi.CheckRequest{
		Attributes: bag,
		DeduplicationId: strconv.Itoa(time.Now().Nanosecond()),
	}
	_, err = client.Check(context.Background(), &checkReq)
	if err != nil {
		t.Errorf("fail to send check to Mixer %v", err)
	}
	// todo: how to verify this actually worked?

	// call report interface
	bag = testenv.GetAttrBag(map[string]interface{}{
		"request.time": time.Now(),
	}, args.ConfigIdentityAttribute, args.ConfigIdentityAttributeDomain)
	reportReq := mixerapi.ReportRequest{Attributes: []mixerapi.CompressedAttributes{bag}}
	_, err = client.Report(context.Background(), &reportReq)
	if err != nil {
		t.Errorf("fail to send report to Mixer %v", err)
	}
	// todo: how to verify this actually worked?
}

func safeClose(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Fatal(err)
	}
}

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
kind: logentry
metadata:
  name: helloworld
  namespace: istio-system
spec:
  severity: '"Default"'
  timestamp: request.time
  variables:
    apikey: request.headers["apikey"] | "" # HACK
    apigeeproxy: '"helloworld"'
  monitoredResourceType: '"UNSPECIFIED"'
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
    - helloworld.logentry

`