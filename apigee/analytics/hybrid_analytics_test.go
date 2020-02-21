// Copyright 2020 Google LLC
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
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/util"
)

func TestHybridAnalyticsSubmit(t *testing.T) {
	t.Parallel()
	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018
	now := func() time.Time { return time.Unix(ts, 0) }
	startTime := now()

	context := &TestContext{
		orgName: "org",
		envName: "env",
	}
	authContext := &auth.Context{
		Context:        context,
		DeveloperEmail: "email",
		Application:    "app",
		AccessToken:    "token",
		ClientID:       "clientId",
	}
	axRecord := Record{
		ResponseStatusCode:           201,
		RequestVerb:                  "PATCH",
		RequestPath:                  "/test",
		UserAgent:                    "007",
		ClientReceivedStartTimestamp: timeToUnix(startTime),
		ClientReceivedEndTimestamp:   timeToUnix(startTime),
		ClientSentStartTimestamp:     timeToUnix(startTime),
		ClientSentEndTimestamp:       timeToUnix(startTime),
		TargetSentStartTimestamp:     timeToUnix(startTime),
		TargetSentEndTimestamp:       timeToUnix(startTime),
		TargetReceivedStartTimestamp: timeToUnix(startTime),
		TargetReceivedEndTimestamp:   timeToUnix(startTime),
	}

	port, err := util.FreePort()
	if err != nil {
		t.Fatal(err)
	}
	tf, err := createPropsFile(port, true)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tf.Name())

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %s", err)
	}
	defer os.RemoveAll(d)

	opts := Options{
		BufferPath:         d,
		StagingFileLimit:   1,
		BaseURL:            &url.URL{},
		Key:                "x",
		Secret:             "x",
		Client:             http.DefaultClient,
		HybridConfigFile:   tf.Name(), // key to creating a hybrid manager
		now:                now,
		CollectionInterval: time.Minute,
	}
	mgr, err := NewManager(opts)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		if err := mgr.SendRecords(authContext, []Record{axRecord}); err != nil {
			panic(err)
		}
		mgr.Close() // force write
	}()
	endpoint := fmt.Sprintf("localhost:%d", port)

	cert, err := tls.LoadX509KeyPair("testdata/cert.pem", "testdata/key.pem")
	if err != nil {
		t.Fatal(err)
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
	listener, err := tls.Listen("tcp", endpoint, tlsConfig)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("scan failed")
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	got := scanner.Text()

	up := mgr.(*manager).uploader
	uuid := up.(*hybridUploader).clientUUID

	tag := fmt.Sprintf(tagFormat, recType, context.Organization(), context.Environment(), uuid)
	axRecord = axRecord.ensureFields(authContext)
	axJSON, err := json.Marshal(axRecord)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("[\"%s\", %d, %s]", tag, ts, axJSON)

	// the gatewayFlowID value is variable, just trim it off
	if got[:len(got)-40] != want[:len(want)-40] {
		t.Errorf("got record: %s, want: %s", got, want)
	}
}

func createPropsFile(port int, useMTLS bool) (*os.File, error) {

	propsData := fmt.Sprintf(`
	conf_datadispatcher_destination.batch=localhost:%d
	conf_datadispatcher_use.mtls=true
	conf_datadispatcher_ca.pem.location=testdata/cert.pem
	conf_datadispatcher_certificate.pem.location=testdata/cert.pem
	conf_datadispatcher_key.pem.location=testdata/key.pem
	`, port)

	tf, err := ioutil.TempFile("", "test.props")
	if err != nil {
		return nil, err
	}

	if _, err := tf.WriteString(propsData); err != nil {
		return nil, err
	}

	if err := tf.Close(); err != nil {
		return nil, err
	}

	return tf, nil
}
