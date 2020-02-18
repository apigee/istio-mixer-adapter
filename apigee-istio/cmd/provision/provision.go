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
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	rnd "math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee-istio/apigee"
	"github.com/apigee/istio-mixer-adapter/apigee-istio/proxies"
	"github.com/apigee/istio-mixer-adapter/apigee-istio/shared"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/multierr"
	"gopkg.in/yaml.v2"
)

const (
	kvmName           = "istio"
	encryptKVM        = true
	authProxyName     = "istio-auth"
	mgmtProxyName     = "istio-mgmt"
	internalProxyName = "edgemicro-internal"

	legacyCredentialURLFormat = "%s/credential/organization/%s/environment/%s" // InternalProxyURL, org, env

	legacyAuthProxyZip = "istio-auth-legacy.zip"
	hybridAuthProxyZip = "istio-auth-hybrid.zip"
	mgmtProxyZip       = "istio-mgmt.zip"
	internalProxyZip   = "istio-internal.zip"

	apiProductsPath        = "apiproducts"
	developersPath         = "developers"
	applicationsPathFormat = "developers/%s/apps"                // developer email
	keyCreatePathFormat    = "developers/%s/apps/%s/keys/create" // developer email, app ID
	keyPathFormat          = "developers/%s/apps/%s/keys/%s"     // developer email, app ID, key ID

	certsURLFormat        = "%s/certs"        // CustomerProxyURL
	productsURLFormat     = "%s/products"     // CustomerProxyURL
	verifyAPIKeyURLFormat = "%s/verifyApiKey" // CustomerProxyURL
	quotasURLFormat       = "%s/quotas"       // CustomerProxyURL
	rotateURLFormat       = "%s/rotate"       // CustomerProxyURL

	analyticsURLFormat      = "%s/analytics/organization/%s/environment/%s"   // InternalProxyURL, org, env
	legacyAnalyticURLFormat = "%s/axpublisher/organization/%s/environment/%s" // InternalProxyURL, org, env

	// virtualHost is only necessary for legacy
	virtualHostReplaceText    = "<VirtualHost>default</VirtualHost>"
	virtualHostReplacementFmt = "<VirtualHost>%s</VirtualHost>" // each virtualHost
)

type provision struct {
	*shared.RootArgs
	certExpirationInYears int
	certKeyStrength       int
	forceProxyInstall     bool
	virtualHosts          string
	verifyOnly            bool
	provisionKey          string
	provisionSecret       string
	developerEmail        string
}

// Cmd returns base command
func Cmd(rootArgs *shared.RootArgs, printf, fatalf shared.FormatFn) *cobra.Command {
	p := &provision{RootArgs: rootArgs}

	c := &cobra.Command{
		Use:   "provision",
		Short: "Provision your Apigee environment for Istio",
		Long: `The provision command will set up your Apigee environment for Istio. This includes creating 
and installing a kvm with certificates, creating credentials, and deploying a necessary proxy 
to your organization and environment.`,
		Args: cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := rootArgs.Resolve(false)
			if err == nil {
				if !p.verifyOnly && p.IsHybrid && p.developerEmail == "" {
					fatalf("hybrid provisioning requires an email address for --developer-email")
				}
			}
			return err
		},

		Run: func(cmd *cobra.Command, _ []string) {
			if p.verifyOnly && (p.provisionKey == "" || p.provisionSecret == "") {
				fatalf("--verifyOnly requires values for --key and --secret")
			}
			p.run(printf, fatalf)
		},
	}

	c.Flags().StringVarP(&p.developerEmail, "developer-email", "d", "",
		"email used to create a developer (hybrid only)")
	c.Flags().IntVarP(&p.certExpirationInYears, "years", "", 1,
		"number of years before the jwt cert expires")
	c.Flags().IntVarP(&p.certKeyStrength, "strength", "", 2048,
		"key strength")
	c.Flags().BoolVarP(&p.forceProxyInstall, "forceProxyInstall", "f", false,
		"force new proxy install (upgrades proxy)")
	c.Flags().StringVarP(&p.virtualHosts, "virtualHosts", "", "default,secure",
		"override proxy virtualHosts")
	c.Flags().BoolVarP(&p.verifyOnly, "verifyOnly", "", false,
		"verify only, don’t provision anything")

	c.Flags().StringVarP(&p.provisionKey, "key", "k", "", "gateway key (for --verify-only)")
	c.Flags().StringVarP(&p.provisionSecret, "secret", "s", "", "gateway secret (for --verify-only)")

	return c
}

