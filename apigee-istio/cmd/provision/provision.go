// Copyright 2017 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provision

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	rnd "math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee-istio/apigee"
	"github.com/apigee/istio-mixer-adapter/apigee-istio/shared"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// todo: switch edgemicro names to istio
const (
	kvmName                 = "istio"
	customerProxyBundleName = "istio-auth.zip"
	customerProxyName       = "istio-auth"

	credentialURLFormat = "%s/credential/organization/%s/environment/%s" // internalProxyURL, org, env

	// todo: switch to formal release location
	defaultProxy          = "https://github.com/srinandan/istio-auth/raw/master/istio-auth-default.zip"
	secureProxy           = "https://github.com/srinandan/istio-auth/raw/master/istio-auth-secure.zip"
	defaultAndSecureProxy = "https://github.com/srinandan/istio-auth/raw/master/istio-auth.zip"
	//customerProxySource := "https://github.com/srinandan/microgateway-edgeauth/raw/master/edgemicro-auth.zip"

	jwkPublicKeysURLFormat = "%s/jwkPublicKeys"                            // customerProxyURL
	productsURLFormat      = "%s/products"                                 // customerProxyURL
	verifyAPIKeyURLFormat  = "%s/verifyApiKey"                             // customerProxyURL
	analyticsURLFormat     = "%s/analytics/organization/%s/environment/%s" // internalProxyURL, org, env
	quotasURLFormat        = "%s/quotas/organization/%s/environment/%s"    // internalProxyURL, org, env
)

type provision struct {
	*shared.RootArgs
	certExpirationInYears int
	certKeyStrength       int
	forceProxyInstall     bool
	virtualHosts          string
	credentialURL         string
	customerProxySource   string
	verifyOnly            bool
}

func Cmd(rootArgs *shared.RootArgs, printf, fatalf shared.FormatFn) *cobra.Command {
	cfg := &provision{RootArgs: rootArgs}

	c := &cobra.Command{
		Use:   "provision",
		Short: "Provision your Apigee environment for Istio",
		Long: `The provision command will set up your Apigee environment for Istio. This includes creating 
and installing a KVM with certificates, creating credentials, and deploying a necessary proxy 
to your organization and environment.`,
		Args: cobra.NoArgs,

		Run: func(cmd *cobra.Command, _ []string) {
			cfg.run(printf, fatalf)
		},
	}

	c.Flags().IntVarP(&cfg.certExpirationInYears, "years", "y", 1,
		"number of years before the cert expires")
	c.Flags().IntVarP(&cfg.certKeyStrength, "strength", "s", 2048,
		"key strength")
	c.Flags().BoolVarP(&cfg.forceProxyInstall, "forceProxyInstall", "f", false,
		"force new proxy install")
	c.Flags().StringVarP(&cfg.virtualHosts, "virtualHosts", "", "default,secure",
		"override proxy virtualHosts")
	c.Flags().BoolVarP(&cfg.verifyOnly, "verifyOnly", "x", false,
		"verify only, don’t provision anything")

	return c
}

func (p *provision) run(printf, fatalf shared.FormatFn) {

	p.credentialURL = fmt.Sprintf(credentialURLFormat, p.InternalProxyURL, p.Org, p.Env)

	// select proxy source
	def := strings.Contains(p.virtualHosts, "default")
	sec := strings.Contains(p.virtualHosts, "secure")
	switch {
	case def && sec:
		p.customerProxySource = defaultAndSecureProxy
	case def:
		p.customerProxySource = defaultProxy
	case sec:
		p.customerProxySource = secureProxy
	default:
		fatalf("invalid virtualHosts: %s, must be: default|secure|default,secure", p.virtualHosts)
	}

	var verbosef = shared.NoPrintf
	if p.Verbose {
		verbosef = printf
	}

	var err error
	var cred *credential
	if !p.verifyOnly {
		if err := p.getOrCreateKVM(verbosef); err != nil {
			fatalf("error creating KVM: %v", err)
		}

		cred, err = p.createCredential(verbosef)
		if err != nil {
			fatalf("error generating credential: %v", err)
		}

		if err := p.importAndDeployProxy(verbosef); err != nil {
			fatalf("error deploying proxy: %v", err)
		}
	}

	// use generated credentials
	opts := *p.ClientOpts
	if cred != nil {
		opts.Auth = &apigee.EdgeAuth{
			Username: cred.Key,
			Password: cred.Secret,
		}
		p.Client, err = apigee.NewEdgeClient(&opts)
		if err != nil {
			fatalf("can't create new client: %v", err)
		}
	}

	printf("verifying internal proxy...")
	p.verifyInternalProxy(opts.Auth, printf)

	printf("verifying customer proxy...")
	p.verifyCustomerProxy(opts.Auth, printf)

	if !p.verifyOnly {
		if err := p.printApigeeHandler(cred, printf); err != nil {
			fatalf("error printing handler: %v", err)
		}
	}
}

