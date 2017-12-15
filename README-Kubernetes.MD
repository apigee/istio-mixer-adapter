# Full build and run in Mixer

## Build Custom Mixer

Mixer build instructions are for Goobunto Linux.

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


## Install

Instructions are on OS X using Minikube w/ xhyve driver.

### Install Istio

    curl -L https://git.io/getLatestIstio | sh -

Also add istioctl binary to your path.

### Start Minikube w/ initializer support

    minikube start --vm-driver=xhyve --extra-config=apiserver.Admission.PluginNames="Initializers,NamespaceLifecycle,LimitRanger,ServiceAccount,DefaultStorageClass,ResourceQuota"
    
To connect to minikube docker env in a terminal window, execute:    
    
    eval $(minikube docker-env)

### Update Istio.yaml to use custom Mixer

Edit <istio_dir>/install/kubernetes/istio-auth.yaml (~line 267) to switch to custom Mixer:

        # image: gcr.io/istio-testing/mixer:5253b6b574a98b209c0ef3d0d6e90c1b8d6a5c2a
        image: gcr.io/apigee-edgex-project/mixer:experiment

### Pull the custom Mixer image that includes Apigee adapter into docker  

    gcloud docker -- pull gcr.io/apigee-edgex-project/mixer:experiment
    
You must do this before installing istio or you will have image pull errors in docker.
You must also have gcloud installed and configured prior to this step. 
(Alternatively, you could set up Docker to work with Google Container Registry.)  
     
### Install Istio components on Kubernetes

    kubectl apply -f install/kubernetes/istio-auth.yaml
    kubectl apply -f install/kubernetes/istio-initializer.yaml
    
## Set up Apigee

### Install Istio-managed test services

    cd <istio-mixer-adapter>/apid
    kubectl apply -f helloworld-service.yaml
    kubectl apply -f httpbin-service.yaml

Assign the ingress URL to an env var for convenience 

    ingress=$(minikube service istio-ingress --url -n istio-system | head -n1)
    echo $ingress
    
Try it.
    
    curl -H "apikey: badkey" $ingress
    
It should succeed as we haven't installed Apigee yet.

### Install apid and configure Apigee mixer adapter  

    kubectl apply -f apid-service.yaml
    kubectl apply -f apigee-adapter.yaml
    kubectl apply -f apigee-handler.yaml

Try a test call again if you'd like.
    
    curl -H "apikey: badkey" $ingress
    
It should still succeed as we haven't applied any rules yet.

### Define Istio API and apply rule

    kubectl apply -f api-spec.yaml
    kubectl apply -f apigee-rule.yaml
    
This rule applies apikey and analytics to all calls with label "gateway: apigee"
    
Try it.
    
    curl $ingress/get
    curl -H "apikey: badkey" $ingress/get
    curl -H "apikey: 4XxBlHNPEhEhiX23I305iayNfNdxPmGJ" $ingress/get
    curl -H "apikey: 4XxBlHNPEhEhiX23I305iayNfNdxPmGJ" $ingress

The call with a no apikey or a bad apikey should now fail, the valid one should succeed.
The calls will show up in the edge analytics (edgex01 org, helloworld proxy).
