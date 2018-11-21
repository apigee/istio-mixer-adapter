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

const workerPoolSize = 10

type (
	// Server is the gRPC server instance
	Server interface {
		Addr() string
		Close() error
		Run(shutdown chan error)
	}

	handlerMap map[string]*ApigeeHandler // tenant name -> handler

	// GRPCAdapter handles multi-tenancy
	GRPCAdapter struct {
		listener net.Listener
		server   *grpc.Server

		info         adapter.Info
		handlers     handlerMap
		handlersLock sync.RWMutex
	}

	// ApigeeHandler handles a single tenant (org/env)
	ApigeeHandler struct {
		env     adapter.Env
		handler adapter.Handler
	}
)

// Ensure required interfaces are implemented.
var _ authorization.HandleAuthorizationServiceServer = &GRPCAdapter{}
var _ analytics.HandleAnalyticsServiceServer = &GRPCAdapter{}

// HandleAuthorization is a gRPC endpoint
func (g *GRPCAdapter) HandleAuthorization(ctx context.Context,
	r *authorization.HandleAuthorizationRequest) (*model.CheckResult, error) {

	h, err := g.getHandler(r.AdapterConfig.Value)
	if err != nil {
		return nil, err
	}

	cr, err := h.HandleAuthorization(ctx, r.Instance)
	if err != nil {
		return nil, err
	}

	return &model.CheckResult{
		Status:        cr.Status,
		ValidDuration: cr.ValidDuration,
		ValidUseCount: cr.ValidUseCount,
	}, nil
}

// HandleAnalytics is a gRPC endpoint
func (g *GRPCAdapter) HandleAnalytics(ctx context.Context,
	r *analytics.HandleAnalyticsRequest) (*model.ReportResult, error) {

	h, err := g.getHandler(r.AdapterConfig.Value)
	if err != nil {
		return nil, err
	}

	err = h.HandleAnalytics(ctx, r.Instances)
	return &model.ReportResult{}, err
}

// maintains exactly one per org/env (the first one in)
func (g *GRPCAdapter) getHandler(rawConfig []byte) (*ApigeeHandler, error) {

	cfg := *g.info.DefaultConfig.(*config.Params)
	if err := cfg.Unmarshal(rawConfig); err != nil {
		return nil, err
	}

	g.handlersLock.RLock()

	tenant := fmt.Sprintf("%s~%s", cfg.OrgName, cfg.EnvName)

	apigeeHandler, ok := g.handlers[tenant]
	if ok {
		g.handlersLock.RUnlock()
		return apigeeHandler, nil
	}

	g.handlersLock.RUnlock()
	g.handlersLock.Lock()
	defer g.handlersLock.Unlock()

	// check again
	apigeeHandler, ok = g.handlers[tenant]
	if ok {
		return apigeeHandler, nil
	}

	// create new handler
	goroutinePool := pool.NewGoroutinePool(workerPoolSize, false)
	goroutinePool.AddWorkers(workerPoolSize)
	env := rtHandler.NewEnv(0, tenant, goroutinePool)
	apigeeHandler = &ApigeeHandler{
		env: env,
	}

	ctx := context.Background()
	builder := g.info.NewBuilder()

	builder.SetAdapterConfig(&cfg)
	if errs := builder.Validate(); errs != nil {
		return nil, errs
	}

	h, err := builder.Build(ctx, env)
	if err != nil {
		env.Logger().Errorf("could not build handler: %v", err)
		return nil, err
	}
	apigeeHandler.handler = h

	env.Logger().Infof("created apigee tenant handler")

	g.handlers[tenant] = apigeeHandler
	return apigeeHandler, nil
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

	g.handlersLock.Lock()
	defer g.handlersLock.Unlock()
	for _, h := range g.handlers {
		h.handler.Close()
	}

	return nil
}

// HandleAuthorization is in the context of a single tenant
func (h *ApigeeHandler) HandleAuthorization(ctx context.Context, im *authorization.InstanceMsg) (*model.CheckResult, error) {
	h.env.Logger().Debugf("HandleAuthorization: %v", im)

	decodeValue := func(in interface{}) interface{} {
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

	decodeValueMap := func(in map[string]*policy.Value) map[string]interface{} {
		out := make(map[string]interface{}, len(in))
		for k, v := range in {
			out[k] = decodeValue(v.GetValue())
		}
		return out
	}

	subject := &authorization.Subject{
		User:       im.Subject.User,
		Groups:     im.Subject.Groups,
		Properties: decodeValueMap(im.Subject.Properties),
	}

	action := &authorization.Action{
		Namespace:  im.Action.Namespace,
		Service:    im.Action.Service,
		Method:     im.Action.Method,
		Path:       im.Action.Path,
		Properties: decodeValueMap(im.Action.Properties),
	}

	inst := &authorization.Instance{
		Name:    im.Name,
		Subject: subject,
		Action:  action,
	}

	cr, err := h.handler.(authorization.Handler).HandleAuthorization(ctx, inst)
	if err != nil {
		h.env.Logger().Errorf("Could not process: %v", err)
		return nil, err
	}

	return &model.CheckResult{
		Status:        cr.Status,
		ValidDuration: cr.ValidDuration,
		ValidUseCount: cr.ValidUseCount,
	}, nil
}

// HandleAnalytics is in the context of a single tenant
func (h *ApigeeHandler) HandleAnalytics(ctx context.Context, im []*analytics.InstanceMsg) error {
	h.env.Logger().Debugf("HandleAnalytics: %v", im)

	decodeTimestamp := func(t *policy.TimeStamp) time.Time {
		if t == nil {
			return time.Time{}
		}
		return time.Unix(t.GetValue().Seconds, int64(t.GetValue().Nanos))
	}

	instances := make([]*analytics.Instance, 0, len(im))
	for _, inst := range im {
		instances = append(instances, &analytics.Instance{
			Name:                         inst.Name,
			ApiProxy:                     inst.ApiProxy,
			ResponseStatusCode:           inst.ResponseStatusCode,
			ClientIp:                     inst.ClientIp.Value,
			RequestVerb:                  inst.RequestVerb,
			RequestPath:                  inst.RequestPath,
			RequestUri:                   inst.RequestUri,
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

	err := h.handler.(analytics.Handler).HandleAnalytics(ctx, instances)
	if err != nil {
		h.env.Logger().Errorf("Could not process: %v", err)
		return err
	}

	return nil
}

// NewGRPCAdapter creates a new no session server from given args.
func NewGRPCAdapter(addr string) (*GRPCAdapter, error) {
	s := &GRPCAdapter{
		info:     GetInfo(),
		handlers: handlerMap{},
	}
	var err error
	if s.listener, err = net.Listen("tcp", addr); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("unable to listen on socket: %v", err)
	}
	fmt.Printf("listening on :%v\n", s.listener.Addr())

	s.server = grpc.NewServer()
	authorization.RegisterHandleAuthorizationServiceServer(s.server, s)
	analytics.RegisterHandleAnalyticsServiceServer(s.server, s)
	return s, nil
}
