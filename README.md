# Istio Apigee Adapter

[![CircleCI](https://circleci.com/gh/apigee/istio-mixer-adapter.svg?style=shield)](https://circleci.com/gh/apigee/istio-mixer-adapter)
[![Go Report Card](https://goreportcard.com/badge/github.com/apigee/istio-mixer-adapter)](https://goreportcard.com/report/github.com/apigee/istio-mixer-adapter)
[![codecov.io](https://codecov.io/github/apigee/istio-mixer-adapter/coverage.svg?branch=master)](https://codecov.io/github/apigee/istio-mixer-adapter?branch=master)

This is the source repository for Apigee's Istio Mixer Adapter. This allows users of Istio to
incorporate Apigee Authentication, Authorization, and Analytics policies to protect and
report through the Apigee UI.

To join the Apigee pre-release program for additional documentation and support, please contact:
<anchor-prega-support@google.com>.

---

# Installation and usage

## Version note

The current release is based on Istio 0.8. The included sample files and instructions below will 
automatically install the correct Istio version for you onto Kubernetes. It is recommended that
you install onto Kubernetes 1.9 or newer. See the [Istio](https://istio.io) web page for more information.  

## Prerequisite: Apigee

You must have an [Apigee Edge](https://login.apigee.com) account. If you need one, you can create one [here](https://login.apigee.com/sign_up).

## Download a Release

Istio Mixer Adapter releases can be found [here](https://github.com/apigee/istio-mixer-adapter/releases).

Download the appropriate release package for your operating system and extract it. You should a file list similar to:

    LICENSE
    README.md
    samples/apigee/authentication-policy.yaml
    samples/apigee/definitions.yaml
    install/handler.yaml
    install/httpapispec.yaml
    install/rule.yaml
    install/authentication-policy.yaml
    samples/istio/helloworld.yaml
    samples/istio/istio-demo.yaml
    samples/istio/istio-demo-auth.yaml
    apigee-istio

`apigee-istio` (or apigee-istio.exe on Windows) is the Command Line Interface (CLI) for this project. 
You may add it to your PATH for quick access - or remember to specify the path for the commands below.

The yaml files in the samples/ directory contain the configuration for Istio and the adapter. We discuss 
these below.

## Provision Apigee for Istio

The first thing you'll need to do is provision your Apigee environment to work with the Istio adapter. 
This will install a proxy, set up a certificate, and generate some credentials for you:  

    apigee-istio -u {your username} -p {your password} -o {your organization name} -e {your environment name} provision

You should see a response like this:

    verifying internal proxy...
      ok: https://istioservices.apigee.net/istio/analytics/organization/myorg/environment/myenv
      ok: https://istioservices.apigee.net/istio/quotas/organization/myorg/environment/myenv
    verifying customer proxy...
      ok: https://myorg-myenv.apigee.net/istio-auth/certs
      ok: https://myorg-myenv.apigee.net/istio-auth/products
      ok: https://myorg-myenv.apigee.net/istio-auth/verifyApiKey
    
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

That last block is important: It's the configuration for your handler. We'll use it in the next section.

Notes:

* `apigee-istio` will automatically pick up the username and password from a 
[.netrc](https://ec.haxx.se/usingcurl-netrc.html) file in your home directory if you have an entry for 
`machine api.enterprise.apigee.com`.
* For Apigee Private Cloud (OPDK), you'll need to also specify your `--managementBase` in the command.
In this case, the .netrc entry should match this host. 

### Set your configuration 

Edit `samples/apigee/handler.yaml` and replace the configuration with what you got during provisioning above. 
You may simply replace the contents of the entire file with that configuration block.

## Install Istio with Apigee mixer

Be sure both your Kubernetes cluster and `kubectl` CLI are ready to use. Two sample Istio install files have 
been provided for you in the release as a convenience. 

To start Istio without mutual TLS enabled between services, you can simply run:

     kubectl apply -f samples/istio/istio-demo.yaml
     
Or with mutual TLS enabled with:

    kubectl apply -f samples/istio/istio-demo-auth.yaml
    
Note: The key difference between these files and the ones provided with Istio is simply that the pointer 
to the `docker.io/istio/mixer` docker image in the original files have been replaced with a custom build 
that includes the Apigee adapter.
 
You should soon be able to now see all the Istio components running in your Kubernetes cluster:

    kubectl get pods -n istio-system
    
Be sure istio-pilot, istio-ingressgateway, istio-policy, istio-telemetry, and istio-citadel are running
before continuing. More information on verifying the Istio installation is
[here](https://istio.io/docs/setup/kubernetes/quick-start/#verifying-the-installation).
 
## Install a target service

We'll install a simple [Hello World](https://github.com/istio/istio/tree/master/samples/helloworld)
application.

    kubectl apply -f samples/istio/helloworld.yaml
    
You should be able to verify two instances are running:

    kubectl get pods

And now you should be able to access the service successfully:

    curl http://${GATEWAY_URL}/hello

## Configure Apigee Mixer in Istio

Now that Istio is running, it's time to add Apigee policies. Apply the Apigee configuration:

        kubectl apply -f apigee-definitions.yaml
        kubectl apply -f apigee-handler.yaml
        kubectl apply -f apigee-rule.yaml

Once this has been done, you should no longer be able to access your `helloworld` service as it 
will now be protected per your Apigee policy. If you curl it:

    curl http://${GATEWAY_URL}/hello
    
You should receive a permission denied error:

    PERMISSION_DENIED:apigee-handler.apigee.istio-system:missing authentication
    
Great! Now if only you had some credentials. Let's fix that...

### Configure your policies as an Apigee API Product

[Create an API Product](https://apigee.com/apiproducts) in your Apigee organization:

* Give the API Product definition a name (`helloworld` is good).
* Select all environment(s).
* Set the Quota to `5` requests every `1` `minute`.
* Add a Path with the `+Custom Resource` button. Set the path to `/`.
* Save the product.

[Create a Developer](https://apigee.com/developers) using any values you wish.

[Create an App](https://apigee.com/apps):

* Give your App a name (`helloworld` is good)
* Select the developer you just created
* Add your API Product with the `+Product` button and save.
* Save

Still on the App page, you should now see a `Credentials` section. Click the `Show` button 
under the `Consumer Key` heading. Copy that key!

### Bind your Apigee API Product to your service

Now that you have a policy, we just need to bind it to your Istio service:

    apigee-istio -o {your organization name} -e {your environment name} bindings add helloworld.default.svc.cluster.local  helloworld

By the way, a handy way to see the policies bound to your services is using the `bindings list` command:

    apigee-istio -o {your organization name} -e {your environment name} bindings list

### Access helloworld with your key

You should now be able to access the helloworld service in Istio by passing the key you  
copied from your Apigee app above. Just send it as part of the header:
    
    curl http://${GATEWAY_URL}/hello -H "x-api-key: {your consumer key}"

This call should now be successful. Your authentication policy works!

### Hit your Quota

Remember that Quota you set for 5 requests per minute? Let's max it out.

Make the same request you did above just a few more times:

    curl http://${GATEWAY_URL}/hello -H "x-api-key: {your consumer key}"

If you're in a Unix shell, you can use repeat:

    repeat 10 curl http://${GATEWAY_URL}/hello -H "x-api-key: {your consumer key}"
    
Either way, you should see some successful calls... followed by failures that look like this:

    RESOURCE_EXHAUSTED:apigee-handler.apigee.istio-system:quota exceeded 

(Did you see mixed successes and failures? That's OK. The Quota system is designed to have 
very low latency for your requests, so it uses a cache that is _eventually consistent_ with 
the remote server. Client requests don't wait for the server to respond and you could have 
inconsistent results for a second or two, but it will be worked out quickly and no clients 
have to wait in the meantime.) 

### Bonus: Use JWT authentication instead of API Keys

Update the `install/authentication-policy.yaml` file to set correct URLs for your environment:

    origins:
    - jwt:
        issuer: https://{your organization}-{your environment}.apigee.net/istio-auth/token
        jwks_uri: https://{your organization}-{your environment}.apigee.net/istio-auth/certs

The hostname (and ports) for these URLs should mirror what you used for `customer_base` config above
(adjust as appropriate if you're using OPDK).

Now, apply your Istio authentication policy:

    istioctl create -f samples/apigee/authentication-policy.yaml

Now calls to `helloworld` (with or without the API Key):

    curl http://${GATEWAY_URL}/hello
    
Should receive an auth error similar to:

    Origin authentication failed.

We need a JWT token. So have `apigee-istio` get you a JWT token:

    apigee-istio token create -o {your organization} -e {your environment} -i {your key} -s {your secret}
    
Or, you can do it yourself through the API: 

    curl https://{your organization}-{your environment}.apigee.net/istio-auth/token -d '{ "client_id":"{your key}", "client_secret":"your secret", "grant_type":"client_credentials" }' -H "Content-Type: application/json"

Now, try again with your newly minted JWT token:

    curl http://${GATEWAY_URL}/hello -H "Authorization: Bearer {your jwt token}"

This call should now be successful.

### Check your analytics

Head back to the [Apigee Edge UI](https://apigee.com/edge).

Click the `Analyze` in the menu on the left and check out those analytics.

---

To join the Apigee pre-release program for additional documentation and support, please contact:
<anchor-prega-support@google.com>.
