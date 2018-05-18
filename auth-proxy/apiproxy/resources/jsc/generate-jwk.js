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

 const alg = "RS256";
 const use = "sig";
 var certificate1 = context.getVariable("private.certificate1");
 var certificate2 = context.getVariable("private.certificate2");
 var certificatelist = {};

 certificatelist.keys = [];
 
 if (!certificate1) {
    throw Error("No certificate found");
 }
 
 var key1 = KEYUTIL.getKey(certificate1);
 var jwk1 = KEYUTIL.getJWKFromKey(key1);
 var cert1_kid = context.getVariable("private.certificate1_kid") || null;

 if (cert1_kid !== null) {
    jwk1.kid = cert1_kid;
    jwk1.alg = alg;
    jwk1.use = use;
 }
 certificatelist.keys.push(jwk1);
 
 if (certificate2) {
    var key2 = KEYUTIL.getKey(certificate2);
    var jwk2 = KEYUTIL.getJWKFromKey(key2);
    var cert2_kid = context.getVariable("private.certificate2_kid") || null;
    if (cert2_kid !== null) {
        jwk2.kid = cert2_kid;
        jwk2.alg = alg;
        jwk2.use = use;
    }
    certificatelist.keys.push(jwk2);
 }

 context.setVariable("jwkmessage", JSON.stringify(certificatelist));

