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

// build the protos
//go:generate $GOPATH/src/github.com/apigee/istio-mixer-adapter/bin/codegen.sh -a adapter/config/config.proto -x "-n apigee -s=false -t apigee-authorization -t apigee-analytics"

package adapter

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/apigee/istio-mixer-adapter/adapter/config"
	"github.com/apigee/istio-mixer-adapter/template/analytics"
	"google.golang.org/grpc"
	model "istio.io/api/mixer/adapter/model/v1beta1"
	policy "istio.io/api/policy/v1beta1"
	"istio.io/istio/mixer/pkg/adapter"
	"istio.io/istio/mixer/pkg/pool"
	rtHandler "istio.io/istio/mixer/pkg/runtime/handler"
	"istio.io/istio/mixer/template/authorization"
)

type (
	// Server is basic server interface
	Server interface {
		Addr() string
		Close() error
		Run(shutdown chan error)
	}

	// GRPCAdapter supports templates
	GRPCAdapter struct {
		listener net.Listener
		server   *grpc.Server

		defaultConfig []byte
		rawConfig     []byte
		builder       adapter.HandlerBuilder
		handler       adapter.Handler
		env           adapter.Env
		builderLock   sync.RWMutex
	}
)

// Ensure required interfaces are implemented.
var _ authorization.HandleAuthorizationServiceServer = &GRPCAdapter{}
var _ analytics.HandleAnalyticsServiceServer = &GRPCAdapter{}

func (g *GRPCAdapter) HandleAuthorization(ctx context.Context,
	r *authorization.HandleAuthorizationRequest) (*model.CheckResult, error) {

	g.env.Logger().Infof("HandleAuthorization: %v", r)

	h, err := g.getHandler(r.AdapterConfig.Value)
	if err != nil {
		return nil, err
	}

	cr, err := h.(authorization.Handler).HandleAuthorization(ctx, instance(r.Instance))
	if err != nil {
		g.env.Logger().Errorf("Could not process: %v", err)
		return nil, err
	}

	return &model.CheckResult{
		Status:        cr.Status,
		ValidDuration: cr.ValidDuration,
		ValidUseCount: cr.ValidUseCount,
	}, nil
}

func (g *GRPCAdapter) HandleAnalytics(ctx context.Context,
	r *analytics.HandleAnalyticsRequest) (*model.ReportResult, error) {

	g.env.Logger().Infof("HandleAnalytics: %v", r)

	h, err := g.getHandler(r.AdapterConfig.Value)
	if err != nil {
		return nil, err
	}

	err = h.(analytics.Handler).HandleAnalytics(ctx, analyticsInstances(r.Instances))
	if err != nil {
		g.env.Logger().Errorf("Could not process: %v", err)
		return nil, err
	}

	return &model.ReportResult{}, nil
}

func instance(in *authorization.InstanceMsg) *authorization.Instance {
	return instances([]*authorization.InstanceMsg{in})[0]
}

func instances(in []*authorization.InstanceMsg) []*authorization.Instance {
	out := make([]*authorization.Instance, 0, len(in))
	for _, inst := range in {
		out = append(out, &authorization.Instance{
			Name:    inst.Name,
			Subject: decodeSubject(inst.Subject),
			Action:  decodeAction(inst.Action),
		})
	}
	return out
}

func analyticsInstances(in []*analytics.InstanceMsg) []*analytics.Instance {
	out := make([]*analytics.Instance, 0, len(in))
	for _, inst := range in {
		out = append(out, &analytics.Instance{
			Name:                         inst.Name,
			ApiProxy:                     inst.ApiProxy,
			ResponseStatusCode:           inst.ResponseStatusCode,
			ClientIp:                     inst.ClientIp.Value,
			RequestVerb:                  inst.RequestVerb,
			RequestPath:                  inst.RequestPath,
			Useragent:                    inst.Useragent,
			ClientReceivedStartTimestamp: decodeTimestamp(inst.ClientReceivedStartTimestamp),
			ClientReceivedEndTimestamp:   decodeTimestamp(inst.ClientReceivedEndTimestamp),
			ClientSentStartTimestamp:     decodeTimestamp(inst.ClientSentStartTimestamp),
			ClientSentEndTimestamp:       decodeTimestamp(inst.ClientSentEndTimestamp),
			TargetSentStartTimestamp:     decodeTimestamp(inst.TargetSentStartTimestamp),
			TargetSentEndTimestamp:       decodeTimestamp(inst.TargetSentEndTimestamp),
			TargetReceivedStartTimestamp: decodeTimestamp(inst.TargetReceivedStartTimestamp),
			TargetReceivedEndTimestamp:   decodeTimestamp(inst.TargetReceivedEndTimestamp),
			ApiClaims:                    inst.ApiClaims,
			ApiKey:                       inst.ApiKey,
		})
	}
	return out
}

