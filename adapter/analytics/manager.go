package analytics

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/auth"
	"istio.io/istio/mixer/pkg/adapter"
)

// A Manager wraps all things related to analytics processing
type Manager interface {
	Start(env adapter.Env)
	Close()
	SendRecords(ctx *auth.Context, records []Record) error
}

// NewManager constructs and starts a new manager. Call Close when you are done.
func NewManager(env adapter.Env, opts Options) (Manager, error) {
	if opts.LegacyEndpoint {
		return &legacyAnalytics{}, nil
	}

	m, err := newManager(opts)
	if err != nil {
		return nil, err
	}
	m.Start(env)
	return m, nil
}

func newManager(opts Options) (*manager, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	// Ensure that the buffer path exists and we can access it.
	td := path.Join(opts.BufferPath, tempDir)
	sd := path.Join(opts.BufferPath, stagingDir)
	if err := os.MkdirAll(td, bufferMode); err != nil {
		return nil, fmt.Errorf("mkdir %s: %s", td, err)
	}
	if err := os.MkdirAll(sd, bufferMode); err != nil {
		return nil, fmt.Errorf("mkdir %s: %s", sd, err)
	}

	return &manager{
		close: make(chan bool),
		client: &http.Client{
			Timeout: httpTimeout,
		},
		now:                time.Now,
		collectionInterval: defaultCollectionInterval,
		tempDir:            td,
		stagingDir:         sd,
		bufferSize:         opts.BufferSize,
		buckets:            map[string]bucket{},
		baseURL:            opts.BaseURL,
		key:                opts.Key,
		secret:             opts.Secret,
	}, nil
}
