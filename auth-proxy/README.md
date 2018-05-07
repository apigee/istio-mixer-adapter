# istio-auth

An Apigee Edge proxy to support generating, refreshing and revoking access tokens for istio-mixer-adapter.

The istio-auth proxy acts as an auth server and provides four functions:

* Provides a list of all products in the org (/products)
* Provides a signed JWT if the API Key is valid (/verifyApiKey)
* Generates an access token, which is a signed JWT. Supports client_credentials grant type (/token)
* Refresh an access token (/refresh)
* Revoke a refresh token (/revoke)

### Installation

This proxy will automatically be installed during provisioning with the apigee-istio CLI.

### Customizations

#### How do I set custom expiry?

In the flow named 'Obtain Access Token' you'll find an Assign Message Policy called 'Create OAuth Request'. 
Change the value here:

    <AssignVariable>
        <Name>token_expiry</Name>
        <Value>300000</Value>
    </AssignVariable>


#### How can I get refresh tokens?

The OAuth v2 policy supports password grant. Send a request as below:

    POST /token
    {
      "client_id":"foo",
      "client_secret":"foo",
      "grant_type":"password",
      "username":"blah",
      "password": "blah"
    }

If valid, the response will contain a refresh token.

#### How do I refresh an access_token?

Send a request as below:

    POST /refresh
    {
        "grant_type": "refresh_token",
        "refresh_token": "foo",
        "client_id":"blah",
            "client_secret":"blah"
    }

If valid, the response will contain a new access_token.

#### What grant types are supported?

* client_credentials
* password
* refresh_token

Users may extend the Apigee OAuth v2 policy to add support for additional grant types.

#### Support for JSON Web Keys

istio-mixer-adapter stores private keys and public keys in an encrypted kvm on Apigee Edge. 
The proxy exposes an endpoint '/jwkPublicKeys' to return public keys as JWK.

#### Support for JWT "kid" - Key Identifiers. 

If the KVM includes a field called 'private_key_kid' (value can be any string), the JWT header will include the "kid".

    {
      "alg": "RS256",
      "typ": "JWT",
      "kid": "1"
    }

The "kid" will be leveraged during validation of JWTs.