// hash for key and secret
func newHash() string {
	// use crypto seed
	var seed int64
	binary.Read(rand.Reader, binary.BigEndian, &seed)
	rnd.Seed(seed)

	//rnd.Seed(time.Now().UnixNano())
	t := time.Now()
	h := sha256.New()
	h.Write([]byte(t.String() + string(rnd.Int())))
	str := hex.EncodeToString(h.Sum(nil))
	return str
}

// converts an RSA public key to PKCS#1, ASN.1 DER form
func marshalPKCS1PublicKey(key *rsa.PublicKey) []byte {
	derBytes, _ := asn1.Marshal(pkcs1PublicKey{
		N: key.N,
		E: key.E,
	})
	return derBytes
}

// generate a self signed key and certificate
func (p *provision) genKeyCert(printf shared.FormatFn) (string, string, string, error) {
	printf("generating a new key and cert...")
	privateKey, err := rsa.GenerateKey(rand.Reader, p.certKeyStrength)
	if err != nil {
		log.Fatalf("failed to generate private key: %s", err)
		return "", "", "", err
	}
	now := time.Now()
	template := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0),
		Subject: pkix.Name{
			CommonName:   kvmName,
			Organization: []string{kvmName},
		},
		NotBefore:    now.Add(-5 * time.Minute).UTC(),
		NotAfter:     now.AddDate(p.certExpirationInYears, 0, 0).UTC(),
		IsCA:         true,
		SubjectKeyId: []byte{1, 2, 3, 4}, // todo: need to find out how to generate this
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}
	derBytes, err := x509.CreateCertificate(
		rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		log.Fatalf("failed to create CA certificate: %s", err)
		return "", "", "", err
	}

	certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	printf("certificate:\n%s", string(certBytes))

	pkBytes := marshalPKCS1PublicKey(&privateKey.PublicKey)
	publicKeyBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkBytes})
	printf("public key:\n%s", string(publicKeyBytes))

	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	printf("private key:\n%s", string(keyBytes))

	return string(certBytes), string(publicKeyBytes), string(keyBytes), nil
}

//check if the KVM exists, if it doesn't, create a new one
func (p *provision) getOrCreateKVM(printf shared.FormatFn) error {
	printf("checking if KVM %s exists...", kvmName)
	_, resp, err := p.Client.KVMService.Get(kvmName)
	if err != nil && resp == nil {
		return err
	}
	switch resp.StatusCode {
	case 200:
		printf("KVM %s exists", kvmName)
	case 404:
		printf("KVM %s does not exist, creating a new one...", kvmName)
		certBytes, publicKeyBytes, keyBytes, err := p.genKeyCert(printf)
		if err != nil {
			return err
		}
		kvm := apigee.KVM{
			Name:      kvmName,
			Encrypted: false,
			Entries: []apigee.Entry{
				{
					Name:  "private_key",
					Value: keyBytes,
				},
				{
					Name:  "public_key",
					Value: certBytes,
				},
				{
					Name:  "public_key1",
					Value: publicKeyBytes,
				},
				{
					Name:  "public_key1_kid",
					Value: "1",
				},
				{
					Name:  "private_key_kid",
					Value: "1",
				},
			},
		}
		resp, err = p.Client.KVMService.Create(kvm)
		if err != nil {
			return err
		}
		if resp.StatusCode != 201 {
			return fmt.Errorf("error creating KVM %s, status code: %v", kvmName, resp.StatusCode)
		}
		printf("KVM %s created", kvmName)
	default:
		return fmt.Errorf("error checking for KVM %s, status code: %v", kvmName, resp.StatusCode)
	}
	return nil
}

