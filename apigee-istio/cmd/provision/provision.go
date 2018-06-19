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
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	rnd "math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"path/filepath"

	"io"

	"archive/zip"

	"encoding/xml"

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
	internalProxyName = "edgemicro-internal"

	credentialURLFormat = "%s/credential/organization/%s/environment/%s" // internalProxyURL, org, env

	authProxyZip     = "istio-auth.zip"
	internalProxyZip = "istio-internal.zip"

	certsURLFormat          = "%s/certs"                                      // customerProxyURL
	productsURLFormat       = "%s/products"                                   // customerProxyURL
	verifyAPIKeyURLFormat   = "%s/verifyApiKey"                               // customerProxyURL
	analyticsURLFormat      = "%s/analytics/organization/%s/environment/%s"   // internalProxyURL, org, env
	quotasURLFormat         = "%s/quotas/organization/%s/environment/%s"      // internalProxyURL, org, env
	legacyAnalyticURLFormat = "%s/axpublisher/organization/%s/environment/%s" // internalProxyURL, org, env

	virtualHostReplaceText    = "<VirtualHost>default</VirtualHost>"
	virtualHostReplacementFmt = "<VirtualHost>%s</VirtualHost>" // each virtualHost
)

type provision struct {
	*shared.RootArgs
	certExpirationInYears int
	certKeyStrength       int
	forceProxyInstall     bool
	virtualHosts          string
	credentialURL         string
	verifyOnly            bool
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
			return rootArgs.Resolve(false)
		},

		Run: func(cmd *cobra.Command, _ []string) {
			p.run(printf, fatalf)
		},
	}

	c.Flags().IntVarP(&p.certExpirationInYears, "years", "", 1,
		"number of years before the cert expires")
	c.Flags().IntVarP(&p.certKeyStrength, "strength", "", 2048,
		"key strength")
	c.Flags().BoolVarP(&p.forceProxyInstall, "forceProxyInstall", "f", false,
		"force new proxy install")
	c.Flags().StringVarP(&p.virtualHosts, "virtualHosts", "", "default,secure",
		"override proxy virtualHosts")
	c.Flags().BoolVarP(&p.verifyOnly, "verifyOnly", "", false,
		"verify only, don’t provision anything")

	return c
}

