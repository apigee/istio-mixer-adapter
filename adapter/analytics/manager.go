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

package analytics

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
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
		return &legacyAnalytics{client: opts.Client}, nil
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

	// Ensure that base temp dir exists
	bufferMode := os.FileMode(0700)
	td := filepath.Join(opts.BufferPath, "temp")
	if err := os.MkdirAll(td, bufferMode); err != nil {
		return nil, fmt.Errorf("mkdir %s: %s", td, err)
	}
	// Ensure that base stage dir exists
	sd := filepath.Join(opts.BufferPath, "staging")
	if err := os.MkdirAll(sd, bufferMode); err != nil {
		return nil, fmt.Errorf("mkdir %s: %s", sd, err)
	}

	return &manager{
		closeStaging:       make(chan bool),
		client:             opts.Client,
		now:                time.Now,
		collectionInterval: defaultCollectionInterval,
		tempDir:            td,
		stagingDir:         sd,
		stagingFileLimit:   opts.StagingFileLimit,
		buckets:            map[string]*bucket{},
		baseURL:            opts.BaseURL,
		key:                opts.Key,
		secret:             opts.Secret,
		sendChannelSize:    opts.SendChannelSize,
	}, nil
}

// A manager is a way for Istio to interact with Apigee's analytics platform.
type manager struct {
	env                adapter.Env
	closeStaging       chan bool
	client             *http.Client
	now                func() time.Time
	log                adapter.Logger
	collectionInterval time.Duration
	tempDir            string // open gzip files being written to
	stagingDir         string // gzip files staged for upload
	stagingFileLimit   int
	bucketsLock        sync.RWMutex
	buckets            map[string]*bucket // dir ("org~env") -> bucket
	baseURL            url.URL
	key                string
	secret             string
	sendChannelSize    int
	stageLock          sync.Mutex
	closed             bool
	uploadChan         chan<- interface{}
	uploadersWait      sync.WaitGroup
}

// Options allows us to specify options for how this analytics manager will run.
type Options struct {
	// LegacyEndpoint is true if using older direct-submit protocol
	LegacyEndpoint bool
	// BufferPath is the directory where the adapter will buffer analytics records.
	BufferPath string
	// StagingFileLimit is the maximum number of files stored in the staging directory.
	// Once this is reached, the oldest files will start being removed.
	StagingFileLimit int
	// Base Apigee URL
	BaseURL url.URL
	// Key for submit auth
	Key string
	// Secret for submit auth
	Secret string
	// Client is a configured HTTPClient
	Client *http.Client
	// SendChannelSize is the size of the records channel
	SendChannelSize int
}

func (o *Options) validate() error {
	if o.BufferPath == "" ||
		o.StagingFileLimit <= 0 ||
		o.Key == "" ||
		o.Client == nil ||
		o.Secret == "" {
		return fmt.Errorf("all analytics options are required")
	}
	return nil
}
