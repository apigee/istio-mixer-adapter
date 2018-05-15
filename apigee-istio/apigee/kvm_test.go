package apigee

//func TestGet(t *testing.T) {
//
//	org := "demo"
//	env := "test"
//	auth := EdgeAuth{Username: "", Password: ""}
//	opts := &EdgeClientOptions{Org: org, Env: env, Auth: &auth, Debug: true}
//	client, e := NewEdgeClient(opts)
//
//	if e != nil {
//		t.Error(e)
//		return
//	}
//
//	kvm, resp, err := client.KVMService.Get("istio")
//
//	if err != nil {
//		t.Error(err)
//		return
//	}
//
//	if resp != nil {
//		t.Log(resp.StatusCode)
//	}
//
//	if kvm != nil {
//		t.Log(kvm)
//	}
//}
//
//func TestCreate(t *testing.T) {
//	kvm := KVM{}
//	kvm.Name = "test"
//	kvm.Encrypted = false
//	entry := new(Entry)
//	entry.Name = "test"
//	entry.Value = "value"
//	kvm.Entries = append(kvm.Entries, *entry)
//
//	org := "demo"
//	env := "test"
//	auth := EdgeAuth{Username: "", Password: ""}
//	opts := &EdgeClientOptions{Org: org, Env: env, Auth: &auth, Debug: true}
//	client, e := NewEdgeClient(opts)
//
//	if e != nil {
//		t.Error(e)
//		return
//	}
//
//	resp, err := client.KVMService.Create(kvm)
//
//	if err != nil {
//		t.Error(err)
//		return
//	}
//
//	if resp != nil {
//		t.Log(resp.StatusCode)
//	}
//}
