package authtest

import (
	"fmt"
	"net/url"

	"istio.io/istio/mixer/pkg/adapter"
)

type Context struct {
	apigeeBase   url.URL
	customerBase url.URL
	orgName      string
	envName      string
	key          string
	secret       string
	log          adapter.Logger
}

func NewContext(base string, log adapter.Logger) *Context {
	u, err := url.Parse(base)
	if err != nil {
		panic(fmt.Sprintf("Could not parse URL: %s", base))
	}
	return &Context{
		apigeeBase:   *u,
		customerBase: *u,
		log:          log,
	}
}

func (c *Context) Log() adapter.Logger {
	return c.log
}
func (c *Context) ApigeeBase() url.URL {
	return c.apigeeBase
}
func (c *Context) CustomerBase() url.URL {
	return c.customerBase
}
func (c *Context) SetOrganization(o string) {
	c.orgName = o
}
func (c *Context) Organization() string {
	return c.orgName
}
func (c *Context) SetEnvironment(e string) {
	c.envName = e
}
func (c *Context) Environment() string {
	return c.envName
}
func (c *Context) Key() string {
	return c.key
}
func (c *Context) Secret() string {
	return c.secret
}
