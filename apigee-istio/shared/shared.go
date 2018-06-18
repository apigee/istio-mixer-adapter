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
	// DefaultManagementBase is the default management API URL for Apigee
	DefaultManagementBase = "https://api.enterprise.apigee.com"
	// DefaultRouterBase is the default (fake format) for base of the organization router URL
	DefaultRouterBase = "https://{org}-{env}.apigee.net"
	// RouterBaseFormat is the real format for base of the organization router URL
	RouterBaseFormat = "https://%s-%s.apigee.net"

	internalProxyURLFormat     = "%s://istioservices.%s/edgemicro" // routerBase scheme, routerBase domain
	internalProxyURLFormatOPDK = "%s/edgemicro"                    // routerBase
	customerProxyURLFormat     = "%s/istio-auth"                   // routerBase
)

// BuildInfoType holds version information
type BuildInfoType struct {
	Version string
	Commit  string
	Date    string
}

// BuildInfo is populated by main init()
var BuildInfo BuildInfoType

// RootArgs is the base struct to hold all command arguments
type RootArgs struct {
	RouterBase     string // "https://org-env.apigee.net"
	ManagementBase string // "https://api.enterprise.apigee.com"
	Verbose        bool
	Org            string
	Env            string
	Username       string
	Password       string
	NetrcPath      string
	IsOPDK         bool

	// the following is derived in Resolve()
	InternalProxyURL string
	CustomerProxyURL string
	Client           *apigee.EdgeClient
	ClientOpts       *apigee.EdgeClientOptions
}

// Resolve is used to populate shared args, it's automatically called prior when creating the root command
func (r *RootArgs) Resolve(skipAuth bool) error {
	if r.RouterBase == DefaultRouterBase {
		r.RouterBase = fmt.Sprintf(RouterBaseFormat, r.Org, r.Env)
	}
	r.IsOPDK = !strings.Contains(r.ManagementBase, "api.enterprise.apigee.com")

	// calculate internal proxy URL from router URL (reuse the scheme and domain)
	if r.IsOPDK {
		r.InternalProxyURL = fmt.Sprintf(internalProxyURLFormatOPDK, r.RouterBase)
	} else {
		u, err := url.Parse(r.RouterBase)
		if err != nil {
			return err
		}
		domain := u.Host[strings.Index(u.Host, ".")+1:]
		r.InternalProxyURL = fmt.Sprintf(internalProxyURLFormat, u.Scheme, domain)
	}
	r.CustomerProxyURL = fmt.Sprintf(customerProxyURLFormat, r.RouterBase)

	r.ClientOpts = &apigee.EdgeClientOptions{
		MgmtUrl: r.ManagementBase,
		Org:     r.Org,
		Env:     r.Env,
		Auth: &apigee.EdgeAuth{
			NetrcPath: r.NetrcPath,
			Username:  r.Username,
			Password:  r.Password,
			SkipAuth:  skipAuth,
		},
		Debug: r.Verbose,
	}
	var err error
	r.Client, err = apigee.NewEdgeClient(r.ClientOpts)
	if err != nil {
		if strings.Contains(err.Error(), ".netrc") { // no .netrc and no auth
			baseURL, err := url.Parse(r.ManagementBase)
			if err != nil {
				return fmt.Errorf("unable to parse managementBase url %s: %v", r.ManagementBase, err)
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
	Errorf(format, args)
	os.Exit(-1)
}

// Printf is a FormatFn that prints the formatted string to os.Stdout.
func Printf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// Errorf is a FormatFn that prints the formatted string to os.Stderr.
func Errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// NoPrintf is a FormatFn that does nothing
func NoPrintf(format string, args ...interface{}) {
}
