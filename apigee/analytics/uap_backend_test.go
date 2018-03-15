package analytics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/authtest"
	adaptertest "istio.io/istio/mixer/pkg/adapter/test"
)

func fakeAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(""))
}

func fakeServer() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/analytics", fakeAnalyticsHandler)
	return m
}

func TestSmoke(t *testing.T) {
	// TODO(robbrit): This is a smoke test. Make it not a smoke test.
	srv := httptest.NewServer(fakeServer())
	defer srv.Close()

	m := newUAPBackend().(*uapBackend)

	r := []Record{
		{APIProxy: "proxy"},
	}

	tc := authtest.NewContext(srv.URL, adaptertest.NewEnv(t))
	tc.SetOrganization("hi")
	tc.SetEnvironment("test")
	ctx := &auth.Context{Context: tc}

	if err := m.SendRecords(ctx, r); err != nil {
		t.Errorf("Error on SendRecords(): %s", err)
	}

	if err := m.flush(); err != nil {
		t.Errorf("Error on flush(): %s", err)
	}
}
