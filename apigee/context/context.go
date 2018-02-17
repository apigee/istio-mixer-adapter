package context

import (
	"istio.io/istio/mixer/pkg/adapter"
	"net/url"
)

type Context interface {
	Log() adapter.Logger
	Organization() string
	Environment() string
	Key() string
	Secret() string

	ApigeeBase() url.URL
	CustomerBase() url.URL
}
