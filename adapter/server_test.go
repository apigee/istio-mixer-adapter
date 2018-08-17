package adapter

//import (
//	"fmt"
//	"net/http"
//	"strings"
//	"testing"
//
//	"istio.io/istio/mixer/pkg/adapter/test"
//)
//
//func TestServer(t *testing.T) {
//	testAddr := "127.0.0.1:0"
//	s := newServer(testAddr)
//
//	if err := s.Start(test.NewEnv(t), http.HandlerFunc(noop)); err != nil {
//		t.Fatalf("Start() failed unexpectedly: %v", err)
//	}
//
//	updatedAddr := strings.Replace(testAddr, ":0", fmt.Sprintf(":%d", s.Port()), 1)
//	testURL := fmt.Sprintf("http://%s%s", updatedAddr, authPath)
//
//	resp, err := http.Get(testURL)
//	if err != nil {
//		t.Fatalf("Failed to retrieve '%s' path: %v", authPath, err)
//	}
//
//	if resp.StatusCode != http.StatusOK {
//		t.Errorf("http.GET => %v, wanted '%v'", resp.StatusCode, http.StatusOK)
//	}
//
//	_ = resp.Body.Close()
//
//	s2 := newServer(updatedAddr)
//	if err := s2.Start(test.NewEnv(t), http.HandlerFunc(noop)); err == nil {
//		t.Fatal("Start() succeeded, expecting a failure")
//	}
//
//	if err := s.Close(); err != nil {
//		t.Errorf("Failed to close server properly: %v", err)
//	}
//
//	if err := s2.Close(); err != nil {
//		t.Errorf("Failed to close server properly: %v", err)
//	}
//}
//
//func TestServerInst_Close(t *testing.T) {
//	testAddr := "127.0.0.1:0"
//	s := newServer(testAddr)
//	env := test.NewEnv(t)
//
//	if err := s.Close(); err != nil {
//		t.Fatalf("Failed to close server properly: %v", err)
//	}
//
//	if err := s.Start(env, http.HandlerFunc(noop)); err != nil {
//		t.Fatalf("Start() failed unexpectedly: %v", err)
//	}
//
//	if err := s.Start(env, http.HandlerFunc(noop)); err != nil {
//		t.Fatalf("Start() failed unexpectedly: %v", err)
//	}
//
//	if s.srv == nil {
//		t.Fatalf("expected server to be non-nil")
//	}
//
//	if err := s.Close(); err != nil {
//		t.Fatalf("Failed to close server properly: %v", err)
//	}
//
//	if s.srv == nil {
//		t.Fatalf("expected server to be non-nil")
//	}
//
//	if err := s.Close(); err != nil {
//		t.Fatalf("Failed to close server properly: %v", err)
//	}
//
//	if s.srv != nil {
//		t.Fatalf("expected server to be nil: %v", s.srv)
//	}
//}
//
//func noop(http.ResponseWriter, *http.Request) {}
