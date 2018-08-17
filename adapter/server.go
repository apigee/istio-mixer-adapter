package adapter

//import (
//	"context"
//	"fmt"
//	"io"
//	"net"
//	"net/http"
//	"sync"
//	"time"
//
//	"istio.io/istio/mixer/pkg/adapter"
//)
//
//type (
//	// Server represents an HTTP Server for the adapter
//	Server interface {
//		io.Closer
//		Start(adapter.Env, http.Handler) error
//		Port() int
//	}
//
//	serverInst struct {
//		addr string
//
//		lock    sync.Mutex // protects resources below
//		srv     *http.Server
//		handler *metaHandler
//		refCnt  int
//
//		port int // port this server instance is listening on
//	}
//)
//
//const (
//	serverName  = "Apigee auth"
//	authPath    = "/authorization"
//	defaultAddr = ":42422"
//)
//
//func newServer(addr string) *serverInst {
//	if addr == "" {
//		addr = defaultAddr
//	}
//	return &serverInst{addr: addr}
//}
//
//// metaHandler switches the delegate without downtime
//type metaHandler struct {
//	delegate http.Handler
//	lock     sync.RWMutex
//}
//
//func (m *metaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//	m.lock.RLock()
//	m.delegate.ServeHTTP(w, r)
//	m.lock.RUnlock()
//}
//
//func (m *metaHandler) setDelegate(delegate http.Handler) {
//	m.lock.Lock()
//	m.delegate = delegate
//	m.lock.Unlock()
//}
//
//func (s *serverInst) Port() int {
//	return s.port
//}
//
//// Start the singleton listener
//func (s *serverInst) Start(env adapter.Env, metricsHandler http.Handler) (err error) {
//	s.lock.Lock()
//	defer s.lock.Unlock()
//
//	// if server is already running, just switch the delegate handler
//	if s.srv != nil {
//		s.refCnt++
//		s.handler.setDelegate(metricsHandler)
//		return nil
//	}
//
//	listener, err := net.Listen("tcp", s.addr)
//	if err != nil {
//		return fmt.Errorf("could not start %s server: %v", serverName, err)
//	}
//
//	s.port = listener.Addr().(*net.TCPAddr).Port
//
//	// handle auth
//	srvMux := http.NewServeMux()
//	s.handler = &metaHandler{delegate: metricsHandler}
//	srvMux.Handle(authPath, s.handler)
//	srv := &http.Server{Addr: s.addr, Handler: srvMux}
//	env.ScheduleDaemon(func() {
//		env.Logger().Infof("serving %s on %d", serverName, s.port)
//		if err := srv.Serve(listener.(*net.TCPListener)); err != nil {
//			if err == http.ErrServerClosed {
//				env.Logger().Infof("%s HTTP server stopped", serverName)
//			} else {
//				_ = env.Logger().Errorf("%s HTTP server error: %v", serverName, err) // nolint: gas
//			}
//		}
//	})
//	s.srv = srv
//	s.refCnt++
//
//	return nil
//}
//
//// Close decrements ref count and closes the server if <= 0
//func (s *serverInst) Close() error {
//	s.lock.Lock()
//	defer s.lock.Unlock()
//
//	if s.srv == nil {
//		return nil
//	}
//
//	s.refCnt--
//	if s.refCnt > 0 {
//		return nil
//	}
//	s.refCnt = 0
//
//	// close
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	srv := s.srv
//	s.srv = nil
//	s.handler = nil
//	return srv.Shutdown(ctx)
//}