func decodeSubject(in *authorization.SubjectMsg) *authorization.Subject {
	return &authorization.Subject{
		User:       in.User,
		Groups:     in.Groups,
		Properties: decodeValueMap(in.Properties),
	}
}

func decodeAction(in *authorization.ActionMsg) *authorization.Action {
	return &authorization.Action{
		Namespace:  in.Namespace,
		Service:    in.Service,
		Method:     in.Method,
		Path:       in.Path,
		Properties: decodeValueMap(in.Properties),
	}
}

func decodeValueMap(in map[string]*policy.Value) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = decodeValue(v.GetValue())
	}
	return out
}

func decodeTimestamp(t *policy.TimeStamp) time.Time {
	return time.Unix(t.GetValue().Seconds, int64(t.GetValue().Nanos))
}

func decodeValue(in interface{}) interface{} {
	switch t := in.(type) {
	case *policy.Value_StringValue:
		return t.StringValue
	case *policy.Value_Int64Value:
		return t.Int64Value
	case *policy.Value_DoubleValue:
		return t.DoubleValue
	default:
		return fmt.Sprintf("%v", in)
	}
}

// todo: this only supports a single config, consider multiple
func (g *GRPCAdapter) getHandler(rawConfig []byte) (adapter.Handler, error) {
	g.builderLock.RLock()

	// todo: this is a hack
	if g.handler != nil {
		return g.handler, nil
	}

	if 0 == bytes.Compare(rawConfig, g.rawConfig) {
		h := g.handler
		g.builderLock.RUnlock()
		return h, nil
	}
	g.builderLock.RUnlock()

	// todo: will this even work?
	cfg := &config.Params{}
	if err := cfg.Unmarshal(g.defaultConfig); err != nil {
		return nil, err
	}
	if err := cfg.Unmarshal(rawConfig); err != nil {
		return nil, err
	}

	g.builderLock.Lock()
	defer g.builderLock.Unlock()

	// recheck
	if 0 == bytes.Compare(rawConfig, g.rawConfig) {
		return g.handler, nil
	}

	g.env.Logger().Infof("Loaded handler with: %v", cfg)

	g.builder.SetAdapterConfig(cfg)
	if ce := g.builder.Validate(); ce != nil {
		return nil, ce
	}

	h, err := g.builder.Build(context.Background(), g.env)
	if err != nil {
		g.env.Logger().Errorf("could not build: %v", err)
		return nil, err
	}
	g.handler = h
	g.rawConfig = rawConfig
	return h, err
}

// Addr returns the listening address of the server
func (g *GRPCAdapter) Addr() string {
	return g.listener.Addr().String()
}

// Run starts the server run
func (g *GRPCAdapter) Run(shutdown chan error) {
	shutdown <- g.server.Serve(g.listener)
}

// Close gracefully shuts down the server; used for testing
func (g *GRPCAdapter) Close() error {
	if g.server != nil {
		g.server.GracefulStop()
	}

	if g.listener != nil {
		_ = g.listener.Close()
	}

	return nil
}

// NewGRPCAdapter creates a new no session server from given args.
func NewGRPCAdapter(addr string) (*GRPCAdapter, error) {
	goroutinePool := pool.NewGoroutinePool(5, false)
	info := GetInfo()
	defaultConfig, err := info.DefaultConfig.(*config.Params).Marshal()
	if err != nil {
		return nil, err
	}
	s := &GRPCAdapter{
		defaultConfig: defaultConfig,
		builder:       info.NewBuilder(),
		env:           rtHandler.NewEnv(0, "apigee", goroutinePool),
		rawConfig:     []byte{0xff, 0xff},
	}
	if s.listener, err = net.Listen("tcp", addr); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("unable to listen on socket: %v", err)
	}

	s.env.Logger().Infof("listening on :%v\n", s.listener.Addr())
	s.server = grpc.NewServer()
	authorization.RegisterHandleAuthorizationServiceServer(s.server, s)
	analytics.RegisterHandleAnalyticsServiceServer(s.server, s)
	return s, nil
}
