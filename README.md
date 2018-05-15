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

## Prerequisite: Apigee

You must have an [Apigee Edge](https://login.apigee.com) account. If you need one, you can create one [here](https://login.apigee.com/sign_up).

## Download the Release

Istio Mixer Adapter releases can be found [here](https://github.com/apigee/istio-mixer-adapter/releases).

Download the appropriate release for your operating system and extract it. You should a file list similar to:

    LICENSE
    README.md
    install/api-spec.yaml
    install/apigee-definitions.yaml
    install/apigee-handler.yaml
    install/apigee-rule.yaml
    install/authentication-policy.yaml
    apigee-istio
 
`apigee-istio` (or apigee-istio.exe on Windows) is the Command Line Interface (CLI) for this project. You may put it on your PATH for quick access - or just remember to specify the path for the commands below.

The yaml files in the install/ directory contain the configuration for the adapter. We'll discuss these in a bit.

## Provision Apigee for Istio

The first thing you'll need to do is provision Apigee. The `apigee-istio` command will do the work for you:

    apigee-istio -u {your username} -p {your password} -o {your organization name} -e {your environment name} provision

You should see a response like this:

    verifying internal proxy...
      ok: https://edgemicroservices.apigee.net/edgemicro/analytics/organization/myorg/environment/myenv
      ok: https://edgemicroservices.apigee.net/edgemicro/quotas/organization/myorg/environment/myenv
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
      apigee_base: https://edgemicroservices.apigee.net/edgemicro
      customer_base: https://myorg-myenv.apigee.net/istio-auth
      org_name: myorg
      env_name: myenv
      key: 06a40b65005d03ea24c0d53de69ab795590b0c332526e97fed549471bdea00b9
      secret: 93550179f344150c6474956994e0943b3e93a3c90c64035f378dc05c98389633

That last block is important: It's the configuration for your handler. We'll use it in the next section.

Notes:

* For Apigee Private Cloud (OPDK), you'll need to also specify your `--managementBase` in the command.  
* `apigee-istio` will automatically pick up the username and password from a [.netrc](https://ec.haxx.se/usingcurl-netrc.html) file in your home directory if you have an entry for `machine api.enterprise.apigee.com` (or whatever you set as managementBase).

### Set your configuration 

Edit `install/apigee-handler.yaml` and replace the configuration with what you got during provisioning above. You may simply replace the contents of the entire file with that configuration block.

## Install Istio with Apigee mixer

The quickest way to get started with Istio is to follow the [Istio Kubernetes Quick Start](https://istio.io/docs/setup/kubernetes/quick-start.html). 

BUT! Before you install Istio into Kubernetes (step 5: Install Istio’s core components), you'll need to edit Istio's install file to point to the Apigee mixer including the adapter instead of the generic Istio mixer.

In the Istio directory, just edit `install/kubernetes/istio.yaml` or `install/kubernetes/istio-auth.yaml` to change the mixer image Istio will use:

Find:
    
    gcr.io/istio-release/mixer:0.8.0-pre20180421-09-15
    
and replace with:

    gcr.io/apigee-api-management-istio/istio-mixer:nightly
    
Important: There will two instances: One for policy, one for telemetry. Replace both.

Now you may finish installing and verifying Istio per the Quick Start instructions.  

## Install your service

For the following, we'll assume you've installed Istio's [Hello World](https://github.com/istio/istio/tree/master/samples/helloworld).

You should be able to access this service successfully:

    curl http://${HELLOWORLD_URL}/hello

## Configure Apigee Mixer in Istio

Now, apply the Apigee configuration to Istio:

        kubectl apply -f apigee-definitions.yaml
        kubectl apply -f apigee-handler.yaml
        kubectl apply -f apigee-rule.yaml

At this point, you should no longer be able to access your helloworld service. If you curl it:

    curl http://${HELLOWORLD_URL}/hello
    
You should receive a permission denied error:

    PERMISSION_DENIED:apigee-handler.apigee.istio-system:missing authentication
    
If only you had some credentials! Let's fix that...

### Configure an Apigee API Product to represent your service

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
    
    curl http://${HELLOWORLD_URL}/hello -H "x-api-key: {your consumer key}"

This call should now be successful.

### Quota: Check your quota (unreleased: nightly build only)

Remember that Quota you set for 5 requests per minute? Let's max it out.

Make that same request a few more times:

    curl http://${HELLOWORLD_URL}/hello -H "x-api-key: {your consumer key}"

Or, if you're in a Unix shell, you can use repeat:

    repeat 10 curl http://${HELLOWORLD_URL}/hello -H "x-api-key: {your consumer key}"
    
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
        issuer: https://{your organization}-{your environment}.apigee.net/istio-auth/token
        jwks_uri: https://{your organization}-{your environment}.apigee.net/istio-auth/certs

The hostname (and ports) for these URLs should mirror what you used for `customer_base` config above,
adjust as appropriate if you are using OPDK.

Now, apply your Istio authentication policy:

    istioctl create -f authentication-policy.yaml

Calls to helloworld should now fail (with or without the API Key):

    curl http://${HELLOWORLD_URL}/hello
    
And should receive an auth error:

    Origin authentication failed.

Now get a JWT Bearer token:

    apigee-istio token get -o {your organization} -e {your environment} -i {your key} -s {your secret}
    
or

    curl https://{your organization}-{your environment}.apigee.net/istio-auth/token -d '{ "client_id":"{your key}", "client_secret":"your secret", "grant_type":"client_credentials" }' -H "Content-Type: application/json"

Try again with your token:

    curl http://${HELLOWORLD_URL}/hello -H "Authorization: Bearer {your jwt token}"

This call should now be successful.

### Check your analytics

Head back to the [Apigee Edge UI](https://apigee.com/edge).

Click the `Analyze` in the menu on the left and check out those analytics.

---

To join the Apigee pre-release program for additional documentation and support, please contact:
<anchor-prega-support@google.com>.