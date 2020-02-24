module github.com/apigee/istio-mixer-adapter

go 1.13

replace github.com/apigee/istio-mixer-adapter/apigee => ./apigee

replace github.com/apigee/istio-mixer-adapter/mixer => ./mixer

replace github.com/apigee/istio-mixer-adapter/apigee-istio => ./apigee-istio

require (
	github.com/apigee/istio-mixer-adapter/apigee v0.0.0-00010101000000-000000000000 // indirect
	github.com/apigee/istio-mixer-adapter/apigee-istio v0.0.0-00010101000000-000000000000 // indirect
	github.com/apigee/istio-mixer-adapter/mixer v0.0.0-00010101000000-000000000000 // indirect
)
