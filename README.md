# Istio Apigee Adapter

This is the source repository for Apigee's Istio Mixer Adapter. This allows users of Istio to
incorporate Apigee Authentication, Authorization, and Analytics policies to protect and
report through the Apigee UI.

# Installation and usage

## Install Istio with Apigee mixer

The quickest way to get started is to follow the [Istio Kubernetes Quick Start](https://istio.io/docs/setup/kubernetes/quick-start.html).

Before you install Istio into Kubernetes (step 5: Install Istio’s core components), edit the `istio.yaml` or `istio-auth.yaml` file to point to the Apigee mixer instead of the Istio vanilla mixer.

Assuming Istio 0.6.0:

Replace: `docker.io/istio/mixer:0.6.0` with: `gcr.io/apigee-api-management-istio/istio-mixer:test`
Replace: `zipkinURL` with `trace_zipkin_url`

(note: after Istio 0.6.0, you shouldn't need the `zipkinURL` replacement anymore)

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

In the [install]() directory, there are several .yaml files for configuring Istio.

### Set your configuration 

Edit `install/apigee-handler.yaml` and replace the configuration values with your own:

      customer_base: https://{your organization}-{your environment}.apigee.net/edgemicro-auth
      org_name: {your organization name}
      env_name: {your environment name}
      key: {your key}
      secret: {your secret}

Note: If you're using Apigee OPDK, you'll need to point `customer_base` and `apigee_base` to your 
local installation.

Now, apply the Apigee configuration to Istio:

        kubectl apply -f apigee-definitions.yaml
        kubectl apply -f apigee-handler.yaml
        kubectl apply -f apigee-rule.yaml
        kubectl apply -f api-spec.yaml

At this point, you should no longer be able to access your helloworld service:

    curl http://$HELLOWORLD_URL/hello
    
Should receive:

    PERMISSION_DENIED:apigee-handler.apigee.istio-system:missing authentication
    
Let's fix that...

### Configure Apigee

Create an [API Product](https://apigee.com/apiproducts) in your Apigee organization:

* Give the API Product definition a name ("helloworld" is fine)
* Make sure the correct environment is checked
* Add a Path with the "+Custom Resource" button. Set it to "/".
* Add an Attribute with the "+Custom Attribute" button. Set the key to "istio-services" and the value to "helloworld.default.svc.cluster.local". 
* Save

Create a [Developer](https://apigee.com/developers)
* Use any values you want

Create an [App](https://apigee.com/apps)
* Give your App a name ("helloworld" is fine)
* Select your developer
* Add your API Product with the "+Product" button.
* Save

Still on the App page, you should now see a "Credentials" section. Click the "Show" button under the "Consumer Key" heading. Copy the key!

### Access helloworld with your key

You should now be able to access the helloworld service in Istio by passing the key you just copied from your Apigee app. Just send it as part of the header:
    
    curl http://$HELLOWORLD_URL/hello -H "x-api-key: {your consumer key}"

This call should now be successful.

### Bonus: Force JWT authentication

Update the Istio authentication policy file to set your correct URLs:

      jwts:
      - issuer: https://{your organization}-{your environment}.apigee.net/edgemicro-auth/verifyApiKey
        jwks_uri: https://{your organization}-{your environment}.apigee.net/edgemicro-auth/jwkPublicKeys
      - issuer: https://{your organization}-{your environment}.apigee.net/edgemicro-auth/token
        jwks_uri: https://{your organization}-{your environment}.apigee.net/edgemicro-auth/jwkPublicKeys
        
The hostname (and ports) for these URLs should mirror what you used for `customer_base` config above,
adjust as appropriate if you are using OPDK.

Now, apply your Istio authentication policy:

    istioctl create -f authentication-policy.yaml

Calls to helloworld should now fail (with or without the API Key):

    curl http://$HELLOWORLD_URL/hello
    
Should receive:

    Required JWT token is missing

Now get a JWT token:

    curl https://{your organization}-{your environment}.apigee.net/edgemicro-auth/verifyApiKey -d '{ "apiKey":"{your consumer key}" }' -H "Content-Type: application/json"

or

    edgemicro token get -o {your organization} -e {your environment} -i {your key} -s {your secret}

Try again with your token:

    curl http://$HELLOWORLD_URL/hello -H "Authorization: Bearer {your jwt token}"

This call should now be successful.
