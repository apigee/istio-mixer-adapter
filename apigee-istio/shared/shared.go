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

package shared

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/apigee/istio-mixer-adapter/apigee-istio/apigee"
)

const (
	DefaultManagementBase = "https://api.enterprise.apigee.com"
	DefaultRouterBase     = "https://{org}-{env}.apigee.net"
	RouterBaseFormat      = "https://%s-%s.apigee.net"

	internalProxyURLFormat = "%s://istioservices.%s/edgemicro" // routerBase scheme, routerBase domain
	customerProxyURLFormat = "%s/istio-auth"                   // routerBase
)

type BuildInfoType struct {
	Version string
	Commit  string
	Date    string
}

var BuildInfo BuildInfoType

type RootArgs struct {
	RouterBase     string // "https://org-env.apigee.net"
	ManagementBase string // "https://api.enterprise.apigee.com"
	Verbose        bool
	Org            string
	Env            string
	Username       string
	Password       string
	NetrcPath      string

	// the following is derived in Resolve()
	InternalProxyURL string
	CustomerProxyURL string
	Client           *apigee.EdgeClient
	ClientOpts       *apigee.EdgeClientOptions
}

func (p *RootArgs) Resolve() error {
	if p.RouterBase == DefaultRouterBase {
		p.RouterBase = fmt.Sprintf(RouterBaseFormat, p.Org, p.Env)
	}
	// calculate internal proxy URL from router URL (reuse the scheme and domain)
	u, err := url.Parse(p.RouterBase)
	if err != nil {
		return err
	}
	domain := u.Host[strings.Index(u.Host, ".")+1:]
	p.InternalProxyURL = fmt.Sprintf(internalProxyURLFormat, u.Scheme, domain)
	p.CustomerProxyURL = fmt.Sprintf(customerProxyURLFormat, p.RouterBase)

	p.ClientOpts = &apigee.EdgeClientOptions{
		MgmtUrl: p.ManagementBase,
		Org:     p.Org,
		Env:     p.Env,
		Auth: &apigee.EdgeAuth{
			NetrcPath: p.NetrcPath,
			Username:  p.Username,
			Password:  p.Password,
		},
		Debug: p.Verbose,
	}
	p.Client, err = apigee.NewEdgeClient(p.ClientOpts)
	if err != nil {
		if strings.Contains(err.Error(), ".netrc") { // no .netrc and no auth
			baseURL, err := url.Parse(p.ManagementBase)
			if err != nil {
				return fmt.Errorf("unable to parse managementBase url %s: %v", p.ManagementBase, err)
			}
			return fmt.Errorf("no auth: must have username and password or a ~/.netrc entry for %s", baseURL.Host)
		}
		return fmt.Errorf("error initializing Edge client: %v", err)
	}

	return nil
}

// FormatFn formats the supplied arguments according to the format string
// provided and executes some set of operations with the result.
type FormatFn func(format string, args ...interface{})

// Fatalf is a FormatFn that prints the formatted string to os.Stderr and then
// calls os.Exit().
func Fatalf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...) // #nosec
	os.Exit(-1)
}

// Printf is a FormatFn that prints the formatted string to os.Stdout.
func Printf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// NoPrintf is a FormatFn that does nothing
func NoPrintf(format string, args ...interface{}) {
}
