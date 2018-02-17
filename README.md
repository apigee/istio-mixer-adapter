# Istio Apigee Adapter

This workspace holds an Apigee adapter for Istio Mixer. It can be tested standalone as noted below.
Instructions for building and running in Kubernetes are available here: [README-Kubernetes.MD]().

## Building and testing standalone

1. Install dep

        [](https://github.com/golang/dep) 

2. Install dependencies

        dep ensure 

3. Generate protos, build adapter, and run tests

        go generate ./...
        go build ./...
        go test ./...
   
## Testing in Mixer

### Build mixer with apigee-mixer-adapter

1. clone istio

        cd $GOPATH/
        git clone https://github.com/istio/istio
        export ISTIO=$GOPATH/src/istio.io

2. install dependencies:

        go get github.com/lestrrat/go-jwx
        go get github.com/lestrrat/go-pdebug

3. put deps and apigee in mixer vendor dir:

        ln -s $GOPATH/src/github.com/lestrrat $ISTIO/istio/vendor/github.com/lestrrat
        ln -s $GOPATH/src/github.com/apigee $ISTIO/istio/vendor/github.com/apigee
        mv $ISTIO/istio/vendor/github.com/apigee/vendor $ISTIO/istio/vendor/github.com/apigee/vendor.bak

4. patch mixer/adapter/inventory.yaml add:

        apigee: "github.com/apigee/istio-mixer-adapter/apigee"

5. patch mixer/template/inventory.yaml, add:

        ../../../../github.com/apigee/istio-mixer-adapter/template/analytics/template_proto.descriptor_set: "github.com/apigee/istio-mixer-adapter/template/analytics"

6. generate templates and make mixer:
        
        go generate ./...
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
