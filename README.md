# Istio Apigee Adapter

This workspace holds an Apigee adapter for Istio Mixer. It can be tested standalone as noted below.
Instructions for building and running in Kubernetes are available here: [README-Kubernetes.MD]().

Note: This repo should be in $GOPATH/src/github.com/apigee/istio-mixer-adapter

Note: You can only build Istio Docker images on Linux.

## Building and testing standalone

1. Install protoc and dep prerequisites:

        [](https://developers.google.com/protocol-buffers/docs/downloads) 
        [](https://github.com/golang/dep)

2. Run install script.

        $GOPATH/src/github.com/apigee/istio-mixer-adapter/install/local_install.sh

## Testing in Mixer

### Configure mixer

Run local mixer via install script:

        export APIGEE_ORG=my-org
        export APIGEE_ENV=my-env
        $GOPATH/src/github.com/apigee/istio-mixer-adapter/install/run_local.sh

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

## Deploying Mixer

Note from above: you can only do this on Linux.

1. First, build the docker image:

        export GCP_PROJECT=my-gcp-project
        $GOPATH/src/github.com/apigee/istio-mixer-adapter/install/build_mixer_docker.sh

2. Next, push to GKE:

        export GCP_PROJECT=my-gcp-project
        $GOPATH/src/github.com/apigee/istio-mixer-adapter/install/push_docker_to_gke.sh

3. Go to [Pantheon](https://pantheon.corp.google.com/kubernetes/workload), you
   should see the mixer running there.