func (p *provision) createCredential(printf shared.FormatFn) (*credential, error) {
	printf("creating credential...")
	cred := &credential{
		Key:    newHash(),
		Secret: newHash(),
	}

	req, err := p.Client.NewRequest(http.MethodPost, p.credentialURL, cred)
	if err != nil {
		return nil, err
	}
	req.URL, err = url.Parse(p.credentialURL) // override client's munged URL
	if err != nil {
		return nil, err
	}

	resp, err := p.Client.Do(req, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("failed to create credential, status: %d", resp.StatusCode)
	}
	printf("credential created")
	return cred, nil
}

func (p *provision) printApigeeHandler(cred *credential, printf shared.FormatFn) error {
	handler := apigeeHandler{
		ApiVersion: "config.istio.io/v1alpha2",
		Kind:       "apigee",
		Metadata: metadata{
			Name:      "apigee-handler",
			Namespace: "istio-system",
		},
		Spec: specification{
			ApigeeBase:   p.InternalProxyURL,
			OrgName:      p.Org,
			EnvName:      p.Env,
			Key:          cred.Key,
			Secret:       cred.Secret,
			CustomerBase: p.CustomerProxyURL,
		},
	}
	formattedBytes, err := yaml.Marshal(handler)
	if err != nil {
		return err
	}
	printf("\n# istio handler configuration for apigee adapter\n%s", string(formattedBytes))
	return nil
}

// returns directory for downloaded file
func (p *provision) downloadProxy(printf shared.FormatFn) (string, error) {
	printf("downloading most recent proxy bundle...")
	dir, err := ioutil.TempDir("", "apigee")
	out, err := os.Create(filepath.Join(dir, customerProxyBundleName))
	if err != nil {
		return "", err
	}
	defer out.Close()

	resp, err := http.Get(p.customerProxySource)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}
	printf("proxy bundle downloaded to: %s", out.Name())

	return dir, nil
}

func (p *provision) importAndDeployProxy(printf shared.FormatFn) error {
	if !p.forceProxyInstall {
		printf("checking if proxy %s deployment exists...", customerProxyName)
		deployments, _, err := p.Client.Proxies.GetDeployments(customerProxyName)
		if err != nil {
			return err
		}
		var deployment *apigee.EnvironmentDeployment
		for _, ed := range deployments.Environments {
			if ed.Name == p.Env {
				deployment = &ed
				break
			}
		}
		if deployment != nil {
			rev := deployment.Revision[0].Number
			printf("proxy %s revision %s already deployed to %s", customerProxyName, rev, p.Env)
			return nil
		}
	}

	dir, err := p.downloadProxy(printf)
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	printf("checking proxy %s status...", customerProxyName)
	proxy, resp, e := p.Client.Proxies.Get(customerProxyName)

	var revision apigee.Revision = 1
	if proxy != nil {
		revision = proxy.Revisions[len(proxy.Revisions)-1] + 1
		printf("proxy %s exists. highest revision is: %d", customerProxyName, revision-1)
	}

	// create a new client to avoid dumping the proxy binary to stdout during Import
	noDebugClient := p.Client
	if p.Verbose {
		opts := *p.ClientOpts
		opts.Debug = false
		noDebugClient, err = apigee.NewEdgeClient(&opts)
		if err != nil {
			return err
		}
	}

	printf("creating new proxy %s revision: %d...", customerProxyName, revision)
	_, resp, e = noDebugClient.Proxies.Import(customerProxyName, filepath.Join(dir, customerProxyBundleName))
	if e != nil {
		return fmt.Errorf("error importing proxy %s: %v", customerProxyName, e)
	}
	defer resp.Body.Close()

	printf("deploying proxy %s revision %d to env %s...", customerProxyName, revision, p.Env)
	_, resp, e = p.Client.Proxies.Deploy(customerProxyName, p.Env, revision)
	if e != nil {
		return fmt.Errorf("error deploying proxy %s: %v", customerProxyName, e)
	}
	defer resp.Body.Close()
	return nil
}

