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
	UpdateEntry(kvmName string, entry Entry) (*Response, error)
	AddEntry(kvmName string, entry Entry) (*Response, error)
}

// Entry is an entry in the KVM
type Entry struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// KVM represents an Apigee KVM
type KVM struct {
	Name      string  `json:"name,omitempty"`
	Encrypted bool    `json:"encrypted,omitempty"`
	Entries   []Entry `json:"entry,omitempty"`
}

// GetValue returns a value from the KVM
func (k *KVM) GetValue(name string) (v string, ok bool) {
	for _, e := range k.Entries {
		if e.Name == name {
			return e.Value, true
		}
	}
	return
}

// KVMServiceOp represents a KVM service operation
type KVMServiceOp struct {
	client *EdgeClient
}

var _ KVMService = &KVMServiceOp{}

// Get returns a response given a KVM map name
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

// Create creates a KVM and returns a response
func (s *KVMServiceOp) Create(kvm KVM) (*Response, error) {
	path := path.Join(kvmPath)
	req, e := s.client.NewRequest("POST", path, kvm)
	if e != nil {
		return nil, e
	}
	resp, e := s.client.Do(req, &kvm)
	return resp, e
}

// UpdateEntry updates a KVM entry
func (s *KVMServiceOp) UpdateEntry(kvmName string, entry Entry) (*Response, error) {
	path := path.Join(kvmPath, kvmName, "entries", entry.Name)
	req, e := s.client.NewRequest("POST", path, entry)
	if e != nil {
		return nil, e
	}
	resp, e := s.client.Do(req, &entry)
	return resp, e
}

// AddEntry add an entry to the KVM
func (s *KVMServiceOp) AddEntry(kvmName string, entry Entry) (*Response, error) {
	path := path.Join(kvmPath, kvmName, "entries")
	req, e := s.client.NewRequest("POST", path, entry)
	if e != nil {
		return nil, e
	}
	resp, e := s.client.Do(req, &entry)
	return resp, e
}
