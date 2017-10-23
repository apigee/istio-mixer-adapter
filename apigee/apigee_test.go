package apigee

//import (
//	"io"
//	"log"
//	"testing"
//
//	"golang.org/x/net/context"
//
//	mixerapi "istio.io/api/mixer/v1"
//	"istio.io/mixer/pkg/adapter"
//	"istio.io/mixer/test/testenv"
//	"istio.io/mixer/template"
//	pkgTmpl "istio.io/mixer/pkg/template"
//	"path/filepath"
//	//"github.com/guptasu/remoteAdapter/testmixersupportedtmpls"
//)
//
//func TestApigeeAdapter(t *testing.T) {
//	opConfig,err := filepath.Abs("../testdata/operatorconfig")
//	if err != nil {
//		t.Fatalf("fail to get absolute path for operatorconfig: %v", err)
//	}
//
//	var args = testenv.Args{
//		// Start Mixer server on a free port on loop back interface
//		MixerServerAddr:               `127.0.0.1:0`,
//		ConfigStoreURL:                `fs://` + opConfig,
//		ConfigStore2URL:               `fs://` + opConfig,
//		ConfigDefaultNamespace:        "istio-system",
//		ConfigIdentityAttribute:       "destination.service",
//		ConfigIdentityAttributeDomain: "svc.cluster.local",
//	}
//
//	st := make(map[string]pkgTmpl.Info)
//
//	for k,v := range testmixersupportedtmpls.SupportedTmplInfo {
//		st[k] = v
//	}
//	for k,v := range template.SupportedTmplInfo {
//		st[k] = v
//	}
//
//	env, err := testenv.NewEnv(&args, st, []adapter.InfoFn{GetInfo})
//	if err != nil {
//		t.Fatalf("fail to create testenv: %v", err)
//	}
//	defer closeHelper(env)
//
//	client, conn, err := env.CreateMixerClient()
//	if err != nil {
//		t.Fatalf("fail to create client connection: %v", err)
//	}
//	defer closeHelper(conn)
//
//	attrs := map[string]interface{}{"response.code": int64(400)}
//	bag := testenv.GetAttrBag(attrs, args.ConfigIdentityAttribute, args.ConfigIdentityAttributeDomain)
//	request := mixerapi.ReportRequest{Attributes: []mixerapi.CompressedAttributes{ bag}}
//	_, err = client.Report(context.Background(), &request)
//	if err != nil {
//		t.Errorf("fail to send report to Mixer %v", err)
//	}
//}
//
//func closeHelper(c io.Closer) {
//	err := c.Close()
//	if err != nil {
//		log.Fatal(err)
//	}
//}