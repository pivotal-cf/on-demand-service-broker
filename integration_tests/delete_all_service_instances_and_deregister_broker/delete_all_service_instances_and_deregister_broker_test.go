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

package delete_all_service_instances_and_deregister_broker_test

import (
	"gopkg.in/yaml.v2"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/deleter"
	"github.com/pivotal-cf/on-demand-service-broker/integration_tests/helpers"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
)

var _ = Describe("purge instances and deregister tool", func() {
	const (
		serviceOfferingName = "service-id"
		serviceOfferingGUID = "some-cc-service-offering-guid"

		planID   = "plan-id"
		planGUID = "some-cc-plan-guid"

		cfAccessToken      = "cf-oauth-token"
		cfUaaClientID      = "cf-uaa-client-id"
		cfUaaClientSecret  = "cf-uaa-client-secret"
		instanceGUID       = "some-instance-guid"
		boundAppGUID       = "some-bound-app-guid"
		serviceBindingGUID = "some-binding-guid"
		serviceKeyGUID     = "some-key-guid"

		serviceBrokerGUID = "some-service-broker-guid"
		serviceBrokerName = "some-broker-name"

		timeout = time.Second * 5
	)

	var (
		cfAPI          *mockhttp.Server
		cfUAA          *mockuaa.ClientCredentialsServer
		purgerSession  *gexec.Session
		configuration  deleter.Config
		configFilePath string
	)

	BeforeEach(func() {
		cfAPI = mockcfapi.New()
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, cfAccessToken)

		configuration = deleter.Config{
			ServiceCatalog: deleter.ServiceCatalog{
				ID: serviceOfferingName,
			},
			DisableSSLCertVerification: true,
			CF: config.CF{
				URL: cfAPI.URL,
				Authentication: config.Authentication{
					UAA: config.UAAAuthentication{
						URL: cfUAA.URL,
						ClientCredentials: config.ClientCredentials{
							ID:     cfUaaClientID,
							Secret: cfUaaClientSecret,
						},
					},
				},
			},
			PollingInitialOffset: 0,
			PollingInterval:      0,
		}

		configYAML, err := yaml.Marshal(configuration)
		Expect(err).ToNot(HaveOccurred())

		configFilePath = helpers.WriteConfig(configYAML, tempDir)
	})

	AfterEach(func() {
		cfAPI.VerifyMocks()
		cfAPI.Close()
		cfUAA.Close()
	})

	It("deletes the service instance and deregisters the service broker", func() {
		cfAPI.VerifyAndMock(
			//Step 1 of the purger, Disabling service access
			mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceOfferingName, serviceOfferingGUID),
			mockcfapi.ListServicePlans(serviceOfferingGUID).RespondsWithServicePlan(planID, planGUID),
			mockcfapi.DisablePlanAccess(planGUID).RespondsCreated(),
			//Step 2 of the purger, deleting all service instances
			mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceOfferingName, serviceOfferingGUID),
			mockcfapi.ListServicePlans(serviceOfferingGUID).RespondsWithServicePlan(planID, planGUID),
			mockcfapi.ListServiceInstances(planGUID).RespondsWithServiceInstances(instanceGUID),
			mockcfapi.ListServiceBindings(instanceGUID).RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
			mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).RespondsNoContent(),
			mockcfapi.ListServiceKeys(instanceGUID).RespondsWithServiceKey(serviceKeyGUID, instanceGUID),
			mockcfapi.DeleteServiceKey(serviceKeyGUID).RespondsNoContent(),
			mockcfapi.DeleteServiceInstance(instanceGUID).RespondsAcceptedWith(""),
			mockcfapi.GetServiceInstance(instanceGUID).RespondsWithInProgress(mockcfapi.Delete),
			mockcfapi.GetServiceInstance(instanceGUID).RespondsNotFoundWith(""),
			mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceOfferingName, serviceOfferingGUID),
			mockcfapi.ListServicePlans(serviceOfferingGUID).RespondsWithServicePlan(planID, planGUID),
			mockcfapi.ListServiceInstances(planGUID).RespondsWithNoServiceInstances(),
			//Step 3 of the purger, deregistering the broker
			mockcfapi.ListServiceBrokers().RespondsWithBrokers(serviceBrokerName, serviceBrokerGUID),
			mockcfapi.DeregisterBroker(serviceBrokerGUID).RespondsNoContent(),
		)

		params := []string{"-configFilePath", configFilePath, "-brokerName", serviceBrokerName}
		purgerSession = helpers.StartBinaryWithParams(binaryPath, params)
		Eventually(purgerSession, timeout).Should(gexec.Exit(0))

		Expect(purgerSession).To(gbytes.Say("FINISHED PURGE INSTANCES AND DEREGISTER BROKER"))
	})

	It("fails when the purger fails", func() {
		cfAPI.VerifyAndMock(
			mockcfapi.ListServiceOfferings().RespondsInternalServerErrorWith("failed"),
		)

		params := []string{"-configFilePath", configFilePath, "-brokerName", serviceBrokerName}
		purgerSession = helpers.StartBinaryWithParams(binaryPath, params)
		Eventually(purgerSession, timeout).Should(gexec.Exit(1))
		Eventually(purgerSession).Should(gbytes.Say("Purger Failed:"))

	})

	It("fails when broker name is not provided", func() {
		params := []string{"-configFilePath", configFilePath}
		purgerSession = helpers.StartBinaryWithParams(binaryPath, params)

		Eventually(purgerSession, timeout).Should(gexec.Exit(1))
		Eventually(purgerSession).Should(gbytes.Say("Missing argument -brokerName"))
	})

	It("fails when config file path is not provided", func() {
		params := []string{"-brokerName", serviceBrokerName}
		purgerSession = helpers.StartBinaryWithParams(binaryPath, params)

		Eventually(purgerSession, timeout).Should(gexec.Exit(1))
		Eventually(purgerSession).Should(gbytes.Say("Missing argument -configFilePath"))
	})

	It("fails when configFilePath cannot be read", func() {
		params := []string{"-configFilePath", "/tmp/foo/bar", "-brokerName", serviceBrokerName}
		purgerSession = helpers.StartBinaryWithParams(binaryPath, params)

		Eventually(purgerSession, timeout).Should(gexec.Exit(1))
		Eventually(purgerSession).Should(gbytes.Say("Error reading config file:"))
	})

	It("fails when the config is not valid yaml", func() {
		configFilePath := helpers.WriteConfig([]byte("not valid yaml"), tempDir)
		params := []string{"-configFilePath", configFilePath, "-brokerName", serviceBrokerName}
		purgerSession = helpers.StartBinaryWithParams(binaryPath, params)

		Eventually(purgerSession, timeout).Should(gexec.Exit(1))
		Eventually(purgerSession).Should(gbytes.Say("Invalid config file:"))
	})

})
