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
	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/service"

	"os"
	"syscall"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/task"

	"math/rand"

	"math"

	"io"
	"io/ioutil"
	"net/http"

	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	credhubfakes "github.com/pivotal-cf/on-demand-service-broker/credhubbroker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/manifestsecrets"
	manifestsecretsfakes "github.com/pivotal-cf/on-demand-service-broker/manifestsecrets/fakes"
	taskfakes "github.com/pivotal-cf/on-demand-service-broker/task/fakes"
)

func TestOnDemandServiceBroker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OnDemandServiceBroker Collaboration Test Suite")
}

const (
	componentName  = "collaboration-tests"
	serviceName    = "service-name"
	brokerUsername = "username"
	brokerPassword = "password"
)

var (
	stopServer                chan os.Signal
	serverPort                = rand.Intn(math.MaxInt16-1024) + 1024
	serverURL                 = fmt.Sprintf("localhost:%d", serverPort)
	deployer                  broker.Deployer
	fakeServiceAdapter        *fakes.FakeServiceAdapterClient
	fakeCredentialStore       *credhubfakes.FakeCredentialStore
	fakeBoshClient            *fakes.FakeBoshClient
	fakeCfClient              *fakes.FakeCloudFoundryClient
	fakeTaskBoshClient        *taskfakes.FakeBoshClient
	fakeTaskManifestGenerator *taskfakes.FakeManifestGenerator
	odbSecrets                manifestsecrets.ODBSecrets
	fakeTaskBulkSetter        *taskfakes.FakeBulkSetter
	loggerBuffer              *gbytes.Buffer
	shouldSendSigterm         bool
	secretManager             broker.ManifestSecretManager
	serviceOfferingID         string

	fakeCredhubOperator *manifestsecretsfakes.FakeCredhubOperator

	instanceLister service.InstanceLister
)

var _ = BeforeEach(func() {
	fakeBoshClient = new(fakes.FakeBoshClient)
	fakeServiceAdapter = new(fakes.FakeServiceAdapterClient)
	fakeCredentialStore = new(credhubfakes.FakeCredentialStore)
	fakeCfClient = new(fakes.FakeCloudFoundryClient)

	fakeTaskBoshClient = new(taskfakes.FakeBoshClient)
	fakeTaskManifestGenerator = new(taskfakes.FakeManifestGenerator)
	fakeTaskBulkSetter = new(taskfakes.FakeBulkSetter)
	serviceOfferingID = "simba"
	odbSecrets = manifestsecrets.ODBSecrets{ServiceOfferingID: serviceOfferingID}
	deployer = task.NewDeployer(fakeTaskBoshClient, fakeTaskManifestGenerator, odbSecrets, fakeTaskBulkSetter)

	credhubPathMatcher := new(manifestsecrets.CredHubPathMatcher)
	fakeCredhubOperator = new(manifestsecretsfakes.FakeCredhubOperator)
	secretManager = manifestsecrets.BuildManager(true, credhubPathMatcher, fakeCredhubOperator)
})

var _ = AfterEach(func() {
	if shouldSendSigterm {
		stopServer <- syscall.SIGTERM
		Eventually(loggerBuffer).Should(gbytes.Say("Server gracefully shut down"))
	}
})

func StartServer(conf config.Config) {
	stopServer = make(chan os.Signal, 1)
	conf.Broker.ShutdownTimeoutSecs = 1
	shouldSendSigterm = true
	StartServerWithStopHandler(conf, stopServer)
}

func StartServerWithStopHandler(conf config.Config, stopServerChan chan os.Signal) {
	var err error

	loggerBuffer = gbytes.NewBuffer()
	loggerFactory := loggerfactory.New(loggerBuffer, componentName, loggerfactory.Flags)
	logger := loggerFactory.New()

	instanceLister, err = service.BuildInstanceLister(fakeCfClient, conf.ServiceCatalog.ID, conf.ServiceInstancesAPI, logger)
	Expect(err).ToNot(HaveOccurred(), "unexpected error building instance lister")

	fakeOnDemandBroker, err := broker.New(
		fakeBoshClient,
		fakeCfClient,
		conf.ServiceCatalog,
		conf.Broker,
		nil,
		fakeServiceAdapter,
		deployer,
		secretManager,
		instanceLister,
		loggerFactory,
	)
	Expect(err).NotTo(HaveOccurred())
	var fakeBroker apiserver.CombinedBroker
	if conf.HasRuntimeCredHub() {
		fakeBroker = credhubbroker.New(fakeOnDemandBroker, fakeCredentialStore, conf.ServiceCatalog.Name, loggerFactory)
	} else {
		fakeBroker = fakeOnDemandBroker
	}
	server := apiserver.New(
		conf,
		fakeBroker,
		componentName,
		loggerFactory,
		logger,
	)
	go apiserver.StartAndWait(conf, server, logger, stopServerChan)
	Eventually(loggerBuffer).Should(gbytes.Say("Listening on"))
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
