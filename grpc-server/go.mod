module github.com/apigee/istio-mixer-adapter/grpc-server

go 1.13

replace github.com/apigee/istio-mixer-adapter/apigee => ../apigee

replace github.com/apigee/istio-mixer-adapter/mixer => ../mixer

require (
	github.com/apigee/istio-mixer-adapter/apigee v0.0.0
	github.com/apigee/istio-mixer-adapter/mixer v0.0.0-00010101000000-000000000000
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/spf13/cobra v0.0.6
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/sys v0.0.0-20200116001909-b77594299b42 // indirect
	golang.org/x/text v0.3.2 // indirect
	google.golang.org/grpc v1.27.1
	istio.io/pkg v0.0.0-20200227125209-63966175aa01
)