func (p *provision) run(printf, fatalf shared.FormatFn) {

	var cred *credential

	var verbosef = shared.NoPrintf
	if p.Verbose || p.verifyOnly {
		verbosef = printf
	}

	if !p.verifyOnly {

		tempDir, err := ioutil.TempDir("", "apigee")
		if err != nil {
			fatalf("error creating temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		replaceVH := func(proxyDir string) error {
			proxiesFile := filepath.Join(proxyDir, "proxies", "default.xml")
			bytes, err := ioutil.ReadFile(proxiesFile)
			if err != nil {
				return errors.Wrapf(err, "error reading file %s", proxiesFile)
			}
			newVH := ""
			for _, vh := range strings.Split(p.virtualHosts, ",") {
				if strings.TrimSpace(vh) != "" {
					newVH = newVH + fmt.Sprintf(virtualHostReplacementFmt, vh)
				}
			}
			bytes = []byte(strings.Replace(string(bytes), virtualHostReplaceText, newVH, 1))
			if err := ioutil.WriteFile(proxiesFile, bytes, 0); err != nil {
				return errors.Wrapf(err, "error writing %s", proxiesFile)
			}
			return nil
		}

		replaceVHAndAuthTarget := func(proxyDir string) error {
			if err := replaceVH(proxyDir); err != nil {
				return err
			}

			if p.IsOPDK {
				policiesFile := filepath.Join(proxyDir, "policies", "Authenticate-Call.xml")
				bytes, err := ioutil.ReadFile(policiesFile)
				if err != nil {
					return errors.Wrapf(err, "error reading file %s", policiesFile)
				}
				oldTarget := "https://edgemicroservices.apigee.net"
				bytes = []byte(strings.Replace(string(bytes), oldTarget, p.RouterBase, 1))
				if err := ioutil.WriteFile(policiesFile, bytes, 0); err != nil {
					return errors.Wrapf(err, "error writing %s", policiesFile)
				}
			}
			return nil
		}

		if p.IsOPDK {
			if err := p.deployInternalProxy(replaceVH, tempDir, verbosef); err != nil {
				fatalf("error deploying internal proxy: %v", err)
			}
		}

		// input istio-auth proxy
		var customizedProxy string
		if p.IsHybrid {
			customizedProxy, err = getCustomizedProxy(tempDir, hybridAuthProxyZip, nil)
		} else {
			customizedProxy, err = getCustomizedProxy(tempDir, legacyAuthProxyZip, replaceVHAndAuthTarget)
		}
		if err != nil {
			fatalf(err.Error())
		}

		if err := p.checkAndDeployProxy(authProxyName, customizedProxy, verbosef); err != nil {
			fatalf("error deploying %s proxy: %v", authProxyName, err)
		}

		if p.IsHybrid {
			cred, err = p.createHybridCredential(verbosef)
		} else {
			cred, err = p.createLegacyCredential(verbosef)
		}
		if err != nil {
			fatalf("error generating credential: %v", err)
		}

		if err := p.getOrCreateKVM(cred, verbosef); err != nil {
			fatalf("error retrieving or creating kvm: %v", err)
		}

	} else { // verifyOnly == true
		cred = &credential{
			Key:    p.provisionKey,
			Secret: p.provisionSecret,
		}
	}

	// use generated credentials
	opts := *p.ClientOpts
	if cred != nil {
		opts.Auth = &apigee.EdgeAuth{
			Username: cred.Key,
			Password: cred.Secret,
		}
		var err error
		if p.Client, err = apigee.NewEdgeClient(&opts); err != nil {
			fatalf("can't create new client: %v", err)
		}
	}

	var verifyErrors error
	if !p.IsHybrid {
		verbosef("verifying internal proxy...")
		verifyErrors = p.verifyInternalProxy(opts.Auth, verbosef, fatalf)
	}

	verbosef("verifying customer proxy...")
	verifyErrors = multierr.Combine(verifyErrors, p.verifyCustomerProxy(opts.Auth, verbosef, fatalf))

	if verifyErrors != nil {
		shared.Errorf("\nWARNING: Apigee may not be provisioned properly.")
		shared.Errorf("Unable to verify proxy endpoint(s). Errors:\n")
		for _, err := range multierr.Errors(verifyErrors) {
			shared.Errorf("  %s", err)
		}
		shared.Errorf("\n")
	}

	if !p.verifyOnly {
		if err := p.printApigeeHandler(cred, printf, verifyErrors); err != nil {
			fatalf("error generating handler: %v", err)
		}
	}

	if verifyErrors != nil {
		os.Exit(1)
	}

	verbosef("provisioning verified OK")
}

// ensures that there's a product, developer, and app
func (p *provision) createHybridCredential(verbosef shared.FormatFn) (*credential, error) {
	const istioAuthName = "istio-auth"

	// create product
	product := apiProduct{
		Name:         istioAuthName,
		DisplayName:  istioAuthName,
		ApprovalType: "auto",
		Attributes: []attribute{
			{Name: "access", Value: "internal"},
		},
		Description:  istioAuthName + " access",
		APIResources: []string{"/**"},
		Environments: []string{p.Env},
		Proxies:      []string{istioAuthName},
	}
	req, err := p.Client.NewRequestNoEnv(http.MethodPost, apiProductsPath, product)
	if err != nil {
		return nil, err
	}
	res, err := p.Client.Do(req, nil)
	if err != nil {
		if res.StatusCode != http.StatusConflict { // exists
			return nil, err
		}
		verbosef("product %s already exists", istioAuthName)
	}

	// create developer
	devEmail := p.developerEmail
	dev := developer{
		Email:     devEmail,
		FirstName: istioAuthName,
		LastName:  istioAuthName,
		UserName:  istioAuthName,
	}
	req, err = p.Client.NewRequestNoEnv(http.MethodPost, developersPath, dev)
	if err != nil {
		return nil, err
	}
	res, err = p.Client.Do(req, nil)
	if err != nil {
		if res.StatusCode != http.StatusConflict { // exists
			return nil, err
		}
		verbosef("developer %s already exists", devEmail)
	}

	// create application
	app := application{
		Name:        istioAuthName,
		APIProducts: []string{istioAuthName},
	}
	applicationsPath := fmt.Sprintf(applicationsPathFormat, devEmail)
	req, err = p.Client.NewRequestNoEnv(http.MethodPost, applicationsPath, &app)
	if err != nil {
		return nil, err
	}
	res, err = p.Client.Do(req, &app)
	if err == nil {
		appCred := app.Credentials[0]
		cred := &credential{
			Key:    appCred.Key,
			Secret: appCred.Secret,
		}
		verbosef("credentials created: %v", cred)
		return cred, nil
	}

	if res == nil || res.StatusCode != http.StatusConflict {
		return nil, err
	}

	// http.StatusConflict == app exists, create a new credential
	verbosef("app %s already exists", istioAuthName)
	appCred := appCredential{
		Key:    newHash(),
		Secret: newHash(),
	}
	createKeyPath := fmt.Sprintf(keyCreatePathFormat, devEmail, istioAuthName)
	if req, err = p.Client.NewRequestNoEnv(http.MethodPost, createKeyPath, &appCred); err != nil {
		return nil, err
	}
	if res, err = p.Client.Do(req, &appCred); err != nil {
		return nil, err
	}

	// adding product to the credential requires a separate call
	appCredDetails := appCredentialDetails{
		APIProducts: []string{istioAuthName},
	}
	keyPath := fmt.Sprintf(keyPathFormat, devEmail, istioAuthName, appCred.Key)
	if req, err = p.Client.NewRequestNoEnv(http.MethodPost, keyPath, &appCredDetails); err != nil {
		return nil, err
	}
	if res, err = p.Client.Do(req, &appCred); err != nil {
		return nil, err
	}

	cred := &credential{
		Key:    appCred.Key,
		Secret: appCred.Secret,
	}
	verbosef("credentials created: %v", cred)

	return cred, nil
}

func (p *provision) deployInternalProxy(replaceVirtualHosts func(proxyDir string) error, tempDir string, verbosef shared.FormatFn) error {

	customizedZip, err := getCustomizedProxy(tempDir, internalProxyZip, func(proxyDir string) error {

		// change server locations
		calloutFile := filepath.Join(proxyDir, "policies", "Callout.xml")
		bytes, err := ioutil.ReadFile(calloutFile)
		if err != nil {
			return errors.Wrapf(err, "error reading file %s", calloutFile)
		}
		var callout JavaCallout
		if err := xml.Unmarshal(bytes, &callout); err != nil {
			return errors.Wrapf(err, "error unmarshalling %s", calloutFile)
		}
		setMgmtURL := false
		for i, cp := range callout.Properties {
			if cp.Name == "REGION_MAP" {
				callout.Properties[i].Value = fmt.Sprintf("DN=%s", p.RouterBase)
			}
			if cp.Name == "MGMT_URL_PREFIX" {
				setMgmtURL = true
				callout.Properties[i].Value = p.ManagementBase
			}
		}
		if !setMgmtURL {
			callout.Properties = append(callout.Properties,
				javaCalloutProperty{
					Name:  "MGMT_URL_PREFIX",
					Value: p.ManagementBase,
				})
		}

		writer, err := os.OpenFile(calloutFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0)
		if err != nil {
			return errors.Wrapf(err, "error writing %s", calloutFile)
		}
		writer.WriteString(xml.Header)
		encoder := xml.NewEncoder(writer)
		encoder.Indent("", "  ")
		err = encoder.Encode(callout)
		if err != nil {
			return errors.Wrapf(err, "error encoding xml to %s", calloutFile)
		}
		err = writer.Close()
		if err != nil {
			return errors.Wrapf(err, "error closing file %s", calloutFile)
		}

		return replaceVirtualHosts(proxyDir)
	})
	if err != nil {
		return err
	}

	return p.checkAndDeployProxy(internalProxyName, customizedZip, verbosef)
}

type proxyModFunc func(name string) error

// returns filename of zipped proxy
func getCustomizedProxy(tempDir, name string, modFunc proxyModFunc) (string, error) {
	if err := proxies.RestoreAsset(tempDir, name); err != nil {
		return "", errors.Wrapf(err, "error restoring asset %s", name)
	}
	zipFile := filepath.Join(tempDir, name)
	if modFunc == nil {
		return zipFile, nil
	}

	extractDir, err := ioutil.TempDir(tempDir, "proxy")
	if err != nil {
		return "", errors.Wrap(err, "error creating temp dir")
	}
	if err := unzipFile(zipFile, extractDir); err != nil {
		return "", errors.Wrapf(err, "error extracting %s to %s", zipFile, extractDir)
	}

	if err := modFunc(filepath.Join(extractDir, "apiproxy")); err != nil {
		return "", err
	}

	// write zip
	customizedZip := filepath.Join(tempDir, "customized.zip")
	if err := zipDir(extractDir, customizedZip); err != nil {
		return "", errors.Wrapf(err, "error zipping %s to %s", extractDir, customizedZip)
	}

	return customizedZip, nil
}

// hash for key and secret
func newHash() string {
	// use crypto seed
	var seed int64
	binary.Read(rand.Reader, binary.BigEndian, &seed)
	rnd.Seed(seed)

	t := time.Now()
	h := sha256.New()
	h.Write([]byte(t.String() + string(rnd.Int())))
	str := hex.EncodeToString(h.Sum(nil))
	return str
}

// GenKeyCert generates a self signed key and certificate
// returns certBytes, privateKeyBytes, error
func GenKeyCert(keyStrength, certExpirationInYears int) (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, keyStrength)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to generate private key")
	}
	now := time.Now()
	subKeyIDHash := sha256.New()
	_, err = subKeyIDHash.Write(privateKey.N.Bytes())
	if err != nil {
		return "", "", errors.Wrap(err, "failed to generate key id")
	}
	subKeyID := subKeyIDHash.Sum(nil)
	template := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0),
		Subject: pkix.Name{
			CommonName:   kvmName,
			Organization: []string{kvmName},
		},
		NotBefore:    now.Add(-5 * time.Minute).UTC(),
		NotAfter:     now.AddDate(certExpirationInYears, 0, 0).UTC(),
		IsCA:         true,
		SubjectKeyId: subKeyID,
		KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature |
			x509.KeyUsageDataEncipherment,
	}
	derBytes, err := x509.CreateCertificate(
		rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to create CA certificate")
	}

	certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	return string(certBytes), string(keyBytes), nil
}

