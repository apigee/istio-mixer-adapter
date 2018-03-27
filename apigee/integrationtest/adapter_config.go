package integrationtest

import (
	"strings"

	"github.com/apigee/istio-mixer-adapter/apigee"
	"istio.io/istio/mixer/pkg/adapter"
	authT "istio.io/istio/mixer/template/authorization"
	quotaT "istio.io/istio/mixer/template/quota"
)

// removed Analytics because the integration test framework can't handle it
func testGetInfo() adapter.Info {
	info := apigee.GetInfo()
	info.SupportedTemplates = []string{
		quotaT.TemplateName,
		authT.TemplateName,
	}
	return info
}

func adapterConfigForQuota() string {
	return strings.Replace(adapterConfig, "__INSTANCE__", "apigee.quota", 1)
}

func adapterConfigForAuthorization() string {
	return strings.Replace(adapterConfig, "__INSTANCE__", "apigee.authorization", 1)
}

const (
	adapterConfig = `

apiVersion: config.istio.io/v1alpha2
kind: apigee
metadata:
  name: apigee-handler
  namespace: istio-system
spec:
  apigee_base: __SERVER_BASE_URL__
  customer_base: __SERVER_BASE_URL__
  org_name: org
  env_name: env
  key: key
  secret: secret

---

apiVersion: config.istio.io/v1alpha2
kind: rule
metadata:
  name: apigee-rule
  namespace: istio-system
spec:
  actions:
  - handler: apigee-handler.apigee
    instances:
    - __INSTANCE__

---

# instance configuration for template 'apigee.quota'
apiVersion: config.istio.io/v1alpha2
kind: quota
metadata:
  name: apigee
  namespace: istio-system
spec:
  dimensions:
    api: api.service | destination.service | ""
    path: request.path | ""
    api_key: request.api_key | request.headers["x-api-key"] | ""
    encoded_claims: request.headers["sec-istio-auth-userinfo"] | ""

---

# instance configuration for template 'apigee.authorization'
apiVersion: config.istio.io/v1alpha2
kind: authorization
metadata:
  name: apigee
  namespace: istio-system
spec:
  subject:
    user: ""
    groups: ""
    properties:
      encoded_claims: request.headers["sec-istio-auth-userinfo"] | ""
      api_key: request.api_key | request.headers["x-api-key"] | ""
      # api_product_list: request.auth.claims["api_product_list"] | ""
      # audience: request.auth.claims["audience"] | ""
      # access_token: request.auth.claims["access_token"] | ""
      # client_id: request.auth.claims["client_id"] | ""
      # application_name: request.auth.claims["application_name"] | ""
      # scopes: request.auth.claims["scopes"] | ""
      # exp: request.auth.claims["exp"] | ""
  action:
    namespace: destination.namespace | "default"
    service: api.service | destination.service | ""
    path: api.operation | request.path | ""
    method: request.method | ""

`
)
