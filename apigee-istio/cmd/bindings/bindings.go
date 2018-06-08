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

package bindings

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/apigee/istio-mixer-adapter/adapter/product"
	"github.com/apigee/istio-mixer-adapter/apigee-istio/shared"
	"github.com/spf13/cobra"
)

const (
	servicesAttr         = "istio-services"
	productsURLFormat    = "%s/products"                                         // customerProxyURL
	productAttrURLFormat = "%s/v1/o/theganyo1-eval/apiproducts/%s/attributes/%s" // ManagementBase, prod, attr
)

type bindings struct {
	*shared.RootArgs
	products []product.APIProduct
}

// Cmd returns base command
func Cmd(rootArgs *shared.RootArgs, printf, fatalf shared.FormatFn) *cobra.Command {
	cfg := &bindings{RootArgs: rootArgs}

	c := &cobra.Command{
		Use:   "bindings",
		Short: "Manage Apigee Product to Istio Service bindings",
		Long:  "Manage Apigee Product to Istio Service bindings.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return rootArgs.Resolve(false)
		},
	}

	c.AddCommand(cmdBindingsList(cfg, printf, fatalf))
	c.AddCommand(cmdBindingsAdd(cfg, printf, fatalf))
	c.AddCommand(cmdBindingsRemove(cfg, printf, fatalf))

	return c
}

func cmdBindingsList(b *bindings, printf, fatalf shared.FormatFn) *cobra.Command {
	c := &cobra.Command{
		Use:   "list",
		Short: "List Apigee Product to Istio Service bindings",
		Long:  "List Apigee Product to Istio Service bindings",
		Args:  cobra.NoArgs,

		Run: func(cmd *cobra.Command, _ []string) {
			b.cmdList(printf, fatalf)
		},
	}

	return c
}

func cmdBindingsAdd(b *bindings, printf, fatalf shared.FormatFn) *cobra.Command {
	c := &cobra.Command{
		Use:   "add [service name] [product name]",
		Short: "Add Istio Service binding to Apigee Product",
		Long:  "Add Istio Service binding to Apigee Product",
		Args:  cobra.ExactArgs(2),

		Run: func(cmd *cobra.Command, args []string) {
			serviceName := args[0]
			productName := args[1]
			p, err := b.getProduct(productName)
			if err != nil {
				fatalf("%v", err)
			}
			if p == nil {
				fatalf("invalid product name: %s", args[0])
			}

			b.bindService(p, serviceName, printf, fatalf)
		},
	}

	return c
}

func cmdBindingsRemove(b *bindings, printf, fatalf shared.FormatFn) *cobra.Command {
	c := &cobra.Command{
		Use:   "remove [service name] [product name]",
		Short: "Remove Istio Service binding from Apigee Product",
		Long:  "Remove Istio Service binding from Apigee Product",
		Args:  cobra.ExactArgs(2),

		Run: func(cmd *cobra.Command, args []string) {
			serviceName := args[0]
			productName := args[1]
			p, err := b.getProduct(productName)
			if err != nil {
				fatalf("%v", err)
			}
			if p == nil {
				fatalf("invalid product name: %s", args[0])
			}

			b.unbindService(p, serviceName, printf, fatalf)
		},
	}

	return c
}

