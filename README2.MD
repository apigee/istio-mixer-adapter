# Full build and run in Mixer

## Build Custo Mixer

Instructions are on Goobunto Linux.

### Install prerequisites

    sudo apt-get update
    sudo apt-get install git-remote-google
    sudo goobuntu-add-repo -e cloud-sdk-trusty &&  sudo apt-get install google-cloud-sdk
    sudo apt-get install bazel
    
    sudo apt-get install docker-engine
    sudo ip link set docker0 down
    echo 'DOCKER_OPTS="${DOCKER_OPTS} --bip=192.168.9.1/24"' |  sudo tee --append /etc/default/docker
    sudo ip link del docker0
    echo 'DOCKER_OPTS="${DOCKER_OPTS} --graph=/usr/local/google/docker"' |  sudo tee --append /etc/default/docker
    sudo usermod --append --groups docker "$USER"
    gob-ctl create user/$USER/istio-mixer-adapter
    gob-ctl acl user/$USER/istio-mixer-adapter -reader mdbgroup/eng-mirror,mdbgroup/eng-intern-mirror
    
### Init gcloud
    
    gcloud init

### Get, build, and push Mixer docker 

    git clone https://github.com/theganyo/mixer.git
    cd mixer/docker
    git checkout istio-mixer-adapter
    bazel build //... 
    ./gcloud_build.sh


Instructions are on OS X using Minikube.

## Install Istio on Minikube

### Start Minikube

    minikube start --vm-driver=xhyve
    
Note: To latest connect to minikub docker env in any terminal, execute:    
    
    eval $(minikube docker-env)

### Install Istio

    curl -L https://git.io/getLatestIstio | sh -

Also add istioctl binary to your path.

### Update Istio.yaml to use custom Mixer

Edit <istio_dir>/install/kubernetes/istio.yaml (~line 273) to switch to custom Mixer:

        # image: gcr.io/istio-testing/mixer:0abf3496329789992df39f752a5af8e76686f2c7
        image: gcr.io/apigee-edgex-project/mixer:experiment

### Pull your custom Mixer image into docker  

    gcloud docker -- pull gcr.io/apigee-edgex-project/mixer:experiment
    
You must do this before installing istio or you will have image pull errors in docker. 
Alternatively, you could set up Docker to work with Google Container Registry.
     
### Install Istio into Kubernetes

    kubectl apply -f install/kubernetes/istio.yaml

## Set up Apigee

### Install Istio-managed test services

    kubectl apply -f <(istioctl kube-inject -f helloworld-service.yaml)
    kubectl apply -f <(istioctl kube-inject -f httpbin-service.yaml)

Assign the ingress URL to an env var for convenience 

    ingress=$(minikube service istio-ingress --url -n istio-system | head -n1)
    echo $ingress
    
Try it.
    
    curl -H "apikey: badkey" $ingress/hello
    
It should succeed as we haven't installed Apigee yet.

### Install apid and configure Apigee mixer adapter  

    cd istio-mixer-adapter/apid
    kubectl apply -f apid-service.yaml
    kubectl apply -f apigee-adapter.yaml
    kubectl apply -f apigee-handler.yaml

Try a test call again if you'd like.
    
    curl -H "apikey: badkey" $ingress/hello
    
It should still succeed as we haven't applied any rules yet.

### Apply Apigee apikey & analytics rule

    kubectl apply -f apigee-rule.yaml
    
This rule applies apikey and analytics to all calls.
    
Try it.
    
    curl -H "apikey: badkey" $ingress/hello
    curl -H "apikey: 4XxBlHNPEhEhiX23I305iayNfNdxPmGJ" $ingress/hello
    curl -H "apikey: 4XxBlHNPEhEhiX23I305iayNfNdxPmGJ" $ingress/get

The call with a bad apikey should now fail, the ones with a valid one succeed.
The calls will show up in the edge analytics (edgex01 org, helloworld proxy).
