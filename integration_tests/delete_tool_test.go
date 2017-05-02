// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_tests

import (
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/deleter"
	"github.com/pivotal-cf/on-demand-service-broker/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
	"gopkg.in/yaml.v2"
)

var _ = Describe("delete all service instances tool", func() {
	const (
		cfAccessToken      = "cf-oauth-token"
		instanceGUID       = "some-instance-guid"
		boundAppGUID       = "some-bound-app-guid"
		serviceBindingGUID = "some-binding-guid"
		serviceKeyGUID     = "some-key-guid"
	)

	var (
		boshDirector   *mockhttp.Server
		boshUAA        *mockuaa.ClientCredentialsServer
		cfAPI          *mockhttp.Server
		cfUAA          *mockuaa.ClientCredentialsServer
		brokerSession  *gexec.Session
		deleterSession *gexec.Session
		logBuffer      *gbytes.Buffer
		configuration  deleter.Config
		configFilePath string
	)

	BeforeEach(func() {
		boshDirector = mockbosh.New()
		boshUAA = mockuaa.NewClientCredentialsServer(boshClientID, boshClientSecret, "bosh uaa token")
		boshDirector.ExpectedAuthorizationHeader(boshUAA.ExpectedAuthorizationHeader())

		cfAPI = mockcfapi.New()
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, cfAccessToken)

		brokerConfig := defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)
		brokerSession = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)

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

		configFilePath = writeDeleteToolConfig(configYAML)
	})

	AfterEach(func() {
		killBrokerAndCheckForOpenConnections(brokerSession, boshDirector.URL)

		boshDirector.VerifyMocks()
		boshDirector.Close()
		boshUAA.Close()

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
			deleterSession, logBuffer = startDeleteTool(params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("logs appropriately", func() {
			Expect(logBuffer).To(gbytes.Say("No service instances found."))
		})

		It("exits with success", func() {
			Expect(deleterSession.ExitCode()).To(Equal(0))
		})
	})

	Context("when there are no service instances", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().
					RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").
					RespondsWithNoServiceInstances(),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession, logBuffer = startDeleteTool(params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("logs appropriately", func() {
			Expect(logBuffer).To(gbytes.Say("No service instances found."))
		})

		It("exits with success", func() {
			Expect(deleterSession.ExitCode()).To(Equal(0))
		})
	})

	Context("when there is one service instance", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().
					RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").
					RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).
					RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
				mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).RespondsNoContent(),
				mockcfapi.ListServiceKeys(instanceGUID).
					RespondsWithServiceKey(serviceKeyGUID, instanceGUID),
				mockcfapi.DeleteServiceKey(serviceKeyGUID).RespondsNoContent(),
				mockcfapi.DeleteServiceInstance(instanceGUID).RespondsAcceptedWith(""),
				mockcfapi.GetServiceInstance(instanceGUID).RespondsWithInProgress(mockcfapi.Delete),
				mockcfapi.GetServiceInstance(instanceGUID).RespondsNotFoundWith(""),
				mockcfapi.ListServiceOfferings().
					RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").
					RespondsWithNoServiceInstances(),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession, logBuffer = startDeleteTool(params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("logs appropriately", func() {
			By("logging that it is deleting the app binding")
			Expect(logBuffer).To(
				gbytes.Say("Deleting binding some-binding-guid of service instance some-instance-guid to app some-bound-app-guid"),
			)

			By("logging that it is deleting the service key")
			Expect(logBuffer).To(
				gbytes.Say(fmt.Sprintf("Deleting service key %s of service instance %s", serviceKeyGUID, instanceGUID)),
			)

			By("logging that it is deleting the instance")
			Expect(logBuffer).To(
				gbytes.Say(fmt.Sprintf("Deleting service instance %s", instanceGUID)),
			)

			By("logging that it starts waiting for the instance to be deleted")
			Expect(logBuffer).To(
				gbytes.Say(fmt.Sprintf("Waiting for service instance %s to be deleted", instanceGUID)),
			)

			By("logs that it has finished")
			Expect(logBuffer).To(
				gbytes.Say("FINISHED DELETES"),
			)
		})

		It("exits with success", func() {
			Expect(deleterSession.ExitCode()).To(BeZero())
		})
	})

	Context("when polling offset and interval are configured", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().
					RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).
					RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
				mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).RespondsNoContent(),
				mockcfapi.ListServiceKeys(instanceGUID).RespondsWithServiceKey(serviceKeyGUID, instanceGUID),
				mockcfapi.DeleteServiceKey(serviceKeyGUID).RespondsNoContent(),
				mockcfapi.DeleteServiceInstance(instanceGUID).RespondsAcceptedWith(""),
				mockcfapi.GetServiceInstance(instanceGUID).RespondsWithInProgress(mockcfapi.Delete),
			)

			configuration.PollingInitialOffset = 1
			configuration.PollingInterval = 1

			configYAML, err := yaml.Marshal(configuration)
			Expect(err).ToNot(HaveOccurred())

			configFilePath = writeDeleteToolConfig(configYAML)

			params := []string{"-configFilePath", configFilePath}
			deleterSession, logBuffer = startDeleteTool(params)

			time.Sleep(2500 * time.Millisecond)
			deleterSession.Kill()
		})

		It("polls exactly once in 2.5 seconds", func() {
			By("logging that it starts waiting for the instance to be deleted")
			Expect(logBuffer).To(
				gbytes.Say(fmt.Sprintf("Waiting for service instance %s to be deleted", instanceGUID)),
			)
		})
	})

	Context("when the configuration file cannot be read", func() {
		BeforeEach(func() {
			configFilePath := "no/file/here"
			params := []string{"-configFilePath", configFilePath}
			deleterSession, logBuffer = startDeleteTool(params)
		})

		It("logs an error", func() {
			Eventually(logBuffer).Should(gbytes.Say("Error reading config file"))
		})

		It("exits non-zero", func() {
			Eventually(deleterSession).Should(gexec.Exit(1))
		})
	})

	Context("when the configuration file is invalid yaml", func() {
		BeforeEach(func() {
			configFilePath := writeDeleteToolConfig([]byte("not:valid:yaml"))
			params := []string{"-configFilePath", configFilePath}
			deleterSession, logBuffer = startDeleteTool(params)
		})

		It("logs an error", func() {
			Eventually(logBuffer).Should(gbytes.Say("Invalid config file"))
		})

		It("exits non-zero", func() {
			Eventually(deleterSession).Should(gexec.Exit(1))
		})
	})

	Context("when CF API cannot be reached", func() {
		BeforeEach(func() {
			cfAPI.Close()

			params := []string{"-configFilePath", configFilePath}
			deleterSession, logBuffer = startDeleteTool(params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("logs appropriately", func() {
			Expect(logBuffer).To(gbytes.Say("connection refused"))
		})

		It("exits with non-zero", func() {
			Expect(deleterSession.ExitCode()).To(Equal(1))
		})
	})

	Context("when a CF API GET request fails", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsInternalServerErrorWith("no services for you"),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession, logBuffer = startDeleteTool(params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("logs appropriately", func() {
			Expect(logBuffer).To(gbytes.Say("Unexpected reponse status 500"))
			Expect(logBuffer).To(gbytes.Say("no services for you"))
		})

		It("exits with non-zero", func() {
			Expect(deleterSession.ExitCode()).To(Equal(1))
		})
	})

	Context("when CF API DELETE request responds with 404 Not Found", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().
					RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").
					RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).
					RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
				mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).
					RespondsNotFoundWith(`{
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
				mockcfapi.ListServiceOfferings().
					RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").
					RespondsWithNoServiceInstances(),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession, logBuffer = startDeleteTool(params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("exits with success", func() {
			Expect(deleterSession.ExitCode()).To(BeZero())
		})
	})

	Context("when CF API GET service bindings responds with 404 Not Found", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().
					RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").
					RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).
					RespondsNotFoundWith(`{
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
				mockcfapi.ListServiceOfferings().
					RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").
					RespondsWithNoServiceInstances(),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession, logBuffer = startDeleteTool(params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("exits with success", func() {
			Expect(deleterSession.ExitCode()).To(BeZero())
		})
	})

	Context("when CF API GET service keys responds with 404 Not Found", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().
					RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").
					RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).
					RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
				mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).RespondsNoContent(),
				mockcfapi.ListServiceKeys(instanceGUID).
					RespondsNotFoundWith(`{
							"code": 111111,
							"description": "The app could not be found: some-bound-app-guid",
							"error_code": "CF-AppNotFound"
						}`),
				mockcfapi.DeleteServiceInstance(instanceGUID).RespondsAcceptedWith(""),
				mockcfapi.GetServiceInstance(instanceGUID).RespondsNotFoundWith(""),
				mockcfapi.ListServiceOfferings().
					RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").
					RespondsWithNoServiceInstances(),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession, logBuffer = startDeleteTool(params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("exits with success", func() {
			Expect(deleterSession.ExitCode()).To(BeZero())
		})
	})

	Context("when a CF API DELETE response is unexpected", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().
					RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").
					RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).
					RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
				mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).
					RespondsForbiddenWith(`{
						"code": 10003,
						"description": "You are not authorized to perform the requested action",
						"error_code": "CF-NotAuthorized"
					}`),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession, logBuffer = startDeleteTool(params)
			Eventually(deleterSession, 10*time.Second).Should(gexec.Exit())
		})

		It("logs the status code and reason", func() {
			Expect(logBuffer).To(gbytes.Say("Unexpected reponse status 403"))
			Expect(logBuffer).To(gbytes.Say("You are not authorized to perform the requested action"))
		})

		It("exits non-zero", func() {
			Eventually(deleterSession).Should(gexec.Exit(1))
		})
	})

	Context("when CF API GET instance response is delete failed", func() {

		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().
					RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").
					RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).
					RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
				mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).RespondsNoContent(),
				mockcfapi.ListServiceKeys(instanceGUID).
					RespondsWithServiceKey(serviceKeyGUID, instanceGUID),
				mockcfapi.DeleteServiceKey(serviceKeyGUID).RespondsNoContent(),
				mockcfapi.DeleteServiceInstance(instanceGUID).RespondsAcceptedWith(""),
				mockcfapi.GetServiceInstance(instanceGUID).RespondsWithFailed(mockcfapi.Delete),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession, logBuffer = startDeleteTool(params)
			Eventually(deleterSession, 1*time.Second).Should(gexec.Exit())
		})

		It("logs that the delete operation failed", func() {
			Expect(logBuffer).To(gbytes.Say("Result: failed to delete service instance %s. Delete operation failed.", instanceGUID))
		})

		It("exits non-zero", func() {
			Eventually(deleterSession).Should(gexec.Exit(1))
		})
	})

	Context("when CF API GET instance response is invalid json", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().
					RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").
					RespondsWithServiceInstances(instanceGUID),
				mockcfapi.ListServiceBindings(instanceGUID).
					RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
				mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).RespondsNoContent(),
				mockcfapi.ListServiceKeys(instanceGUID).
					RespondsWithServiceKey(serviceKeyGUID, instanceGUID),
				mockcfapi.DeleteServiceKey(serviceKeyGUID).RespondsNoContent(),
				mockcfapi.DeleteServiceInstance(instanceGUID).RespondsAcceptedWith(""),
				mockcfapi.GetServiceInstance(instanceGUID).RespondsOKWith("not valid json"),
			)

			params := []string{"-configFilePath", configFilePath}
			deleterSession, logBuffer = startDeleteTool(params)
			Eventually(deleterSession, 1*time.Second).Should(gexec.Exit())
		})

		It("logs that the delete operation failed", func() {
			Expect(logBuffer).To(gbytes.Say("Result: failed to delete service instance %s. Error: Invalid response body", instanceGUID))
		})

		It("exits non-zero", func() {
			Eventually(deleterSession).Should(gexec.Exit(1))
		})
	})
})

func startDeleteTool(params []string) (*gexec.Session, *gbytes.Buffer) {
	cmd := exec.Command(deleteToolPath, params...)
	logBuffer := gbytes.NewBuffer()

	session, err := gexec.Start(cmd, io.MultiWriter(GinkgoWriter, logBuffer), GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session, logBuffer
}

func writeDeleteToolConfig(config []byte) string {
	configFilePath := filepath.Join(tempDirPath, "delete_all_test_config.yml")
	Expect(ioutil.WriteFile(configFilePath, config, 0644)).To(Succeed())
	return configFilePath
}
