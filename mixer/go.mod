module github.com/apigee/istio-mixer-adapter/mixer

go 1.13

replace github.com/apigee/istio-mixer-adapter/apigee => ../apigee

require (
	github.com/apigee/istio-mixer-adapter/apigee v0.0.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gogo/protobuf v1.3.1
	github.com/lestrrat/go-jwx v0.0.0-20180221005942-b7d4802280ae
	google.golang.org/grpc v1.27.1
	istio.io/api v0.0.0-20200221025927-228308df3f1b
	istio.io/istio v0.0.0-20200221194739-b4b8a6846a7a
	istio.io/pkg v0.0.0-20200131182711-9ba13e0e34bb
)