func (b *bindings) getProduct(name string) (*product.APIProduct, error) {
	products, err := b.getProducts()
	if err != nil {
		return nil, err
	}
	for _, p := range products {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, nil
}

func (b *bindings) getProducts() ([]product.APIProduct, error) {
	if b.products != nil {
		return b.products, nil
	}
	productsURL := fmt.Sprintf(productsURLFormat, b.CustomerProxyURL)
	req, err := http.NewRequest(http.MethodGet, productsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	var res product.APIResponse
	resp, err := b.Client.Do(req, &res)
	if err != nil {
		return nil, fmt.Errorf("error retrieving products: %v", err)
	}
	defer resp.Body.Close()

	return res.APIProducts, nil
}

func (b *bindings) cmdList(printf, fatalf shared.FormatFn) error {
	products, err := b.getProducts()
	if err != nil {
		fatalf("%v", err)
	}
	var bound, unbound []product.APIProduct
	for _, p := range products {
		// server returns empty scopes as array with a single empty string, remove for consistency
		if len(p.Scopes) == 1 && p.Scopes[0] == "" {
			p.Scopes = []string{}
		}
		// server may return empty quota field as "null"
		if p.QuotaLimit == "null" {
			p.QuotaLimit = ""
		}
		p.Targets = p.GetBoundServices()
		if p.Targets == nil {
			unbound = append(bound, p)
		} else {
			bound = append(bound, p)
		}
	}

	sort.Sort(byName(bound))
	sort.Sort(byName(unbound))
	data := struct {
		Bound   []product.APIProduct
		Unbound []product.APIProduct
	}{
		Bound:   bound,
		Unbound: unbound,
	}
	tmp := template.New("products")
	tmp.Funcs(template.FuncMap{
		"scopes": func(in []string) string { return strings.Join(in, ",") },
	})
	tmp, err = tmp.Parse(productsTemplate)
	if err != nil {
		fatalf("failed to create template: %v", err)
	}
	err = tmp.Execute(os.Stdout, data)
	if err != nil {
		fatalf("failed to execute template: %v", err)
	}

	return nil
}

func (b *bindings) bindService(p *product.APIProduct, service string, printf, fatalf shared.FormatFn) {
	boundServices := p.GetBoundServices()
	if _, ok := indexOf(boundServices, service); ok {
		fatalf("service %s is already bound to %s", service, p.Name)
	}
	err := b.updateServiceBindings(p, append(boundServices, service))
	if err != nil {
		fatalf("error removing service %s from %s: %v", service, p.Name, err)
	}
	printf("product %s is now bound to: %s", p.Name, service)
}

func (b *bindings) unbindService(p *product.APIProduct, service string, printf, fatalf shared.FormatFn) {
	boundServices := p.GetBoundServices()
	i, ok := indexOf(boundServices, service)
	if !ok {
		fatalf("service %s is not bound to %s", service, p.Name)
	}
	boundServices = append(boundServices[:i], boundServices[i+1:]...)
	err := b.updateServiceBindings(p, boundServices)
	if err != nil {
		fatalf("error removing service %s from %s: %v", service, p.Name, err)
	}
	printf("product %s is no longer bound to: %s", p.Name, service)
}

func (b *bindings) updateServiceBindings(p *product.APIProduct, bindings []string) error {
	newAttr := struct {
		Value string `json:"value"`
	}{
		Value: strings.Join(bindings, ","),
	}
	req, err := b.Client.NewRequest(http.MethodPost, "", newAttr)
	if err != nil {
		return err
	}
	path := fmt.Sprintf(productAttrURLFormat, b.ManagementBase, p.Name, servicesAttr)
	req.URL.Path = path // hack: negate client's incorrect method of determining base URL
	var attr product.Attribute
	_, err = b.Client.Do(req, &attr)
	return err
}

func indexOf(array []string, val string) (index int, exists bool) {
	index = -1
	for i, v := range array {
		if val == v {
			index = i
			exists = true
			break
		}
	}
	return
}

type byName []product.APIProduct

func (a byName) Len() int           { return len(a) }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool { return a[i].Name < a[j].Name }

const productsTemplate = `
{{- define "product"}}
{{.Name}}:
 {{- if .Scopes}}
  Scopes: {{scopes (.Scopes)}}
 {{- end}}
 {{- if .QuotaLimit}}
  Quota: {{.QuotaLimit}} requests every {{.QuotaInterval}} {{.QuotaTimeUnit}} 
 {{- end}}
 {{- if .Targets}}
  Service bindings:
  {{- range .Targets}}
    {{.}}
  {{- end}}
  Paths:
  {{- range .Resources}}
    {{.}}
  {{- end}}
 {{- end}}
{{- end}}
API Products
============          
{{- if .Bound}}
Bound
-----
 {{- range .Bound}}
 {{- template "product" .}}
 {{- end}}
{{- end}}
{{- if .Unbound}}

Unbound
-------
 {{- range .Unbound}}
 {{- template "product" .}}
 {{- end}}
{{- end}}
`