//check if the KVM exists, if it doesn't, create a new one and sets certs for JWT
func (p *provision) getOrCreateKVM(cred *credential, printf shared.FormatFn) error {

	cert, privateKey, err := GenKeyCert(p.certKeyStrength, p.certExpirationInYears)
	if err != nil {
		return err
	}

	kvm := apigee.KVM{
		Name:      kvmName,
		Encrypted: encryptKVM,
	}

	if !p.IsHybrid { // hybrid API breaks with any initial entries
		kvm.Entries = []apigee.Entry{
			{
				Name:  "private_key",
				Value: privateKey,
			},
			{
				Name:  "certificate1",
				Value: cert,
			},
			{
				Name:  "certificate1_kid",
				Value: "1",
			},
		}
	}

	resp, err := p.Client.KVMService.Create(kvm)
	if err != nil && (resp == nil || resp.StatusCode != http.StatusConflict) { // http.StatusConflict == already exists
		return err
	}
	if resp.StatusCode == http.StatusConflict {
		printf("kvm %s already exists", kvmName)
		return nil
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("error creating kvm %s, status code: %v", kvmName, resp.StatusCode)
	}
	printf("kvm %s created", kvmName)

	if p.IsHybrid { // hybrid requires an additional call to set the certificate

		rotateReq := rotateRequest{
			PrivateKey:  privateKey,
			Certificate: cert,
			KeyID:       "1",
		}

		body := new(bytes.Buffer)
		if err = json.NewEncoder(body).Encode(rotateReq); err != nil {
			return err
		}
		rotateURL := fmt.Sprintf(rotateURLFormat, p.CustomerProxyURL)
		req, err := http.NewRequest(http.MethodPost, rotateURL, body)
		if err != nil {
			return err
		}
		req.SetBasicAuth(cred.Key, cred.Secret)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		if resp, err = p.Client.Do(req, nil); err != nil {
			return err
		}
		resp.Body.Close()
	}

	printf("registered a new key and cert for JWTs:\n")
	printf("certificate:\n%s", cert)
	printf("private key:\n%s", privateKey)

	return nil
}

