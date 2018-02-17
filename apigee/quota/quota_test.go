package quota

import (
	"encoding/json"
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"istio.io/istio/mixer/pkg/adapter"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
	"istio.io/istio/mixer/pkg/adapter/test"
)

func TestQuota(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := QuotaResult{
			Allowed:    1,
			Used:       1,
			Exceeded:   1,
			ExpiryTime: time.Now().Unix(),
			Timestamp:  time.Now().Unix(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer ts.Close()

	serverURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	context := &TestContext{
		apigeeBase:   *serverURL,
		customerBase: *serverURL,
		log: test.NewEnv(t),
	}
	authContext := &auth.Context{
		Context:        context,
		DeveloperEmail: "email",
		Application:    "app",
		AccessToken:    "token",
		ClientID:       "clientId",
	}

	product := auth.ApiProductDetails{
		QuotaLimit: "1",
		QuotaInterval: 1,
		QuotaTimeUnit: "second",
	}

	args := adapter.QuotaArgs{
		DeduplicationID: "X",
		QuotaAmount: 1,
		BestEffort: true,
	}

	result, err := Apply(*authContext, product, args)
	if err != nil {
		t.Errorf("error should be nil")
	}

	if result.Used != 1 {
		t.Errorf("result used should be 1")
	}
}

type TestContext struct {
	apigeeBase   url.URL
	customerBase url.URL
	orgName      string
	envName      string
	key          string
	secret       string
	log          adapter.Logger
}

func (h *TestContext) Log() adapter.Logger {
	return h.log
}
func (h *TestContext) ApigeeBase() url.URL {
	return h.apigeeBase
}
func (h *TestContext) CustomerBase() url.URL {
	return h.customerBase
}
func (h *TestContext) Organization() string {
	return h.orgName
}
func (h *TestContext) Environment() string {
	return h.envName
}
func (h *TestContext) Key() string {
	return h.key
}
func (h *TestContext) Secret() string {
	return h.secret
}
