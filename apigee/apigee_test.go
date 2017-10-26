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
)

func TestReport(t *testing.T) {
	// todo: create temp config?
	opConfig, err := filepath.Abs("../testdata/operatorconfig")
	if err != nil {
		t.Fatalf("fail to get absolute path for operatorconfig: %v", err)
	}

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

	// create test env
	env, err := testenv.NewEnv(&args, templateInfo, []adapter.InfoFn{GetInfo}) // <- inject mock GetInfo here
	if err != nil {
		t.Fatalf("fail to create testenv: %v", err)
	}
	defer safeClose(env)

	// create mixer client
	client, conn, err := env.CreateMixerClient()
	if err != nil {
		t.Fatalf("fail to create client connection: %v", err)
	}
	defer safeClose(conn)

	// set mixer attributes here
	attrs := map[string]interface{}{
		"response.code": int64(400),
	}

	// call report interface
	bag := testenv.GetAttrBag(attrs, args.ConfigIdentityAttribute, args.ConfigIdentityAttributeDomain)
	request := mixerapi.ReportRequest{Attributes: []mixerapi.CompressedAttributes{bag}}
	_, err = client.Report(context.Background(), &request)
	if err != nil {
		t.Errorf("fail to send report to Mixer %v", err)
	}
}

func safeClose(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Fatal(err)
	}
}
