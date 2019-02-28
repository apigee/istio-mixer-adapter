# gRPC adapter

The gRPC adapter support is preliminary as it is not yet supported by an Istio release. 

## Installation

1. Install Istio from a master or 1.1.x build

2. Generate (or edit) the handler file

        apigee-istio -u {your username} -p {your password} -o {your organization name} -e {your environment name} provision --grpc > samples/apigee/grpc/handler.yaml

3. Deploy adapter and connect Istio

        kubectl apply -f samples/apigee/grpc/handler.yaml


Notes:

* The `authentication-policy.yaml` and `httpapispec.yaml` files in `samples/apigee` may be used as before.


## Usage

Usage should be indistinguishable from the prior Mixer replacement scheme once installed. 

If something doesn't work:

1. Ensure adapter is running:

        kubectl -n istio-system get po -l app=apigee-adapter
	
2. Check the Mixer logs (policy and/or telemetry)

3. You may also tail the Apigee adapter logs:

        APIGEE_ADAPTER=$(kubectl -n istio-system get po -l app=apigee-adapter -o 'jsonpath={.items[0].metadata.name}')
        kubectl -n istio-system logs $APIGEE_ADAPTER -f



## Development

Build new binary and docker image

    bin/build_adapter_docker.sh
