# Istio Apigee Adapter

This workspace holds an apigee adapter for Istio. It can be tested by itself, but in order
to really use it you need a build of the Istio mixer that pulls it in. Instructions for that
are forthcoming.

## Building

1. Install [Bazel](https://bazel.build/)
2. Build adapter

        bazel build //...

(TODO: Not working. Standalone Bazel build fails because of odd relative references required by Mixer build.)   

## Start APID

* Get your org enabled for hybrid on Apigee (contact Apigee)
* Create a cluster on Apigee (see Apigee docs)
* Build apid docker image

        cd apid
        docker build --no-cache --tag your-repo/apid .
        docker push your-repo/apid

* Edit apid/apid-service.yaml

    * Change `image: scottganyo/apid:latest` to point to `your-repo/apid`
    * Change `apid_apigeesync_consumer_key`, `apid_apigeesync_consumer_secret`, and `apid_apigeesync_cluster_id` to match your cluster config
 
* Deploy apid to Kubernetes

        kubectl apply -f apid-service.yaml

In Minikube, you can retrieve `apid_base` via `minikube service apid --url` 

## Testing in Mixer

### Build mixer with apigee-mixer-adapter

        apply testdata/apigee-mixer-adapter.patch to mixer repo
        edit mixer/WORKSPACE to adjust path to apigee-mixer-adapter repo 
        bazel build //...

### Configure mixer

Edit `testdata/global/adapters.yml` to specify your `apid_base` and `org`.
Edit `operatorconfig/config.yml` to specify your `apid_base` and `org`.

### Launch Mixer

    export MIXER_DIR=<your mixer dir>
    export ADAPTER_DIR=<your adapter dir>

    $MIXER_DIR/bazel-bin/cmd/server/mixs server --alsologtostderr \
        --configStore2URL=fs://$ADAPTER_DIR/testdata/operatorconfig \
        --configStoreURL=fs://$MIXER_DIR

### Sample commands

#### Mixer run

#### do auth check

    export API_KEY=<your api key>

    $MIXER_DIR/bazel-bin/cmd/client/mixc check \
        --string_attributes="destination.service=svc.cluster.local,request.path="/"" \
        --stringmap_attributes="request.headers=apikey:$API_KEY"

You should see "OK" if the API key is valid. If not, there's probably an issue with configuration.

    $MIXER_DIR/bazel-bin/cmd/client/mixc check \
        --string_attributes="destination.service=svc.cluster.local,request.path="/"" \
        --stringmap_attributes="request.headers=apikey:BAD_KEY"

You should see "OK" if the API key is valid. If not, there's probably an issue with configuration.

#### send an analytics record

Note: You'll likely want to adjust the timestamps.

    $MIXER_DIR/bazel-bin/cmd/client/mixc report \
        --string_attributes='destination.service=svc.cluster.local,request.path="/"' \
        --stringmap_attributes="request.headers=apikey:$API_KEY" \
        --timestamp_attributes="request.time=2017-01-01T01:00:00Z,response.time=2017-01-01T01:01:00Z"


Analytics should show up in your org. (TODO: Not working.)
