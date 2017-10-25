# Full build and run in Mixer

## Build - Goobunto Linux

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


## Install Mixer - Minikube on OS X

### Start Minikube

    minikube start --vm-driver=xhyve
    eval $(minikube docker-env)

### Install Istio

    curl -L https://git.io/getLatestIstio | sh -

Also add istioctl binary to your path.

### Update Istio.yaml to use custom Mixer

Edit <istio_dir>/install/kubernetes/istio.yaml (~line 273) to switch to custom Mixer:

        # image: gcr.io/istio-testing/mixer:0abf3496329789992df39f752a5af8e76686f2c7
        image: gcr.io/apigee-edgex-project/mixer:experiment

### Pull your custom Mixer image into docker

(Alternatively, you could set up Docker to work with Google Container Registry.)  

    gcloud docker -- pull gcr.io/apigee-edgex-project/mixer:experiment
     
### Install Istio

    kubectl apply -f install/kubernetes/istio.yaml
    
### Install apid and configure Apigee mixer adapter  

    cd istio-mixer-adapter/apid
    kubectl apply -f apid-service.yaml
    kubectl apply -f adapter-config.yaml

### Install Helloworld service (with Istio)

    kubectl apply -f <(istioctl kube-inject -f helloworld-service.yaml)
    helloworld=$(minikube service helloworld --url)
    
Try it.
    
    curl -H "apikey: 4XxBlHNPEhEhiX23I305iayNfNdxPmGJx" $helloworld/hello
    curl -H "apikey: badkey" $helloworld/hello
    
Both should succeed.

### Apply Apigee apikey & Analytics rule

    kubectl apply -f helloworld-all.yaml
    
(ToDo: currently matches *all* calls, restrict.)
    
Try it.
    
    curl -H "apikey: 4XxBlHNPEhEhiX23I305iayNfNdxPmGJx" $helloworld/hello
    curl -H "apikey: badkey" $helloworld/hello

The 2nd call should now fail.
