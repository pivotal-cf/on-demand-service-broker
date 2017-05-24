// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package delete_all_service_instances_test

import (
	"fmt"
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
	"gopkg.in/yaml.v2"
)

var _ = Describe("delete all service instances tool", func() {
	const (
		serviceID          = "service-id"
		planID             = "plan-id"
		cfAccessToken      = "cf-oauth-token"
		cfUaaClientID      = "cf-uaa-client-id"
		cfUaaClientSecret  = "cf-uaa-client-secret"
		instanceGUID       = "some-instance-guid"
		boundAppGUID       = "some-bound-app-guid"
		serviceBindingGUID = "some-binding-guid"
		serviceKeyGUID     = "some-key-guid"
	)

	var (
		cfAPI          *mockhttp.Server
		cfUAA          *mockuaa.ClientCredentialsServer
		deleterSession *gexec.Session
		configuration  deleter.Config
		configFilePath string
	)

	BeforeEach(func() {
		cfAPI = mockcfapi.New()
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, cfAccessToken)

		configuration = deleter.Config{
			ServiceCatalog: deleter.ServiceCatalog{
				ID: serviceID,
			},
			DisableSSLCertVerification: true,
			CF: config.CF{
				URL: cfAPI.URL,
				Authentication: config.UAAAuthentication{
					URL: cfUAA.URL,
					ClientCredentials: config.ClientCredentials{
						ID:     cfUaaClientID,
						Secret: cfUaaClientSecret,
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

	Context("when the service is not registered with CF", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().
					RespondsOKWith(`{
						"total_results": 0,
						"total_pages": 0,
						"prev_url": null,
						"next_url": null,
						"resources": []
					}`),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession = helpers.StartBinaryWithParams(binaryPath, params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("does nothing", func() {
			Expect(deleterSession.ExitCode()).To(Equal(0))
			Expect(deleterSession).To(gbytes.Say("No service instances found."))
		})
	})

	Context("when there are no service instances", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(planID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithNoServiceInstances(),
			)

			configuration.PollingInitialOffset = 1
			configuration.PollingInterval = 1

			configYAML, err := yaml.Marshal(configuration)
			Expect(err).ToNot(HaveOccurred())

			configFilePath = helpers.WriteConfig(configYAML, tempDir)

			params := []string{"-configFilePath", configFilePath}
			deleterSession = helpers.StartBinaryWithParams(binaryPath, params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("does nothing", func() {
			Expect(deleterSession.ExitCode()).To(Equal(0))
			Expect(deleterSession).To(gbytes.Say("No service instances found."))
		})

		It("logs that the polling interval values are as configured", func() {
			Expect(deleterSession).To(gbytes.Say("Deleter Configuration: polling_intial_offset: 1, polling_interval: 1."))
		})
	})

	Context("when there is one service instance", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(planID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
				mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).RespondsNoContent(),
				mockcfapi.ListServiceKeys(instanceGUID).RespondsWithServiceKey(serviceKeyGUID, instanceGUID),
				mockcfapi.DeleteServiceKey(serviceKeyGUID).RespondsNoContent(),
				mockcfapi.DeleteServiceInstance(instanceGUID).RespondsAcceptedWith(""),
				mockcfapi.GetServiceInstance(instanceGUID).RespondsWithInProgress(mockcfapi.Delete),
				mockcfapi.GetServiceInstance(instanceGUID).RespondsNotFoundWith(""),
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(planID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithNoServiceInstances(),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession = helpers.StartBinaryWithParams(binaryPath, params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("deletes the service instance", func() {
			Expect(deleterSession.ExitCode()).To(BeZero())

			Expect(deleterSession).To(gbytes.Say("Deleting binding some-binding-guid of service instance some-instance-guid to app some-bound-app-guid"))
			Expect(deleterSession).To(gbytes.Say(fmt.Sprintf("Deleting service key %s of service instance %s", serviceKeyGUID, instanceGUID)))
			Expect(deleterSession).To(gbytes.Say(fmt.Sprintf("Deleting service instance %s", instanceGUID)))
			Expect(deleterSession).To(gbytes.Say(fmt.Sprintf("Waiting for service instance %s to be deleted", instanceGUID)))
			Expect(deleterSession).To(gbytes.Say("FINISHED DELETES"))
		})
	})

	Context("when the configuration file cannot be read", func() {
		BeforeEach(func() {
			configFilePath := "no/file/here"
			params := []string{"-configFilePath", configFilePath}
			deleterSession = helpers.StartBinaryWithParams(binaryPath, params)
		})

		It("fails with error", func() {
			Eventually(deleterSession).Should(gexec.Exit(1))
			Eventually(deleterSession).Should(gbytes.Say("Error reading config file"))
		})
	})

	Context("when the configuration file is invalid yaml", func() {
		BeforeEach(func() {
			configFilePath := helpers.WriteConfig([]byte("not:valid:yaml"), tempDir)
			params := []string{"-configFilePath", configFilePath}
			deleterSession = helpers.StartBinaryWithParams(binaryPath, params)
		})

		It("fails with error", func() {
			Eventually(deleterSession).Should(gexec.Exit(1))
			Eventually(deleterSession).Should(gbytes.Say("Invalid config file"))
		})
	})

	Context("when CF API cannot be reached", func() {
		BeforeEach(func() {
			cfAPI.Close()

			params := []string{"-configFilePath", configFilePath}
			deleterSession = helpers.StartBinaryWithParams(binaryPath, params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("fails to connect", func() {
			Expect(deleterSession.ExitCode()).To(Equal(1))
			Expect(deleterSession).To(gbytes.Say("connection refused"))
		})
	})

	Context("when a CF API GET request fails", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsInternalServerErrorWith("no services for you"),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession = helpers.StartBinaryWithParams(binaryPath, params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("fails and does nothing", func() {
			Expect(deleterSession.ExitCode()).To(Equal(1))
			Expect(deleterSession).To(gbytes.Say("Unexpected reponse status 500"))
			Expect(deleterSession).To(gbytes.Say("no services for you"))
		})
	})

	Context("when CF API DELETE request responds with 404 Not Found", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(planID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
				mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).RespondsNotFoundWith(`{
							"code": 111111,
							"description": "The app could not be found: some-bound-app-guid",
							"error_code": "CF-AppNotFound"
						}`),
				mockcfapi.ListServiceKeys(instanceGUID).RespondsOKWith(`{
						"total_results": 0,
						"total_pages": 0,
						"prev_url": null,
						"next_url": null,
						"resources": []
					}`),
				mockcfapi.DeleteServiceInstance(instanceGUID).RespondsAcceptedWith(""),
				mockcfapi.GetServiceInstance(instanceGUID).RespondsNotFoundWith(""),
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(planID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithNoServiceInstances(),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession = helpers.StartBinaryWithParams(binaryPath, params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("exits with success", func() {
			Expect(deleterSession.ExitCode()).To(BeZero())
		})
	})

	Context("when CF API GET service bindings responds with 404 Not Found", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(planID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).RespondsNotFoundWith(`{
							"code": 111111,
							"description": "The app could not be found: some-bound-app-guid",
							"error_code": "CF-AppNotFound"
						}`),
				mockcfapi.ListServiceKeys(instanceGUID).RespondsOKWith(`{
						"total_results": 0,
						"total_pages": 0,
						"prev_url": null,
						"next_url": null,
						"resources": []
					}`),
				mockcfapi.DeleteServiceInstance(instanceGUID).RespondsAcceptedWith(""),
				mockcfapi.GetServiceInstance(instanceGUID).RespondsNotFoundWith(""),
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(planID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithNoServiceInstances(),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession = helpers.StartBinaryWithParams(binaryPath, params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("exits with success", func() {
			Expect(deleterSession.ExitCode()).To(BeZero())
		})
	})

	Context("when CF API GET service keys responds with 404 Not Found", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(planID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
				mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).RespondsNoContent(),
				mockcfapi.ListServiceKeys(instanceGUID).RespondsNotFoundWith(`{
							"code": 111111,
							"description": "The app could not be found: some-bound-app-guid",
							"error_code": "CF-AppNotFound"
						}`),
				mockcfapi.DeleteServiceInstance(instanceGUID).RespondsAcceptedWith(""),
				mockcfapi.GetServiceInstance(instanceGUID).RespondsNotFoundWith(""),
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(planID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithNoServiceInstances(),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession = helpers.StartBinaryWithParams(binaryPath, params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("exits with success", func() {
			Expect(deleterSession.ExitCode()).To(BeZero())
		})
	})

	Context("when a CF API DELETE response is unexpected", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(planID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
				mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).RespondsForbiddenWith(`{
						"code": 10003,
						"description": "You are not authorized to perform the requested action",
						"error_code": "CF-NotAuthorized"
					}`),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession = helpers.StartBinaryWithParams(binaryPath, params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("fails to authorize", func() {
			Eventually(deleterSession).Should(gexec.Exit(1))

			Expect(deleterSession).To(gbytes.Say("Unexpected reponse status 403"))
			Expect(deleterSession).To(gbytes.Say("You are not authorized to perform the requested action"))
		})
	})

	Context("when CF API GET instance response is delete failed", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(planID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
				mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).RespondsNoContent(),
				mockcfapi.ListServiceKeys(instanceGUID).RespondsWithServiceKey(serviceKeyGUID, instanceGUID),
				mockcfapi.DeleteServiceKey(serviceKeyGUID).RespondsNoContent(),
				mockcfapi.DeleteServiceInstance(instanceGUID).RespondsAcceptedWith(""),
				mockcfapi.GetServiceInstance(instanceGUID).RespondsWithFailed(mockcfapi.Delete),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession = helpers.StartBinaryWithParams(binaryPath, params)
			Eventually(deleterSession, 1*time.Second).Should(gexec.Exit())
		})

		It("reports the failure", func() {
			Eventually(deleterSession).Should(gexec.Exit(1))
			Expect(deleterSession).To(gbytes.Say("Result: failed to delete service instance %s. Delete operation failed.", instanceGUID))
		})
	})

	Context("when CF API GET instance response is invalid json", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(planID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
				mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).RespondsNoContent(),
				mockcfapi.ListServiceKeys(instanceGUID).RespondsWithServiceKey(serviceKeyGUID, instanceGUID),
				mockcfapi.DeleteServiceKey(serviceKeyGUID).RespondsNoContent(),
				mockcfapi.DeleteServiceInstance(instanceGUID).RespondsAcceptedWith(""),
				mockcfapi.GetServiceInstance(instanceGUID).RespondsOKWith("not valid json"),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession = helpers.StartBinaryWithParams(binaryPath, params)
			Eventually(deleterSession, 1*time.Second).Should(gexec.Exit())
		})

		It("fails to delete", func() {
			Eventually(deleterSession).Should(gexec.Exit(1))
			Expect(deleterSession).To(gbytes.Say("Result: failed to delete service instance %s. Error: Invalid response body", instanceGUID))
		})
	})
})
