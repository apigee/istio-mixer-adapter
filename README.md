# Istio Apigee Adapter

This workspace holds an Apigee adapter for Istio Mixer. It can be tested standalone as noted below.
Instructions for building and running in Kubernetes are available here: [README-Kubernetes.MD]().

Note: This repo should be in $GOPATH/src/github.com/apigee/istio-mixer-adapter

## Building and testing standalone

1. Install protoc and dep prerequisites:

        [](https://developers.google.com/protocol-buffers/docs/downloads) 
        [](https://github.com/golang/dep)

2. clone Istio and get deps:

        export ISTIO=$GOPATH/src/istio.io/istio
        mkdir -p $ISTIO
        cd $ISTIO
        git clone https://github.com/istio/istio
        cd istio
        make build

3. Install adapter dependencies

        cd $GOPATH/src/github.com/apigee/istio-mixer-adapter
        dep ensure 

4. Generate protos, build adapter, and run tests

        go generate ./...
        go build ./...
        go test ./...
   
## Testing in Mixer

### Build mixer with apigee-mixer-adapter (local copy)

1. install dependencies:

        go get github.com/lestrrat/go-jwx
        go get github.com/lestrrat/go-pdebug

2. put deps and apigee in mixer vendor dir:

        ln -s $GOPATH/src/github.com/lestrrat $ISTIO/istio/vendor/github.com/lestrrat
        ln -s $GOPATH/src/github.com/apigee $ISTIO/istio/vendor/github.com/apigee
        ln -s $ISTIO/istio/vendor/github.com/apigee/istio-mixer-adapter $GOPATH/src/github.com/apigee/istio-mixer-adapter
        mv $GOPATH/src/github.com/apigee/istio-mixer-adapter/vendor $GOPATH/src/github.com/apigee/istio-mixer-adapter/vendor.bak

3. patch $ISTIO/istio/mixer/adapter/inventory.yaml add:

        apigee: "github.com/apigee/istio-mixer-adapter/apigee"

4. patch $ISTIO/istio/mixer/template/inventory.yaml, add:

        ../../../../github.com/apigee/istio-mixer-adapter/template/analytics/template_proto.descriptor_set: "github.com/apigee/istio-mixer-adapter/template/analytics"

5. generate templates and make mixer:
        
        cd $ISTIO/istio
        go generate mixer/adapter/doc.go
        go generate mixer/template/doc.go
        make mixs

### Configure mixer

1. copy apigee config to mixer test directory:

        cp $GOPATH/src/github.com/apigee/istio-mixer-adapter/testdata/operatorconfig/config.yaml $ISTIO/istio/mixer/testdata/config/apigee.yaml

2. edit apigee config in mixer dir, set appropriate values:

      apigee_base: https://edgemicroservices.apigee.net/edgemicro/
      customer_base: http://myorg-myenv.apigee.net/edgemicro-auth
      org_name: myorg
      env_name: myenv
      key: mykey
      secret: mysecret
    
3. hack: patch mixer/testdata/config/attributes.yaml, add to `istio-proxy` manifest:

      request.auth.claims:
        valueType: STRING_MAP

### Launch Mixer (adjust for your platform)

    $GOPATH/out/darwin_amd64/release/mixs server --alsologtostderr \
        --configStoreURL=fs://$ISTIO/istio/mixer/testdata/config

### Sample commands

#### Mixer run

#### do auth check

    export API_KEY=<your api key>

    $GOPATH/out/darwin_amd64/release/mixc check \
        --string_attributes="destination.service=svc.cluster.local,request.path="/"" \
        --stringmap_attributes="request.headers=x-api-key:$API_KEY"

You should see "Check status was OK" if the API key is valid. 
If not, there's probably an issue with configuration.

    $GOPATH/out/darwin_amd64/release/mixc check \
        --string_attributes="destination.service=svc.cluster.local,request.path="/"" \
        --stringmap_attributes="request.headers=x-api-key:BAD_KEY"

You should see "Check status was PERMISSION_DENIED".  

#### send an analytics record

(Note: You'll likely want to adjust the timestamps.)

    $MIXER_DIR/bazel-bin/cmd/client/mixc report \
        --string_attributes='destination.service=svc.cluster.local,request.path="/"' \
        --stringmap_attributes="request.headers=x-api-key:$API_KEY" \
        --timestamp_attributes="request.time=2017-01-01T01:00:00Z,response.time=2017-01-01T01:01:00Z"


Analytics should show up in your org (may take several minutes depending on your account).
