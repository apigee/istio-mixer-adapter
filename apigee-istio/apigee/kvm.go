package apigee

import (
	"path"
)

const kvmPath = "keyvaluemaps"

// KVMService is an interface for interfacing with the Apigee Edge Admin API
// dealing with kvm.
type KVMService interface {
	Get(mapname string) (*KVM, *Response, error)
	Create(kvm KVM) (*Response, error)
}

type Entry struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type KVM struct {
	Name      string  `json:"name,omitempty"`
	Encrypted bool    `json:"encrypted,omitempty"`
	Entries   []Entry `json:"entry,omitempty"`
}

type KVMServiceOp struct {
	client *EdgeClient
}

var _ KVMService = &KVMServiceOp{}

func (s *KVMServiceOp) Get(mapname string) (*KVM, *Response, error) {
	path := path.Join(kvmPath, mapname)
	req, e := s.client.NewRequest("GET", path, nil)
	if e != nil {
		return nil, nil, e
	}
	returnedKVM := KVM{}
	resp, e := s.client.Do(req, &returnedKVM)
	if e != nil {
		return nil, resp, e
	}
	return &returnedKVM, resp, e
}

func (s *KVMServiceOp) Create(kvm KVM) (*Response, error) {
	path := path.Join(kvmPath)
	req, e := s.client.NewRequest("POST", path, kvm)
	if e != nil {
		return nil, e
	}
	resp, e := s.client.Do(req, &kvm)
	return resp, e
}
