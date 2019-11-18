// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package helpers

import (
	"crypto/tls"
	"fmt"
	"github.com/pivotal-cf/on-demand-service-broker/broker/decider"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/pivotal-cf/on-demand-service-broker/manifestsecrets"

	"github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	credhubfakes "github.com/pivotal-cf/on-demand-service-broker/credhubbroker/fakes"
	manifestsecretsfakes "github.com/pivotal-cf/on-demand-service-broker/manifestsecrets/fakes"
	odbserviceadapter "github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	serviceadapterfakes "github.com/pivotal-cf/on-demand-service-broker/serviceadapter/fakes"
	taskfakes "github.com/pivotal-cf/on-demand-service-broker/task/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"

	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/task"

	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

type Server struct {
	stopServerChan chan os.Signal
	loggerBuffer   *gbytes.Buffer
}

func StartServer(
	conf config.Config,
	stopServerChan chan os.Signal,
	fakeCommandRunner *serviceadapterfakes.FakeCommandRunner,
	fakeTaskBoshClient *taskfakes.FakeBoshClient,
	fakeTaskBulkSetter *taskfakes.FakeBulkSetter,
	fakeCfClient *fakes.FakeCloudFoundryClient,
	fakeBoshClient *fakes.FakeBoshClient,
	fakeMapHasher *fakes.FakeHasher,
	maintenanceInfoChecker *fakes.FakeMaintenanceInfoChecker,
	fakeCredentialStore *credhubfakes.FakeCredentialStore,
	fakeCredhubOperator *manifestsecretsfakes.FakeCredhubOperator,
	loggerBuffer *gbytes.Buffer,
) (*Server, error) {
	var err error

	if conf.Broker.ShutdownTimeoutSecs == 0 {
		conf.Broker.ShutdownTimeoutSecs = 2
	}

	serviceAdapterClient := &odbserviceadapter.Client{
		CommandRunner: fakeCommandRunner,
		UsingStdin:    true,
	}

	taskManifestGenerator := task.NewManifestGenerator(serviceAdapterClient, conf.ServiceCatalog, []serviceadapter.Stemcell{}, serviceadapter.ServiceReleases{})
	odbSecrets := manifestsecrets.ODBSecrets{ServiceOfferingID: conf.ServiceCatalog.ID}

	deployer := task.NewDeployer(fakeTaskBoshClient, taskManifestGenerator, odbSecrets, fakeTaskBulkSetter)
	deployer.DisableBoshConfigs = conf.Broker.DisableBoshConfigs

	loggerFactory := loggerfactory.New(loggerBuffer, "collaboration-tests", loggerfactory.Flags)
	logger := loggerFactory.New()

	instanceLister, err := service.BuildInstanceLister(fakeCfClient, conf.ServiceCatalog.ID, conf.ServiceInstancesAPI, logger)
	Expect(err).ToNot(HaveOccurred(), "unexpected error building instance lister")

	credhubPathMatcher := new(manifestsecrets.CredHubPathMatcher)
	secretManager := manifestsecrets.BuildManager(true, credhubPathMatcher, fakeCredhubOperator)

	fakeOnDemandBroker, err := broker.New(
		fakeBoshClient,
		fakeCfClient,
		conf.ServiceCatalog,
		conf.Broker,
		nil,
		serviceAdapterClient,
		deployer,
		secretManager,
		instanceLister,
		fakeMapHasher,
		loggerFactory,
		new(fakes.FakeTelemetryLogger),
		maintenanceInfoChecker,
		decider.Decider{},
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
		"collaboration-tests",
		loggerFactory,
		logger,
	)

	server.ErrorLog = logger

	errChan := make(chan error)
	upChan := make(chan struct{})

	go func() {
		errChan <- apiserver.StartAndWait(conf, server, logger, stopServerChan)
	}()

	go func() {
		for {
			if CanServeHTTP(server.Addr, conf) {
				close(upChan)
				break
			}
			time.Sleep(time.Millisecond * 50)
		}
	}()

	var serverError error

	select {
	case <-upChan:
	case serverError = <-errChan:
	case <-time.After(time.Second * 2):
		Fail("HTTPS server failed to serve https within timeout")
	}

	return &Server{stopServerChan: stopServerChan, loggerBuffer: loggerBuffer}, serverError
}

func CanServeHTTP(serverAddr string, conf config.Config) bool {
	httpSuffix := ""
	if conf.HasTLS() {
		httpSuffix = "s"
	}
	url := fmt.Sprintf("http%s://%s", httpSuffix, serverAddr)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}
	_, err := client.Get(url)
	return err == nil
}

func (s *Server) Close() {
	s.stopServerChan <- syscall.SIGTERM
	Eventually(s.loggerBuffer).Should(gbytes.Say("Server gracefully shut down"))
}
