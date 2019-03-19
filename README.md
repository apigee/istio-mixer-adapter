# Istio Apigee Adapter

[![CircleCI](https://circleci.com/gh/apigee/istio-mixer-adapter.svg?style=shield)](https://circleci.com/gh/apigee/istio-mixer-adapter)
[![Go Report Card](https://goreportcard.com/badge/github.com/apigee/istio-mixer-adapter)](https://goreportcard.com/report/github.com/apigee/istio-mixer-adapter)
[![codecov.io](https://codecov.io/github/apigee/istio-mixer-adapter/coverage.svg?branch=master)](https://codecov.io/github/apigee/istio-mixer-adapter?branch=master)

This is the source repository for Apigee's Istio Mixer Adapter. This allows users of Istio to
incorporate Apigee Authentication, Authorization, and Analytics policies to protect and
report through the Apigee UI.

A Quick Start Tutorial continues below, but complete Apigee documentation on the concepts and usage of this adapter is also available on the [Apigee Adapter for Istio](https://docs.apigee.com/api-platform/istio-adapter/concepts) site. For more information and product support, please [contact Apigee support](https://apigee.com/about/support/portal).

---

# Installation and usage

## Prerequisite: Apigee

You must have an [Apigee Edge](https://login.apigee.com) account. If needed, you may create one [here](https://login.apigee.com/sign_up).

## Prerequisite: Istio

### Istio 1.1

As of Istio 1.1, a custom Mixer image in no longer required. Instead, the Apigee Adapter can simply run as a
stand-alone service and configuration can tell Mixer how to call it. So, choose your favorite way of installing Istio
and [have at it](https://istio.io/docs/setup/).

_Important_  
A key feature of the Apigee adapter that we'll be exploring below is to automatically enforce Apigee policy in Istio 
using Istio's Mixer. However, as of Istio 1.1, policy is not enabled by default. For Apigee policy features to take 
effect, policy control *must be explicitly enabled* in Istio config and the Mixer policy image must be running. See  
[Enabling Policy Enforcement](https://istio.io/docs/tasks/policy-enforcement/enabling-policy/) for more details.

[next step](#download-a-mixer-adapter-release)

### Istio 1.0

If installing onto Istio 1.0, your mesh will need run a custom Mixer (policy and telemetry) image that contains the Apigee 
adapter. There is a release of the Apigee Adapter Mixer corresponding to each release of Istio 1.0 and you should use 
the matching version label. For example, for Istio 1.0.6 you would use the following image:

    MIXER_IMAGE=gcr.io/apigee-api-management-istio/istio-mixer:1.0.6

There are many ways to set the appropriate image for your mesh that depend upon your specific installation of Istio, but 
a couple of common examples are below: 

#### Upgrade Istio running on Kubernetes

If you're using Kubernetes, the simplest method to get started with Istio 1.0 is to simply install it, then replace the Mixer 
image with the correct one containing the Apigee adapter. There is a release of the Apigee Adapter Mixer corresponding to
each release of Istio 1.0 and you should use the matching version label. For example, for Istio 1.0.6, you could run
the following commands against the Kubernetes in your Istio mesh:

    kubectl -n istio-system set image deployment/istio-telemetry mixer=${MIXER_IMAGE}
    kubectl -n istio-system set image deployment/istio-policy mixer=${MIXER_IMAGE}

#### Install Istio 1.0 onto Kubernetes using Helm

Follow the official Istio [Helm Install](https://istio.io/docs/setup/kubernetes/helm-install) instructions, but add
a `--set` parameter to the `helm` command referencing the appropriate Mixer image.

Option 1: Helm Template

    helm template install/kubernetes/helm/istio --name istio --namespace istio-system --set mixer.image=${MIXER_IMAGE} > istio.yaml

Option 2: Helm and Tiller

    helm install install/kubernetes/helm/istio --name istio --namespace istio-system --set mixer.image=${MIXER_IMAGE} 

NOTE: You may also change the container name to `istio-mixer-debug` if you need a Mixer container with debug tools. 

## Download a Mixer Adapter Release

Istio Mixer Adapter releases can be found [here](https://github.com/apigee/istio-mixer-adapter/releases).

Download the appropriate release package for your operating system and Istio version and extract it. 
You should a top-level list similar to:

    LICENSE
    README.md
    install/
    samples/
    apigee-istio

`apigee-istio` (or apigee-istio.exe on Windows) is the Command Line Interface (CLI) for this project. 
Add it to your PATH for quick access - or remember to specify the path for the commands below.

The files in the samples/ and install/ directories contain the configuration for Istio and the adapter. We discuss 
these below.

## Provision Apigee for Istio

The first thing you'll need to do is provision your Apigee environment to work with the Istio adapter. The `provision` 
command will install a proxy into Apigee if necessary, set up a certificate on Apigee, and generate some credentials the 
Adapter will use to securely connect back to Apigee.

_Upgrading_  
By default, running `provision` will not attempt to install a new proxy into Apigee if one already exists. If 
you are upgrading from a prior release, add the `--forceProxyInstall` option to the commands below to ensure that the 
latest Apigee proxy is installed for your organization.

_OPDK_  
If you are running Apigee Private Cloud (OPDK), you'll need to also specify your private server's `--managementBase` 
and `--routerBase` options in the command. The URIs must be reachable from your Istio mesh.  

_Credentials_  
`apigee-istio` will automatically pick up the username and password from a 
[.netrc](https://ec.haxx.se/usingcurl-netrc.html) file in your home directory (or where you specify with the `--netrc` 
option) if you have an entry for `machine api.enterprise.apigee.com` (or the host you specified for OPDK).


### Istio 1.1

To create an Istio 1.1 handler file, run the following:

    apigee-istio --grpc -u {username} -p {password} -o {organization} -e {environment} provision > samples/apigee/grpc/handler.yaml

Once it completes, check your `samples/apigee/grpc/handler.yaml` file. It should look similar to this:

    # Istio handler configuration for Apigee gRPC adapter for Mixer
    apiVersion: config.istio.io/v1alpha2
    kind: apigee
    metadata:
      name: apigee-handler
      namespace: istio-system
    spec:
      adapter: apigee
      connection:
        address: apigee-adapter:5000
      params:
        apigee_base: https://istioservices.apigee.net/edgemicro
        customer_base: https://myorg-env.apigee.net/istio-auth
        org_name: myorg
        env_name: myenv
        key: 06a40b65005d03ea24c0d53de69ab795590b0c332526e97fed549471bdea00b9
        secret: 93550179f344150c6474956994e0943b3e93a3c90c64035f378dc05c98389633   

As noted above, Istio 1.1 adapters are run in a separate process from Mixer and Mixer will connect to the adapter 
via gRPC to the address specified in the `connection.address` property in the Apigee adapter handler config. This 
address must be reachable by the Mixer processes in the Istio mesh. If you deploy the adapter to a location other than
the default, just change the `connection.address` value as appropriate.

[next step](#install-a-target-service)

### Istio 1.0

To create an Istio 1.0 handler file, run the following:

    apigee-istio -u {username} -p {password} -o {organization} -e {environment} provision > samples/apigee/handler.yaml

Once it completes, check your `samples/apigee/handler.yaml` file. It should look similar to this:

    # istio handler configuration for apigee adapter
    apiVersion: config.istio.io/v1alpha2
    kind: apigee
    metadata:
      name: apigee-handler
      namespace: istio-system
    spec:
      apigee_base: https://istioservices.apigee.net/edgemicro
      customer_base: https://myorg-myenv.apigee.net/istio-auth
      org_name: myorg
      env_name: myenv
      key: 06a40b65005d03ea24c0d53de69ab795590b0c332526e97fed549471bdea00b9
      secret: 93550179f344150c6474956994e0943b3e93a3c90c64035f378dc05c98389633


## Install a target service

Next, we'll install a simple [Hello World](https://github.com/istio/istio/tree/master/samples/helloworld)
service into the Istio mesh as a target. From your Istio directory: 

    kubectl label namespace default istio-injection=enabled    
    kubectl apply -f samples/helloworld/helloworld.yaml
    
If you're using Istio 1.1, you'll also need to add the gateway:

    kubectl apply -f samples/helloworld/helloworld-gateway.yaml
    
You should be able to verify two `helloworld` instances are running:

    kubectl get pods

And you should be able to access the service successfully:

    curl http://${GATEWAY_URL}/hello

Note: If you don't know your GATEWAY_URL, you'll need to follow [these instructions](
https://istio.io/docs/tasks/traffic-management/ingress/#determining-the-ingress-ip-and-ports) to set
the INGRESS_IP and INGRESS_PORT variables. Then, your GATEWAY_URL can be set with:
 
    export GATEWAY_URL=$INGRESS_HOST:$INGRESS_PORT

## Configure Istio for the Apigee Adapter 

Now it's time to install Apigee policy onto Istio. Once again, the configuration varies slightly depending on Istio version.

### Istio 1.1

    kubectl apply -f samples/apigee/grpc

[next step](#authentication-test)

### Istio 1.0

    kubectl apply -f samples/apigee/definitions.yaml
    kubectl apply -f samples/apigee/handler.yaml
    kubectl apply -f samples/apigee/rule.yaml

## Authentication Test

Istio should now be fully configured for Apigee control - and you should no longer be able to access your target 
`helloworld` service without authentication. If you curl it:

    curl http://${GATEWAY_URL}/hello
    
You should receive a permission denied error:

    PERMISSION_DENIED:apigee-handler.apigee.istio-system:missing authentication
    
The service is now protected by Apigee. Great! But now you've locked yourself out without a key. 
If only you had credentials. Let's fix that.

_Debugging_  
If you're certain you've applied everything correctly up to this point but are still able to reach the target service, 
please check [the Github wiki](https://github.com/apigee/istio-mixer-adapter/wiki) for troubleshooting tips.  

## Apigee policies

Policy is defined by Apigee API Products and enforced by the Apigee Adapter. Let's create an API Product, Developer,
and App to see how this works.

1. [Create an API Product](https://apigee.com/apiproducts) in your Apigee organization:

* Give the `API Product` definition a name (`helloworld` is fine).
* Select your environment(s).
* Set the Quota to `5` requests every `1` `minute`.
* Add a Path with the `+Custom Resource` button. Set the path to `/`.
* Add the service name `helloworld.default.svc.cluster.local` to `Istio Services`. 
* Save

(Note: You can also use the `apigee-istio bindings` command to control Istio service bindings.)

2. [Create a Developer](https://apigee.com/developers). Use any values you wish. Be creative!

3. [Create an App](https://apigee.com/apps) to grant your Developer access to your API Product:

* Give your `App` a name (`helloworld` is fine).
* Select the `Developer` you just created above.
* Add your `API Product` with the `+Product` button.
* Save

Still on the `App` page, you should now see a `Credentials` section. Click the `Show` button 
under the `Consumer Key` heading. That's your API Key. Your Developer includes this key in requests to Apigee to
establish his or her identify. Copy that key!

## Authentication with API Key

Now you can access your target service by passing the key you received above by including it in the `x-api-key` header:
    
    curl http://${GATEWAY_URL}/hello -H "x-api-key: {your consumer key}"

The call should now be successful. You're back in business and your authentication policy works!

_Note_  
There may be some latency in configuration. The API Product information refreshes from Apigee 
every two minutes. In addition, configuration changes you make to Istio will take a few moments to 
be propagated throughout the mesh. During this time, you could see inconsistent behavior as it takes
effect.

## Hit your Quota

Remember that API Product Quota you set limiting requests from Developer to 5 requests per minute? Let's max it out.

Make the same authenticated request you did above a few more times:

    curl http://${GATEWAY_URL}/hello -H "x-api-key: {your consumer key}"

If you're in a Unix shell, you can use repeat:

    repeat 10 curl http://${GATEWAY_URL}/hello -H "x-api-key: {your consumer key}"
    
Either way, you should see some successful calls... followed by failures like this:

    RESOURCE_EXHAUSTED:apigee-handler.apigee.istio-system:quota exceeded 

_Note_  
Did you see mixed successes and failures? That's OK. The Quota system is designed to have 
very low latency for your requests, so it uses a cache that is _eventually consistent_ with 
the remote server. Client requests don't wait for the server to respond and you could have 
inconsistent results for a second or two, but it will be worked out quickly and no clients 
have to wait in the meantime. 

## Bonus: Authentication with JWT tokens

We don't always want to hand out API Keys to developers, they don't expire and require a call to the server to validate.
Let's look at using a JWT Token instead.  

Update the `samples/apigee/authentication-policy.yaml` file to set correct URLs for your environment. The hostname 
and ports for these URLs should mirror what you used for `customer_base` config above (adjust as appropriate 
if you're using OPDK).

    peers:
    # - mtls:   # uncomment if you're using mTLS between services in your mesh
    origins:
    - jwt:
        issuer: https://{your organization}-{your environment}.apigee.net/istio-auth/token
        jwks_uri: https://{your organization}-{your environment}.apigee.net/istio-auth/certs

_Important_  
The mTLS authentication settings for your mesh and your authentication policy must match. In other words, 
if you set up Istio to use mTLS connections, you must enable mTLS by uncommenting the mTLS the `mtls` line in the 
authentication-policy.yaml file. (If the policies do not match, you will see errors similar to: `upstream connect 
error or disconnect/reset before headers`.) 
 
Save your changes and apply your Istio authentication policy:

    kubectl apply -f samples/apigee/authentication-policy.yaml

Now, calls to `helloworld` - even with the API Key:

    curl http://${GATEWAY_URL}/hello -H "x-api-key: {your consumer key}"
    
Should receive an auth error similar to:

    Origin authentication failed.

We need a JWT token to succeed. We can use `apigee-istio` to get a JWT token:

    apigee-istio token create -o {your organization} -e {your environment} -i {your key} -s {your secret}
    
Or, you can do it yourself through the API: 

    curl https://{your organization}-{your environment}.apigee.net/istio-auth/token -d '{ "client_id":"your key", "client_secret":"your secret", "grant_type":"client_credentials" }' -H "Content-Type: application/json"

Now, try again with your newly minted JWT token:

    curl http://${GATEWAY_URL}/hello -H "Authorization: Bearer {your jwt token}"

This call should now be successful.

## Check your analytics

One more thing: Let's head back to the [Apigee Edge UI](https://apigee.com/edge).

Click the `Analyze` in the menu on the left and check out some of the nifty analytics information on your requests!

---

To join the Apigee pre-release program for additional documentation and support, please contact 
[apigee support](https://apigee.com/about/support/portal)