func (p *provision) run(printf, fatalf shared.FormatFn) {

	var cred *credential
	p.credentialURL = fmt.Sprintf(credentialURLFormat, p.InternalProxyURL, p.Org, p.Env)

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

		replaceVirtualHosts := func(proxyDir string) error {
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

		if p.IsOPDK {

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
				setMgmtUrl := false
				for i, cp := range callout.Properties {
					if cp.Name == "REGION_MAP" {
						callout.Properties[i].Value = fmt.Sprintf("DN=%s", p.RouterBase)
					}
					if cp.Name == "MGMT_URL_PREFIX" {
						setMgmtUrl = true
						callout.Properties[i].Value = p.ManagementBase
					}
				}
				if !setMgmtUrl {
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
				fatalf(err.Error())
			}

			if err := p.checkAndDeployProxy(internalProxyName, customizedZip, verbosef); err != nil {
				fatalf("error deploying internal proxy: %v", err)
			}
		}

		if err := p.getOrCreateKVM(verbosef); err != nil {
			fatalf("error retrieving or creating kvm: %v", err)
		}

		cred, err = p.createCredential(verbosef)
		if err != nil {
			fatalf("error generating credential: %v", err)
		}

		customizedProxy, err := getCustomizedProxy(tempDir, authProxyZip, replaceVirtualHosts)
		if err != nil {
			fatalf(err.Error())
		}

		if err := p.checkAndDeployProxy(authProxyName, customizedProxy, verbosef); err != nil {
			fatalf("error deploying auth proxy: %v", err)
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

	verbosef("verifying internal proxy...")
	verifyErrors := p.verifyInternalProxy(opts.Auth, verbosef, fatalf)

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
}

type proxyModFunc func(name string) error

// returns filename of zipped proxy
func getCustomizedProxy(tempDir, name string, modFunc proxyModFunc) (string, error) {
	if err := proxies.RestoreAsset(tempDir, name); err != nil {
		return "", errors.Wrapf(err, "error restoring asset %s", name)
	}
	zipFile := filepath.Join(tempDir, name)

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

//check if the KVM exists, if it doesn't, create a new one
func (p *provision) getOrCreateKVM(printf shared.FormatFn) error {
	printf("checking if kvm %s exists...", kvmName)
	_, resp, err := p.Client.KVMService.Get(kvmName)
	if err != nil && resp == nil {
		return err
	}
	switch resp.StatusCode {
	case 200:
		printf("kvm %s exists", kvmName)
	case 404:
		printf("kvm %s does not exist, creating a new one...", kvmName)
		printf("generating a new key and cert...")
		cert, privateKey, err := GenKeyCert(p.certKeyStrength, p.certExpirationInYears)
		if err != nil {
			return err
		}
		printf("certificate:\n%s", cert)
		printf("private key:\n%s", privateKey)

		kvm := apigee.KVM{
			Name:      kvmName,
			Encrypted: encryptKVM,
			Entries: []apigee.Entry{
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
			},
		}
		resp, err = p.Client.KVMService.Create(kvm)
		if err != nil {
			return err
		}
		if resp.StatusCode != 201 {
			return fmt.Errorf("error creating kvm %s, status code: %v", kvmName, resp.StatusCode)
		}
		printf("kvm %s created", kvmName)
	default:
		return fmt.Errorf("error checking for kvm %s, status code: %v", kvmName, resp.StatusCode)
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

func (p *provision) printApigeeHandler(cred *credential, printf shared.FormatFn, verifyErrors error) error {
	handler := apigeeHandler{
		APIVersion: "config.istio.io/v1alpha2",
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
	if p.IsOPDK {
		handler.Spec.AnalyticsOptions = analyticsOptions{
			LegacyEndpoint: true,
		}
	}
	formattedBytes, err := yaml.Marshal(handler)
	if err != nil {
		return err
	}
	printf("# istio handler configuration for apigee adapter")
	printf("# generated by apigee-istio provision on %s", time.Now().Format("2006-01-02 15:04:05"))
	if verifyErrors != nil {
		printf("# WARNING: verification of provision failed. May not be valid.")
	}
	printf(string(formattedBytes))
	return nil
}

func (p *provision) checkAndDeployProxy(name, file string, printf shared.FormatFn) error {
	printf("checking if proxy %s deployment exists...", name)
	oldRev, err := p.Client.Proxies.GetDeployedRevision(name, p.Env)
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

	if oldRev != nil {
		printf("undeploying proxy %s revision %d on env %s...",
			name, oldRev, p.Env)
		_, resp, err = p.Client.Proxies.Undeploy(name, p.Env, *oldRev)
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
	req.SetBasicAuth(auth.Username, auth.Password)
	resp, err := p.Client.Do(req, nil)
	defer resp.Body.Close()
	if err != nil && resp == nil {
		fatalf("%s", err)
	}
	if err != nil {
		verifyErrors = multierr.Append(verifyErrors, err)
	}

	quotasURL := fmt.Sprintf(quotasURLFormat, p.InternalProxyURL, p.Org, p.Env)
	req, err = http.NewRequest(http.MethodPost, quotasURL, strings.NewReader("{}"))
	if err != nil {
		fatalf("unable to create request", err)
	}
	req.SetBasicAuth(auth.Username, auth.Password)
	resp, err = p.Client.Do(req, nil)
	defer resp.Body.Close()
	if err != nil && resp == nil {
		fatalf("%s", err)
	}
	if err != nil {
		verifyErrors = multierr.Append(verifyErrors, err)
	}
	return verifyErrors
}

// verify GET customerProxyURL/certs
// verify GET customerProxyURL/products
// verify POST customerProxyURL/verifyApiKey
func (p *provision) verifyCustomerProxy(auth *apigee.EdgeAuth, printf, fatalf shared.FormatFn) error {

	verifyGET := func(targetURL string) error {
		req, err := http.NewRequest(http.MethodGet, targetURL, nil)
		if err != nil {
			fatalf("unable to create request", err)
		}
		resp, err := p.Client.Do(req, nil)
		defer resp.Body.Close()
		if err != nil && resp == nil {
			fatalf("%s", err)
		}
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
	resp, err := p.Client.Do(req, nil)
	if err != nil && resp == nil {
		fatalf("%s", err)
	}
	if resp.StatusCode != 401 { // 401 is ok, we don't actually have a valid api key to test
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
	ApigeeBase       string           `yaml:"apigee_base"`
	CustomerBase     string           `yaml:"customer_base"`
	OrgName          string           `yaml:"org_name"`
	EnvName          string           `yaml:"env_name"`
	Key              string           `yaml:"key"`
	Secret           string           `yaml:"secret"`
	AnalyticsOptions analyticsOptions `yaml:"analytics,omitempty"`
}

type analyticsOptions struct {
	LegacyEndpoint bool `yaml:"legacy_endpoint"`
}

type credential struct {
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

type JavaCallout struct {
	Name                                string `xml:"name,attr"`
	DisplayName, ClassName, ResourceURL string
	Properties                          []javaCalloutProperty `xml:"Properties>Property"`
}

type javaCalloutProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}
