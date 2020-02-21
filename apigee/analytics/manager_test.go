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
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestLegacySelect(t *testing.T) {

	opts := Options{
		LegacyEndpoint:   true,
		BufferPath:       "",
		StagingFileLimit: 10,
		BaseURL:          &url.URL{},
		Key:              "key",
		Secret:           "secret",
		Client:           http.DefaultClient,
		now:              time.Now,
	}

	m, err := NewManager(opts)
	m.Close()
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}

	if _, ok := m.(*legacyAnalytics); !ok {
		t.Errorf("want an *legacyAnalytics type, got: %#v", m)
	}
}

func TestStandardSelect(t *testing.T) {

	opts := Options{
		BufferPath:         "/tmp/apigee-ax/buffer/",
		StagingFileLimit:   10,
		BaseURL:            &url.URL{},
		Key:                "key",
		Secret:             "secret",
		Client:             http.DefaultClient,
		now:                time.Now,
		CollectionInterval: time.Minute,
	}

	m, err := NewManager(opts)
	if err != nil {
		t.Fatalf("newManager: %s", err)
	}
	m.Close()

	if _, ok := m.(*manager); !ok {
		t.Errorf("want an *manager type, got: %#v", m)
	}
}

func TestStandardBadOptions(t *testing.T) {

	opts := Options{
		BufferPath:       "/tmp/apigee-ax/buffer/",
		StagingFileLimit: 0,
		BaseURL:          &url.URL{},
		Key:              "",
		Secret:           "",
		Client:           http.DefaultClient,
		now:              time.Now,
	}

	want := "all analytics options are required"
	m, err := NewManager(opts)
	if err == nil || err.Error() != want {
		t.Errorf("want: %s, got: %s", want, err)
	}
	if m != nil {
		t.Errorf("should not get manager")
		m.Close()
	}
}
