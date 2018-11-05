// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package on_demand_service_broker_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/config"

	"github.com/pivotal-cf/on-demand-service-broker/collaboration_tests/helpers"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"

	"os"
	"testing"

	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"math"

	"io"
	"io/ioutil"
	"net/http"

	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	credhubfakes "github.com/pivotal-cf/on-demand-service-broker/credhubbroker/fakes"
	manifestsecretsfakes "github.com/pivotal-cf/on-demand-service-broker/manifestsecrets/fakes"
	serviceadapterfakes "github.com/pivotal-cf/on-demand-service-broker/serviceadapter/fakes"
	taskfakes "github.com/pivotal-cf/on-demand-service-broker/task/fakes"
)

func TestOnDemandServiceBroker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OnDemandServiceBroker Collaboration Test Suite")
}

const (
	serviceName    = "service-name"
	brokerUsername = "username"
	brokerPassword = "password"
)

var (
	stopServer          chan os.Signal
	serverPort          = rand.Intn(math.MaxInt16-1024) + 1024
	serverURL           = fmt.Sprintf("localhost:%d", serverPort)
	fakeServiceAdapter  *fakes.FakeServiceAdapterClient
	fakeCredentialStore *credhubfakes.FakeCredentialStore
	fakeBoshClient      *fakes.FakeBoshClient
	fakeCfClient        *fakes.FakeCloudFoundryClient
	fakeTaskBoshClient  *taskfakes.FakeBoshClient
	fakeCommandRunner   *serviceadapterfakes.FakeCommandRunner
	fakeTaskBulkSetter  *taskfakes.FakeBulkSetter
	loggerBuffer        *gbytes.Buffer
	shouldSendSigterm   bool

	fakeCredhubOperator *manifestsecretsfakes.FakeCredhubOperator

	brokerServer *helpers.Server
)

var _ = BeforeEach(func() {
	fakeBoshClient = new(fakes.FakeBoshClient)
	fakeServiceAdapter = new(fakes.FakeServiceAdapterClient)
	fakeCredentialStore = new(credhubfakes.FakeCredentialStore)
	fakeCfClient = new(fakes.FakeCloudFoundryClient)

	fakeTaskBoshClient = new(taskfakes.FakeBoshClient)
	fakeCommandRunner = new(serviceadapterfakes.FakeCommandRunner)

	generateManifestOutput := serviceadapter.MarshalledGenerateManifest{
		Manifest: `name: service-instance_some-instance-id`,
		ODBManagedSecrets: map[string]interface{}{
			"": nil,
		},
	}
	generateManifestOutputBytes, err := json.Marshal(generateManifestOutput)
	Expect(err).NotTo(HaveOccurred())
	zero := 0
	fakeCommandRunner.RunWithInputParamsReturns(generateManifestOutputBytes, []byte{}, &zero, nil)

	fakeTaskBulkSetter = new(taskfakes.FakeBulkSetter)
	fakeCredhubOperator = new(manifestsecretsfakes.FakeCredhubOperator)
})

var _ = AfterEach(func() {
	if shouldSendSigterm {
		brokerServer.Close()
	}
})

func StartServer(conf config.Config) {
	loggerBuffer = gbytes.NewBuffer()
	shouldSendSigterm = true
	stopServer = make(chan os.Signal, 1)
	brokerServer = helpers.StartServer(
		conf,
		stopServer,
		fakeCommandRunner,
		fakeTaskBoshClient,
		fakeTaskBulkSetter,
		fakeCfClient,
		fakeBoshClient,
		fakeServiceAdapter,
		fakeCredentialStore,
		fakeCredhubOperator,
		loggerBuffer,
	)
}

func StartServerWithStopHandler(conf config.Config, stopServerChan chan os.Signal) {
	loggerBuffer = gbytes.NewBuffer()
	brokerServer = helpers.StartServer(
		conf,
		stopServerChan,
		fakeCommandRunner,
		fakeTaskBoshClient,
		fakeTaskBulkSetter,
		fakeCfClient,
		fakeBoshClient,
		fakeServiceAdapter,
		fakeCredentialStore,
		fakeCredhubOperator,
		loggerBuffer,
	)
}

func doRequest(method, url string, body io.Reader, requestModifiers ...func(r *http.Request)) (*http.Response, []byte) {
	req, err := http.NewRequest(method, url, body)
	Expect(err).ToNot(HaveOccurred())

	req.SetBasicAuth(brokerUsername, brokerPassword)
	req.Header.Set("X-Broker-API-Version", "2.14")

	for _, f := range requestModifiers {
		f(req)
	}

	req.Close = true
	resp, err := http.DefaultClient.Do(req)
	Expect(err).ToNot(HaveOccurred())

	bodyContent, err := ioutil.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	Expect(resp.Body.Close()).To(Succeed())
	return resp, bodyContent
}

func doHTTPSRequest(method, url string, caCertFile string, cipherSuites []uint16, maxTLSVersion uint16) (*http.Response, []byte, error) {
	Expect(url).To(ContainSubstring("https"))

	// Load CA cert
	caCert, err := ioutil.ReadFile(caCertFile)
	Expect(err).NotTo(HaveOccurred())
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Setup HTTPS client
	tlsConfig := &tls.Config{
		RootCAs:      caCertPool,
		CipherSuites: cipherSuites,
	}
	if maxTLSVersion != 0 {
		tlsConfig.MaxVersion = maxTLSVersion
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}

	req, err := http.NewRequest(method, url, nil)
	Expect(err).ToNot(HaveOccurred())

	req.SetBasicAuth(brokerUsername, brokerPassword)
	req.Header.Set("X-Broker-API-Version", "2.14")

	req.Close = true
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()
	bodyContent, err := ioutil.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	return resp, bodyContent, nil
}
