module github.com/apigee/istio-mixer-adapter/grpc-server

go 1.13

replace github.com/apigee/istio-mixer-adapter/apigee => ../apigee

replace github.com/apigee/istio-mixer-adapter/mixer => ../mixer

require (
	github.com/apigee/istio-mixer-adapter/mixer v0.0.0-00010101000000-000000000000
	github.com/spf13/cobra v0.0.6
	istio.io/pkg v0.0.0-20200131182711-9ba13e0e34bb
)
