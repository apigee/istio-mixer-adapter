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

package adapter_test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter"
	"github.com/apigee/istio-mixer-adapter/adapter/analytics"
	"github.com/apigee/istio-mixer-adapter/adapter/config"
	"github.com/apigee/istio-mixer-adapter/adapter/product"
	analyticsT "github.com/apigee/istio-mixer-adapter/template/analytics"
	protobuf "github.com/gogo/protobuf/types"
	istio_policy_v1beta1 "istio.io/api/policy/v1beta1"
	"istio.io/istio/mixer/pkg/status"
	"istio.io/istio/mixer/template/authorization"
)

func TestGRPCAdapter_HandleAnalytics(t *testing.T) {
	basePath := "/some/path"
	queryString := "with=query"
	pathWithQueryString := basePath + "?" + queryString
	ctx := context.Background()
	var baseURL *url.URL

	uploaded := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if strings.HasPrefix(r.URL.Path, "/analytics/") {
			u := *baseURL
			u.Path = "/upload"
			w.Write([]byte(fmt.Sprintf(`{ "url": "%s" }`, u.String())))
			return
		}

		if strings.HasPrefix(r.URL.Path, "/upload") {
			uploaded = true

			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				t.Fatalf("bad gzip: %v", err)
			}
			var recs []analytics.Record
			bio := bufio.NewReader(gz)
			for {
				line, isPrefix, err := bio.ReadLine()
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Fatalf("ReadLine: %v", err)
				}
				if isPrefix {
					t.Fatalf("isPrefix: %v", err)
				}
				r := bytes.NewReader(line)
				var rec analytics.Record
				if err := json.NewDecoder(r).Decode(&rec); err != nil {
					t.Fatalf("bad JSON: %v", err)
				}
				recs = append(recs, rec)
			}

			rec := recs[0]
			if rec.RequestPath != basePath {
				t.Errorf("RequestPath expected %s, got %s", rec.RequestPath, basePath)
			}

			if rec.RequestURI != pathWithQueryString {
				t.Errorf("RequestURI expected %s, got %s", rec.RequestURI, pathWithQueryString)
			}

			w.WriteHeader(200)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/products") {
			var result = product.APIResponse{
				APIProducts: nil,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)
			return
		}

		t.Fatalf("invalid URL called: %s", r.URL.String())
	}))
	defer ts.Close()
	var err error
	baseURL, err = url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %s", err)
	}
	defer os.RemoveAll(d)

	s, err := adapter.NewGRPCAdapter("")
	if err != nil {
		t.Fatalf("unable to start server: %v", err)
	}

	cfg := &config.Params{
		ApigeeBase:   baseURL.String(),
		CustomerBase: baseURL.String(),
		OrgName:      "org",
		EnvName:      "env",
		Key:          "key",
		Secret:       "secret",
		TempDir:      d,
		Analytics: &config.ParamsAnalyticsOptions{
			FileLimit:       10,
			SendChannelSize: 0,
		},
		Products: &config.ParamsProductOptions{},
	}
	configBytes, err := cfg.Marshal()
	if err != nil {
		t.Fatalf("unable to marshal config: %v", err)
	}
	adapterConfig := &protobuf.Any{
		Value: configBytes,
	}

	now := time.Now()
	instanceMsg := &analyticsT.InstanceMsg{
		Name:                         "name",
		ClientReceivedStartTimestamp: timestamp(now),
		ClientReceivedEndTimestamp:   timestamp(now),
		RequestUri:                   pathWithQueryString,
		RequestPath:                  pathWithQueryString,
		ClientIp: &istio_policy_v1beta1.IPAddress{
			Value: []byte(""),
		},
	}

	r := &analyticsT.HandleAnalyticsRequest{
		Instances: []*analyticsT.InstanceMsg{
			instanceMsg,
		},
		AdapterConfig: adapterConfig,
	}
	s.HandleAnalytics(ctx, r)

	if err := s.Close(); err != nil {
		t.Errorf("Close() returned an unexpected error")
	}

	if !uploaded {
		t.Errorf("analytics not delivered")
	}
}

func timestamp(t time.Time) *istio_policy_v1beta1.TimeStamp {
	return &istio_policy_v1beta1.TimeStamp{
		Value: &protobuf.Timestamp{
			Seconds: t.Unix(),
			Nanos:   int32(t.Nanosecond()),
		},
	}
}

func TestGRPCAdapter_HandleAuthorization(t *testing.T) {
	ctx := context.Background()

	ts := httptest.NewServer(cloudMockHandler(t))
	defer ts.Close()
	baseURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %s", err)
	}
	defer os.RemoveAll(d)

	s, err := adapter.NewGRPCAdapter("")
	if err != nil {
		t.Fatalf("unable to start server: %v", err)
	}

	cfg := &config.Params{
		ApigeeBase:   baseURL.String(),
		CustomerBase: baseURL.String(),
		OrgName:      "org",
		EnvName:      "env",
		Key:          "key",
		Secret:       "secret",
		TempDir:      d,
		Analytics: &config.ParamsAnalyticsOptions{
			FileLimit:       10,
			SendChannelSize: 0,
		},
		Products: &config.ParamsProductOptions{},
	}
	configBytes, err := cfg.Marshal()
	if err != nil {
		t.Fatalf("unable to marshal config: %v", err)
	}
	adapterConfig := &protobuf.Any{
		Value: configBytes,
	}

	instanceMsg := &authorization.InstanceMsg{
		Subject: &authorization.SubjectMsg{
			Properties: map[string]*istio_policy_v1beta1.Value{
				"api_key":     stringValue(""),
				"json_claims": stringValue(""),
			},
		},
		Action: &authorization.ActionMsg{
			Namespace: "default",
			Service:   "service",
			Method:    "GET",
			Path:      "/",
		},
	}

	r := &authorization.HandleAuthorizationRequest{
		Instance:      instanceMsg,
		AdapterConfig: adapterConfig,
	}
	checkResult, err := s.HandleAuthorization(ctx, r)
	if err != nil {
		t.Fatalf("error in HandleAuthorization: %v", err)
	}
	expected := status.WithUnauthenticated("missing authentication")
	if !reflect.DeepEqual(expected, checkResult.Status) {
		t.Errorf("checkResult expected: %v got: %v", expected, checkResult)
	}

	instanceMsg.Subject.Properties["api_key"] = stringValue("badkey")
	checkResult, err = s.HandleAuthorization(ctx, r)
	if err != nil {
		t.Errorf("error in HandleAuthorization: %v", err)
	}
	expected = status.WithPermissionDenied("permission denied")
	if !reflect.DeepEqual(expected, checkResult.Status) {
		t.Errorf("checkResult expected: %v got: %v", expected, checkResult)
	}

	instanceMsg.Subject.Properties["api_key"] = stringValue("goodkey")
	checkResult, err = s.HandleAuthorization(ctx, r)
	if err != nil {
		t.Errorf("error in HandleAuthorization: %v", err)
	}
	if !status.IsOK(checkResult.Status) {
		t.Errorf("checkResult expected: %v got: %v", status.OK, checkResult.Status)
	}

	if err := s.Close(); err != nil {
		t.Errorf("Close() returned an unexpected error")
	}
}

func stringValue(in string) *istio_policy_v1beta1.Value {
	return &istio_policy_v1beta1.Value{
		Value: &istio_policy_v1beta1.Value_StringValue{
			StringValue: in,
		},
	}
}
