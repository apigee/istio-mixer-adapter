#!/bin/bash

# This script will build the samples/apigee/definitions.yaml file.
# Run this if any of the proto files (config, authorization, analytics) are changed.
# See RELEASING.md for documentation of full release process.

SCRIPTPATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOTDIR="$(dirname "$SCRIPTPATH")"

MIXGEN=$ROOTDIR/mixgen/main.go
DEFINITIONS_FILE="${ROOTDIR}/samples/apigee/definitions.yaml"

read -r -d '' DEFINITIONS_BASE <<"EOT"
# This file generated via bin/build_grpc_definitions.sh. Regenerate if
# any of the proto files (config, authorization, analytics) are changed.
#
# Defines the base structures and data map for the Apigee mixer adapter.
# In general, these are static and should not need to be modified.
# However, certain specific behaviors such as where to retrieve an API Key
# could be changed here.
---
# instance for GRPC template authorization
apiVersion: "config.istio.io/v1alpha2"
kind: instance
metadata:
  name: apigee-authorization
  namespace: istio-system
spec:
  template: apigee-authorization
  params:
    subject:
      properties:
        api_key: request.api_key | request.headers["x-api-key"] | ""
        json_claims: request.auth.raw_claims | ""
    action:
      namespace: destination.namespace | "default"
      service: api.service | destination.service.host | ""
      path: api.operation | request.path | ""
      method: request.method | ""
---
# instance for GRPC template analytics
apiVersion: "config.istio.io/v1alpha2"
kind: instance
metadata:
  name: apigee-analytics
  namespace: istio-system
spec:
  template: apigee-analytics
  params:
    api_key: request.api_key | request.headers["x-api-key"] | ""
    api_proxy: api.service | destination.service.host | ""
    response_status_code: response.code | 0
    client_ip: source.ip | ip("0.0.0.0")
    request_verb: request.method | ""
    request_uri: request.path | ""
    useragent: request.useragent | ""
    client_received_start_timestamp: request.time
    client_received_end_timestamp: request.time
    target_sent_start_timestamp: request.time
    target_sent_end_timestamp: request.time
    target_received_start_timestamp: response.time
    target_received_end_timestamp: response.time
    client_sent_start_timestamp: response.time
    client_sent_end_timestamp: response.time
    api_claims: # from jwt
      json_claims: request.auth.raw_claims | ""
---
EOT


templateDS=$GOPATH/src/istio.io/istio/mixer/template/authorization/template_handler_service.descriptor_set
AUTHORIZATION=$(go run $MIXGEN template -d $templateDS -n apigee-authorization)

templateDS=$GOPATH/src/github.com/apigee/istio-mixer-adapter/mixer/analytics/template_handler_service.descriptor_set
ANALYTICS=$(go run $MIXGEN template -d $templateDS -n apigee-analytics)

templateDS=$GOPATH/src/github.com/apigee/istio-mixer-adapter/mixer/config/config.proto_descriptor
APIGEE=$(go run $MIXGEN adapter -c $templateDS -s=false -t apigee-authorization -t apigee-analytics -n apigee)

NEWLINE=$'\n'
echo "$DEFINITIONS_BASE $NEWLINE $AUTHORIZATION $NEWLINE $ANALYTICS $NEWLINE $APIGEE" > $DEFINITIONS_FILE

echo "Generated new file: $DEFINITIONS_FILE"
