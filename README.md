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

## Install Istio with Apigee mixer

The quickest way to get started is to follow the [Istio Kubernetes Quick Start](https://istio.io/docs/setup/kubernetes/quick-start.html).

Before you install Istio into Kubernetes (step 5: Install Istio’s core components), edit the `istio.yaml` or `istio-auth.yaml` file to point to the Apigee mixer instead of the Istio mixer. Choose one below.

### Current Release: 1.0.0-alpha-1

This is the Apigee supported pre-release. 

Install on base of Istio 0.7.1.

Find: `docker.io/istio/mixer:0.7.1` in `istio.yaml` and replace with the following:

    gcr.io/apigee-api-management-istio/istio-mixer:1.0.0-alpha-1

### Nightly Build (unsupported)

May contain new features or bug fixes, but may also be broken. Use at your own risk or with guidance. 

Install on base of Istio 0.7.1.

Find: `docker.io/istio/mixer:0.7.1` in `istio.yaml` and replace with the following:

    gcr.io/apigee-api-management-istio/istio-mixer:nightly

## Install your service

For the following, we'll assume you've installed Istio's [Hello World](https://github.com/istio/istio/tree/master/samples/helloworld).

You should be able to access this service successfully:

    curl http://$HELLOWORLD_URL/hello

## Set up Apigee

1. If you don't have one, get yourself an [Apigee Edge](https://login.apigee.com) account.
2. [Install Edge Microgateway](https://docs.apigee.com/api-platform/microgateway/2.5.x/installing-edge-microgateway)
3. [Configure Edge Microgateway](https://docs.apigee.com/api-platform/microgateway/2.5.x/setting-and-configuring-edge-microgateway#Part1) - Just Part 1 and stop!

At this point, you should have this information:

* organization name
* environment name
* key
* secret

## Configure Istio and Apigee Mixer

### Get the release

#### Current Release: 1.0.0-alpha-1

Download from [here](https://github.com/apigee/istio-mixer-adapter/releases/tag/1.0.0-alpha-1).

Once extracted, you will find several .yaml files for configuring Istio in the install directory. Follow along below for configuration. 

### Set your configuration 

Edit `install/apigee-handler.yaml` and replace the configuration values with your own:

      customer_base: https://{your organization}-{your environment}.apigee.net/edgemicro-auth
      org_name: {your organization name}
      env_name: {your environment name}
      key: {your key}
      secret: {your secret}

Note: If you're using Apigee OPDK, you'll need to point `customer_base` and `apigee_base` to your 
local installation instead of apigee.net.

Now, apply the Apigee configuration to Istio:

        kubectl apply -f apigee-definitions.yaml
        kubectl apply -f apigee-handler.yaml
        kubectl apply -f apigee-rule.yaml
        kubectl apply -f api-spec.yaml

At this point, you should no longer be able to access your helloworld service. If you curl it:

    curl http://$HELLOWORLD_URL/hello
    
You should receive a permission denied error:

    PERMISSION_DENIED:apigee-handler.apigee.istio-system:missing authentication
    
Let's fix that...

### Configure Apigee

Create an [API Product](https://apigee.com/apiproducts) in your Apigee organization:

* Give the API Product definition a name (`helloworld` is fine).
* Make sure the correct environment is checked.
* Set the Quota to `5` requests every `1` `minute`.
* Add a Path with the `+Custom Resource` button. Set it to `/`.
* Add an Attribute with the `+Custom Attribute` button. Set the `key` to `istio-services` and the `value` to `helloworld.default.svc.cluster.local`. 
* Save

Create a [Developer](https://apigee.com/developers)
* Use any values you want

Create an [App](https://apigee.com/apps)
* Give your App a name (`helloworld` is fine)
* Select your developer
* Add your API Product with the `+Product` button.
* Save

Still on the App page, you should now see a `Credentials` section. Click the `Show` button 
under the `Consumer Key` heading. Copy that key!

### Access helloworld with your key

You should now be able to access the helloworld service in Istio by passing the key you just 
copied from your Apigee app. Just send it as part of the header:
    
    curl http://$HELLOWORLD_URL/hello -H "x-api-key: {your consumer key}"

This call should now be successful.

### Quota: Check your quota (unreleased: nightly build only)

Remember that Quota you set for 5 requests per minute? Let's max it out.

Make that same request a few more times:

    curl http://$HELLOWORLD_URL/hello -H "x-api-key: {your consumer key}"

Or, if you're in a Unix shell, you can use repeat:

    repeat 10 curl http://$HELLOWORLD_URL/hello -H "x-api-key: {your consumer key}"
    
Either way, you should see successful calls then failures that look like this:

    RESOURCE_EXHAUSTED:apigee-handler.apigee.istio-system:quota exceeded 

(Oh, did you see mixed successes and failures? That's OK! The Quota system is designed to have 
very low latency for your requests, so it uses a cache that is _eventually consistent_ with 
the remote server. Client requests don't wait for the server to respond and you could even have 
inconsistent results for a second or two, but it will be worked out fairly quickly and nobody 
has to wait in the meantime.) 

### Bonus: Force JWT authentication

Update the `install/authentication-policy.yaml` file to set your correct URLs:

    origins:
    - jwt:
        issuer: https://{your organization}-{your environment}.apigee.net/edgemicro-auth/token
        jwks_uri: https://{your organization}-{your environment}.apigee.net/edgemicro-auth/jwkPublicKeys
    - jwt:
        issuer: https://{your organization}-{your environment}.apigee.net/edgemicro-auth/verifyApiKey
        jwks_uri: https://{your organization}-{your environment}.apigee.net/edgemicro-auth/jwkPublicKeys

The hostname (and ports) for these URLs should mirror what you used for `customer_base` config above,
adjust as appropriate if you are using OPDK.

Now, apply your Istio authentication policy:

    istioctl create -f authentication-policy.yaml

Calls to helloworld should now fail (with or without the API Key):

    curl http://$HELLOWORLD_URL/hello
    
Should receive an auth error:

    Origin authentication failed.

Now get a JWT token:

    curl https://{your organization}-{your environment}.apigee.net/edgemicro-auth/verifyApiKey -d '{ "apiKey":"{your consumer key}" }' -H "Content-Type: application/json"

or

    edgemicro token get -o {your organization} -e {your environment} -i {your key} -s {your secret}

Try again with your token:

    curl http://$HELLOWORLD_URL/hello -H "Authorization: Bearer {your jwt token}"

This call should now be successful.

Congratulations! You're good to go.

---

To join the Apigee pre-release program for additional documentation and support, please contact:
<anchor-prega-support@google.com>.