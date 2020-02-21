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
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/log"
	"github.com/apigee/istio-mixer-adapter/apigee/util"
	"github.com/google/uuid"
)

const (
	tagFormat = "%s.%s.%s.%s" // recType, org, env, clientUUID
	recType   = "api"

	useMTLSKey      = "conf_datadispatcher_use.mtls"
	caCertLocKey    = "conf_datadispatcher_ca.pem.location"
	tlsCertLocKey   = "conf_datadispatcher_certificate.pem.location"
	tlsKeyLocKey    = "conf_datadispatcher_key.pem.location"
	udcaEndpointKey = "conf_datadispatcher_destination.batch"

	fluentdFormat = "[\"%s\", %d, %s]\n" // tag, unix timestamp, record json
)

func newHybridUploader(opts Options) (*hybridUploader, error) {
	props, err := util.ReadPropertiesFile(opts.HybridConfigFile)
	if err != nil {
		return nil, err
	}
	addr := props[udcaEndpointKey]

	tlsConfig, err := loadTLSConfig(props)
	if err != nil {
		return nil, err
	}

	return &hybridUploader{
		network:    "tcp",
		addr:       addr,
		tlsConfig:  tlsConfig,
		now:        opts.now,
		clientUUID: uuid.New().String(),
	}, nil
}

type hybridUploader struct {
	network    string
	addr       string
	tlsConfig  *tls.Config
	now        func() time.Time
	clientUUID string
}

func (h *hybridUploader) isGzipped() bool {
	return false
}

func (h *hybridUploader) workFunc(tenant, fileName string) util.WorkFunc {
	return func(ctx context.Context) error {
		if ctx.Err() == nil {
			return h.upload(fileName)
		}

		log.Warningf("canceled upload of %s: %v", fileName, ctx.Err())
		if err := os.Remove(fileName); err != nil && !os.IsNotExist(err) {
			log.Warningf("unable to remove file %s: %v", fileName, err)
		}
		return nil
	}
}

// format and write records
func (h *hybridUploader) write(incoming []Record, writer io.Writer) error {

	now := h.now()
	for _, record := range incoming {
		recJSON, err := json.Marshal(record)
		if err != nil {
			log.Errorf("dropping unmarshallable record %v: %s", record, err)
			continue
		}

		tag := fmt.Sprintf(tagFormat, recType, record.Organization, record.Environment, h.clientUUID)
		data := fmt.Sprintf(fluentdFormat, tag, now.Unix(), recJSON)
		log.Debugf("queuing analytics record for fluentd: %s", data)

		if _, err := writer.Write([]byte(data)); err != nil {
			return err
		}
	}

	return nil
}

// upload sends a file to UDCA
func (h *hybridUploader) upload(fileName string) error {

	client, err := tls.Dial(h.network, h.addr, h.tlsConfig)
	if err != nil {
		log.Errorf("dial: %s", err)
		return err
	}
	defer client.Close()

	file, err := os.Open(fileName)
	if err != nil {
		log.Errorf("open: %s: %v", fileName, err)
		return err
	}
	defer file.Close()

	_, err = io.Copy(client, file)
	return err
}

func loadTLSConfig(props map[string]string) (*tls.Config, error) {

	if props[useMTLSKey] != "true" {
		return nil, nil
	}

	// ca cert pool
	caCert, err := ioutil.ReadFile(props[caCertLocKey])
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(caCert)
	if !ok {
		return nil, err
	}

	//  tls key pair
	cert, err := tls.LoadX509KeyPair(props[tlsCertLocKey], props[tlsKeyLocKey])
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{cert},
	}, nil
}