// verify POST internalProxyURL/analytics/organization/%s/environment/%s
// verify POST internalProxyURL/quotas/organization/%s/environment/%s
func (p *provision) verifyInternalProxy(auth *apigee.EdgeAuth, printf shared.FormatFn) {
	analyticsURL := fmt.Sprintf(analyticsURLFormat, p.InternalProxyURL, p.Org, p.Env)
	req, err := http.NewRequest(http.MethodGet, analyticsURL, nil)
	if err != nil {
		req.SetBasicAuth(auth.Username, auth.Password)
		q := req.URL.Query()
		q.Add("tenant", "fake")
		q.Add("relative_file_path", "fake")
		q.Add("file_content_type", "application/x-gzip")
		q.Add("encrypt", "true")
		req.URL.RawQuery = q.Encode()
		var resp *apigee.Response
		resp, err = p.Client.Do(req, nil)
		if err == nil && resp.StatusCode != 200 {
			var body []byte
			if body, err = ioutil.ReadAll(resp.Body); err == nil {
				err = fmt.Errorf("response code: %d, body: %s", resp.StatusCode, string(body))
			}
		}
	}
	printVerify(analyticsURL, err, printf)

	quotasURL := fmt.Sprintf(quotasURLFormat, p.InternalProxyURL, p.Org, p.Env)
	req, err = http.NewRequest(http.MethodPost, quotasURL, strings.NewReader("{}"))
	if err != nil {
		req.SetBasicAuth(auth.Username, auth.Password)
		var resp *apigee.Response
		resp, err = p.Client.Do(req, nil)
		if err == nil && resp.StatusCode != 200 {
			var body []byte
			if body, err = ioutil.ReadAll(resp.Body); err == nil {
				err = fmt.Errorf("response code: %d, body: %s", resp.StatusCode, string(body))
			}
		}
	}
	printVerify(quotasURL, err, printf)
}

// verify GET customerProxyURL/jwkPublicKeys
// verify GET customerProxyURL/products
// verify POST customerProxyURL/verifyApiKey
func (p *provision) verifyCustomerProxy(auth *apigee.EdgeAuth, printf shared.FormatFn) {
	jwkPublicKeysURL := fmt.Sprintf(jwkPublicKeysURLFormat, p.CustomerProxyURL)
	printVerify(jwkPublicKeysURL, p.verifyGET(jwkPublicKeysURL), printf)

	productsURL := fmt.Sprintf(productsURLFormat, p.CustomerProxyURL)
	printVerify(productsURL, p.verifyGET(productsURL), printf)

	verifyAPIKeyURL := fmt.Sprintf(verifyAPIKeyURLFormat, p.CustomerProxyURL)
	body := fmt.Sprintf(`{ "apiKey": "%s" }`, auth.Username)
	req, err := http.NewRequest(http.MethodPost, verifyAPIKeyURL, strings.NewReader(body))
	if err != nil {
		var resp *apigee.Response
		resp, err = p.Client.Do(req, nil)
		if err == nil && resp.StatusCode != 200 {
			var body []byte
			if body, err = ioutil.ReadAll(resp.Body); err == nil {
				err = fmt.Errorf("response code: %d, body: %s", resp.StatusCode, string(body))
			}
		}
	}
	printVerify(verifyAPIKeyURL, err, printf)
}

func printVerify(url string, err error, printf shared.FormatFn) {
	if err != nil {
		printf(" bad: %s\n      %s", url, err)
	} else {
		printf("  ok: %s", url)
	}
}

func (p *provision) verifyGET(targetURL string) error {
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return err
	}
	resp, err := p.Client.Do(req, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("response code: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

type apigeeHandler struct {
	ApiVersion string        `json:"apiVersion,omitempty"`
	Kind       string        `json:"kind,omitempty"`
	Metadata   metadata      `json:"metadata,omitempty"`
	Spec       specification `json:"spec,omitempty"`
}

type metadata struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type specification struct {
	ApigeeBase   string `json:"apigee_base,omitempty"`
	CustomerBase string `json:"customer_base,omitempty"`
	OrgName      string `json:"org_name,omitempty"`
	EnvName      string `json:"env_name,omitempty"`
	Key          string `json:"key,omitempty"`
	Secret       string `json:"secret,omitempty"`
}

// ASN.1 structure of a PKCS#1 public key
type pkcs1PublicKey struct {
	N *big.Int
	E int
}

type credential struct {
	Key    string `json:"key"`
	Secret string `json:"secret"`
}
