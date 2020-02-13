// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

var apiCredential = JSON.parse(context.getVariable('apiCredential'));
var apiKey = context.getVariable('apikey');
//{"Credentials":{"Credential":[{"Attributes":{},"ConsumerKey":"xxx","ConsumerSecret":"xx","ExpiresAt":"-1","IssuedAt":"1530046158362","ApiProducts":{"ApiProduct":{"Name":"details product","Status":"approved"}},"Scopes":{},"Status":"approved"}]}}
var credentials = apiCredential.Credentials.Credential;

var apiProductsList = [];
try {
    credentials.forEach(function(credential) {
        if (credential.ConsumerKey == apiKey) {
            credential.ApiProducts.ApiProduct.forEach(function(apiProduct){
                apiProductsList.push(apiProduct.Name);
            });
        }
    });
} catch (err) {
    print(err);
}

var scope = context.getVariable("oauthv2accesstoken.AccessTokenRequest.scope");
if (scope) {
    var scopearr = scope.split(" ");
    context.setVariable("scope", scopearr.join());
} else {
    context.setVariable("scope", "");
}

context.setVariable("apiProductList", apiProductsList.join());
context.setVariable("nbf", new Date().toUTCString());
context.setVariable("iss", context.getVariable("proxyProto") + "://" + context.getVariable("proxyHost") + context.getVariable("proxy.basepath") + context.getVariable("proxy.pathsuffix"));
context.setVariable("jti", 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    var r = Math.random() * 16 | 0,
        v = c == 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
}));