func (p *provision) createLegacyCredential(printf shared.FormatFn) (*credential, error) {
	printf("creating credential...")
	cred := &credential{
		Key:    newHash(),
		Secret: newHash(),
	}

	credentialURL := fmt.Sprintf(legacyCredentialURLFormat, p.InternalProxyURL, p.Org, p.Env)

	req, err := p.Client.NewRequest(http.MethodPost, credentialURL, cred)
	if err != nil {
		return nil, err
	}
	req.URL, err = url.Parse(credentialURL) // override client's munged URL
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

func (p *provision) printApigeeHandler(cred *credential, printf shared.FormatFn, verifyErrors error) error {
	handler := apigeeHandler{
		APIVersion: "config.istio.io/v1alpha2",
		Kind:       "handler",
		Metadata: metadata{
			Name:      "apigee-handler",
			Namespace: "istio-system",
		},
		Spec: specification{
			Adapter: "apigee",
			Connection: connection{
				Address: "apigee-adapter:5000",
			},
			Params: params{
				ApigeeBase:   p.InternalProxyURL,
				CustomerBase: p.CustomerProxyURL,
				OrgName:      p.Org,
				EnvName:      p.Env,
				Key:          cred.Key,
				Secret:       cred.Secret,
			},
		},
	}
	if p.IsOPDK {
		handler.Spec.Params.AnalyticsOptions = analyticsOptions{
			LegacyEndpoint: true,
		}
	}
	if p.IsHybrid {
		handler.Spec.Params.HybridConfig = "/opt/apigee/customer/default.properties"
		handler.Spec.Params.AnalyticsOptions = analyticsOptions{
			CollectionInterval: "10s",
		}
	}
	formattedBytes, err := yaml.Marshal(handler)
	if err != nil {
		return err
	}
	printf("# Istio handler configuration for Apigee gRPC adapter for Mixer")
	printf("# generated by apigee-istio provision on %s", time.Now().Format("2006-01-02 15:04:05"))
	if verifyErrors != nil {
		printf("# WARNING: verification of provision failed. May not be valid.")
	}
	printf(string(formattedBytes))
	return nil
}

func (p *provision) checkAndDeployProxy(name, file string, printf shared.FormatFn) error {
	printf("checking if proxy %s deployment exists...", name)
	var oldRev *apigee.Revision
	var err error
	if p.IsHybrid {
		oldRev, err = p.Client.Proxies.GetHybridDeployedRevision(name)
	} else {
		oldRev, err = p.Client.Proxies.GetDeployedRevision(name)
	}
	if err != nil {
		return err
	}
	if oldRev != nil {
		if p.forceProxyInstall {
			printf("replacing proxy %s revision %s in %s", name, oldRev, p.Env)
		} else {
			printf("proxy %s revision %s already deployed to %s", name, oldRev, p.Env)
			return nil
		}
	}

	printf("checking proxy %s status...", name)
	var resp *apigee.Response
	proxy, resp, err := p.Client.Proxies.Get(name)
	if err != nil && (resp == nil || resp.StatusCode != 404) {
		return err
	}

	return p.importAndDeployProxy(name, proxy, oldRev, file, printf)
}

func (p *provision) importAndDeployProxy(name string, proxy *apigee.Proxy, oldRev *apigee.Revision, file string, printf shared.FormatFn) error {
	var newRev apigee.Revision = 1
	if proxy != nil && len(proxy.Revisions) > 0 {
		sort.Sort(apigee.RevisionSlice(proxy.Revisions))
		newRev = proxy.Revisions[len(proxy.Revisions)-1] + 1
		printf("proxy %s exists. highest revision is: %d", name, newRev-1)
	}

	// create a new client to avoid dumping the proxy binary to stdout during Import
	noDebugClient := p.Client
	if p.Verbose {
		opts := *p.ClientOpts
		opts.Debug = false
		var err error
		noDebugClient, err = apigee.NewEdgeClient(&opts)
		if err != nil {
			return err
		}
	}

	printf("creating new proxy %s revision: %d...", name, newRev)
	_, resp, err := noDebugClient.Proxies.Import(name, file)
	if err != nil {
		return errors.Wrapf(err, "error importing proxy %s", name)
	}
	defer resp.Body.Close()

	if oldRev != nil && !p.IsHybrid { // it's not necessary to undeploy first in hybrid
		printf("undeploying proxy %s revision %d on env %s...",
			name, oldRev, p.Env)
		_, resp, err = p.Client.Proxies.Undeploy(name, p.Env, *oldRev)
		if err != nil {
			return errors.Wrapf(err, "error undeploying proxy %s", name)
		}
	}

	printf("deploying proxy %s revision %d to env %s...", name, newRev, p.Env)
	_, resp, err = p.Client.Proxies.Deploy(name, p.Env, newRev)
	if err != nil {
		return errors.Wrapf(err, "error deploying proxy %s", name)
	}
	defer resp.Body.Close()

	return nil
}

// verify POST internalProxyURL/analytics/organization/%s/environment/%s
// verify POST internalProxyURL/quotas/organization/%s/environment/%s
func (p *provision) verifyInternalProxy(auth *apigee.EdgeAuth, printf, fatalf shared.FormatFn) error {
	var verifyErrors error

	var req *http.Request
	var err error
	if p.IsOPDK {
		analyticsURL := fmt.Sprintf(legacyAnalyticURLFormat, p.InternalProxyURL, p.Org, p.Env)
		req, err = http.NewRequest(http.MethodPost, analyticsURL, strings.NewReader("{}"))
		if err != nil {
			fatalf("unable to create request", err)
		}
	} else {
		analyticsURL := fmt.Sprintf(analyticsURLFormat, p.InternalProxyURL, p.Org, p.Env)
		req, err = http.NewRequest(http.MethodGet, analyticsURL, nil)
		if err != nil {
			fatalf("unable to create request", err)
		}
		q := req.URL.Query()
		q.Add("tenant", fmt.Sprintf("%s~%s", p.Org, p.Env))
		q.Add("relative_file_path", "fake")
		q.Add("file_content_type", "application/x-gzip")
		q.Add("encrypt", "true")
		req.URL.RawQuery = q.Encode()
	}
	auth.ApplyTo(req)
	resp, err := p.Client.Do(req, nil)
	if err != nil && resp == nil {
		fatalf("%s", err)
	}
	defer resp.Body.Close()
	if err != nil {
		verifyErrors = multierr.Append(verifyErrors, err)
	}

	return verifyErrors
}

// verify GET customerProxyURL/certs
// verify GET customerProxyURL/products
// verify POST customerProxyURL/verifyApiKey
// verify POST customerProxyURL/quotas
func (p *provision) verifyCustomerProxy(auth *apigee.EdgeAuth, printf, fatalf shared.FormatFn) error {

	verifyGET := func(targetURL string) error {
		req, err := http.NewRequest(http.MethodGet, targetURL, nil)
		if err != nil {
			fatalf("unable to create request", err)
		}
		auth.ApplyTo(req)
		resp, err := p.Client.Do(req, nil)
		if err != nil && resp == nil {
			fatalf("%s", err)
		}
		defer resp.Body.Close()
		return err
	}

	var verifyErrors error
	certsURL := fmt.Sprintf(certsURLFormat, p.CustomerProxyURL)
	err := verifyGET(certsURL)
	verifyErrors = multierr.Append(verifyErrors, err)

	productsURL := fmt.Sprintf(productsURLFormat, p.CustomerProxyURL)
	err = verifyGET(productsURL)
	verifyErrors = multierr.Append(verifyErrors, err)

	verifyAPIKeyURL := fmt.Sprintf(verifyAPIKeyURLFormat, p.CustomerProxyURL)
	body := fmt.Sprintf(`{ "apiKey": "%s" }`, auth.Username)
	req, err := http.NewRequest(http.MethodPost, verifyAPIKeyURL, strings.NewReader(body))
	if err != nil {
		fatalf("unable to create request", err)
	}
	req.Header.Add("Content-Type", "application/json")
	auth.ApplyTo(req)
	resp, err := p.Client.Do(req, nil)
	if err != nil && resp == nil {
		fatalf("%s", err)
	}
	if resp.StatusCode != 401 { // 401 is ok, we don't actually have a valid api key to test
		verifyErrors = multierr.Append(verifyErrors, err)
	}

	quotasURL := fmt.Sprintf(quotasURLFormat, p.CustomerProxyURL)
	req, err = http.NewRequest(http.MethodPost, quotasURL, strings.NewReader("{}"))
	if err != nil {
		fatalf("unable to create request", err)
	}
	req.Header.Add("Content-Type", "application/json")
	auth.ApplyTo(req)
	resp, err = p.Client.Do(req, nil)
	if err != nil && resp == nil {
		fatalf("%s", err)
	}
	defer resp.Body.Close()
	if err != nil {
		verifyErrors = multierr.Append(verifyErrors, err)
	}

	return verifyErrors
}

func unzipFile(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	os.MkdirAll(dest, 0755)

	extract := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		path := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extract(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func zipDir(source, file string) error {
	zipFile, err := os.Create(file)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)

	var addFiles func(w *zip.Writer, fileBase, zipBase string) error
	addFiles = func(w *zip.Writer, fileBase, zipBase string) error {
		files, err := ioutil.ReadDir(fileBase)
		if err != nil {
			return err
		}

		for _, file := range files {
			fqName := filepath.Join(fileBase, file.Name())
			zipFQName := filepath.Join(zipBase, file.Name())

			if file.IsDir() {
				addFiles(w, fqName, zipFQName)
				continue
			}

			bytes, err := ioutil.ReadFile(fqName)
			if err != nil {
				return err
			}
			f, err := w.Create(zipFQName)
			if err != nil {
				return err
			}
			if _, err = f.Write(bytes); err != nil {
				return err
			}
		}
		return nil
	}

	err = addFiles(w, source, "")
	if err != nil {
		return err
	}

	return w.Close()
}

type apigeeHandler struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   metadata      `yaml:"metadata"`
	Spec       specification `yaml:"spec"`
}

type metadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type specification struct {
	Adapter    string     `yaml:"adapter"`
	Connection connection `yaml:"connection"`
	Params     params     `yaml:"params"`
}

type params struct {
	ApigeeBase       string           `yaml:"apigee_base,omitempty"`
	CustomerBase     string           `yaml:"customer_base"`
	HybridConfig     string           `yaml:"hybrid_config,omitempty"`
	OrgName          string           `yaml:"org_name"`
	EnvName          string           `yaml:"env_name"`
	Key              string           `yaml:"key"`
	Secret           string           `yaml:"secret"`
	AnalyticsOptions analyticsOptions `yaml:"analytics,omitempty"`
}

type analyticsOptions struct {
	LegacyEndpoint     bool   `yaml:"legacy_endpoint,omitempty"`
	CollectionInterval string `yaml:"collection_interval,omitempty"`
}

type credential struct {
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

// JavaCallout must be capitalized to ensure correct generation
type JavaCallout struct {
	Name                                string `xml:"name,attr"`
	DisplayName, ClassName, ResourceURL string
	Properties                          []javaCalloutProperty `xml:"Properties>Property"`
}

type javaCalloutProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

type connection struct {
	Address string `yaml:"address"`
}

type apiProduct struct {
	Name         string      `json:"name,omitempty"`
	DisplayName  string      `json:"displayName,omitempty"`
	ApprovalType string      `json:"approvalType,omitempty"`
	Attributes   []attribute `json:"attributes,omitempty"`
	Description  string      `json:"description,omitempty"`
	APIResources []string    `json:"apiResources,omitempty"`
	Environments []string    `json:"environments,omitempty"`
	Proxies      []string    `json:"proxies,omitempty"`
}

type attribute struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type developer struct {
	Email     string `json:"email,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	UserName  string `json:"userName,omitempty"`
}

type application struct {
	Name        string          `json:"name,omitempty"`
	APIProducts []string        `json:"apiProducts,omitempty"`
	Credentials []appCredential `json:"credentials,omitempty"`
}

type appCredential struct {
	Key    string `json:"consumerKey,omitempty"`
	Secret string `json:"consumerSecret,omitempty"`
}

type rotateRequest struct {
	PrivateKey  string `json:"private_key"`
	Certificate string `json:"certificate"`
	KeyID       string `json:"kid"`
}

type appCredentialDetails struct {
	APIProducts []string    `json:"apiProducts,omitempty"`
	Attributes  []attribute `json:"attributes,omitempty"`
}
