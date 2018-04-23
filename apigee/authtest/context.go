package authtest

import (
	"fmt"
	"net/url"

	"istio.io/istio/mixer/pkg/adapter"
)

// Context implements the context.Context interface and is to be used in tests.
type Context struct {
	apigeeBase   *url.URL
	customerBase *url.URL
	orgName      string
	envName      string
	key          string
	secret       string
	log          adapter.Logger
}

// NewContext constructs a new test context.
func NewContext(base string, log adapter.Logger) *Context {
	u, err := url.Parse(base)
	if err != nil {
		panic(fmt.Sprintf("Could not parse URL: %s", base))
	}
	return &Context{
		apigeeBase:   u,
		customerBase: u,
		log:          log,
	}
}

// Log gets a logger for the test context.
func (c *Context) Log() adapter.Logger { return c.log }

// ApigeeBase gets a URL base to send HTTP requests to.
func (c *Context) ApigeeBase() *url.URL { return c.apigeeBase }

// CustomerBase gets a URL base to send HTTP requests to.
func (c *Context) CustomerBase() *url.URL { return c.customerBase }

// Organization gets this context's organization.
func (c *Context) Organization() string { return c.orgName }

// Environment gets this context's environment.
func (c *Context) Environment() string { return c.envName }

// Key gets this context's API key.
func (c *Context) Key() string { return c.key }

// Secret gets this context's API secret.
func (c *Context) Secret() string { return c.secret }

// SetOrganization sets this context's organization.
func (c *Context) SetOrganization(o string) { c.orgName = o }

// SetEnvironment sets this context's environment.
func (c *Context) SetEnvironment(e string) { c.envName = e }
