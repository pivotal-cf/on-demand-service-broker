// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

var _ = Describe("binding service instances", func() {
	var (
		boshDirector *mockbosh.MockBOSH
		cfAPI        *mockhttp.Server
		boshUAA      *mockuaa.ClientCredentialsServer
		cfUAA        *mockuaa.ClientCredentialsServer

		runningBroker   *gexec.Session
		bindingResponse *http.Response

		instanceID                 = "some-binding-instance-ID"
		manifestForFirstDeployment = bosh.BoshManifest{
			Name:           deploymentName(instanceID),
			Releases:       []bosh.Release{},
			Stemcells:      []bosh.Stemcell{},
			InstanceGroups: []bosh.InstanceGroup{},
		}
	)

	JustBeforeEach(func() {
		boshUAA = mockuaa.NewClientCredentialsServer(boshClientID, boshClientSecret, "bosh uaa token")
		boshDirector = mockbosh.NewWithUAA(boshUAA.URL)
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")
		cfAPI = mockcfapi.New()

		var brokerConfig config.Config
		brokerConfig = defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)

		runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)
	})

	AfterEach(func() {
		killBrokerAndCheckForOpenConnections(runningBroker, boshDirector.URL)

		boshDirector.VerifyMocks()
		boshDirector.Close()
		boshUAA.Close()

		cfAPI.VerifyMocks()
		cfAPI.Close()
		cfUAA.Close()
	})

	Describe("a successful binding", func() {
		var (
			bindingReq      *http.Request
			bindingResponse *http.Response
			bindingParams   = map[string]interface{}{"baz": "bar"}

			bindingPlanID    = "plan-guid-from-cc"
			bindingServiceID = "service-guid-from-cc"
			bindingId        = "Gjklh45ljkhn"
			appGUID          = "app-guid-from-cc"
		)

		BeforeEach(func() {
			adapter.CreateBinding().ReturnsBinding(`{
					"credentials": {"secret": "dont-tell-anyone"},
					"syslog_drain_url": "syslog-url",
					"route_service_url": "excellent route"
					}`)
		})

		JustBeforeEach(func() {
			boshDirector.VerifyAndMock(
				mockbosh.VMsForDeployment(deploymentName(instanceID)).RedirectsToTask(2015),
				mockbosh.Task(2015).RespondsWithTaskContainingState(boshdirector.TaskDone),
				mockbosh.TaskOutput(2015).RespondsWithVMsOutput([]boshdirector.BoshVMsOutput{{IPs: []string{"ip.from.bosh"}, InstanceGroup: "some-instance-group"}}),
				mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifestForFirstDeployment),
			)
			reqBody := map[string]interface{}{
				"plan_id":    bindingPlanID,
				"service_id": bindingServiceID,
				"app_guid":   appGUID,
				"bind_resource": map[string]interface{}{
					"app_guid": appGUID,
				},
				"parameters": bindingParams,
			}
			bodyBytes, err := json.Marshal(reqBody)
			Expect(err).ToNot(HaveOccurred())

			bindingReq, err = http.NewRequest("PUT",
				fmt.Sprintf("http://localhost:%d/v2/service_instances/%s/service_bindings/%s", brokerPort, instanceID, bindingId),
				bytes.NewReader(bodyBytes))
			Expect(err).ToNot(HaveOccurred())
			bindingReq = basicAuthBrokerRequest(bindingReq)

			bindingResponse, err = http.DefaultClient.Do(bindingReq)
			Expect(err).ToNot(HaveOccurred())
		})

		It("exhibits success", func() {
			By("responding with HTTP 201")

			Expect(bindingResponse.StatusCode).To(Equal(http.StatusCreated))

			By("including credentials, syslog drain URL and route service URL in response body")

			var binding brokerapi.Binding
			defer bindingResponse.Body.Close()
			Expect(json.NewDecoder(bindingResponse.Body).Decode(&binding)).To(Succeed())

			credentials := binding.Credentials.(map[string]interface{})
			Expect(credentials).To(Equal(map[string]interface{}{"secret": "dont-tell-anyone"}))
			Expect(binding.RouteServiceURL).To(Equal("excellent route"))
			Expect(binding.SyslogDrainURL).To(Equal("syslog-url"))

			By("calling the adapter with expected binding ID")

			Expect(adapter.CreateBinding().ReceivedID()).To(Equal("Gjklh45ljkhn"))

			By("calling the adapter with expected bosh VMS")

			Expect(adapter.CreateBinding().ReceivedBoshVms()).To(Equal(bosh.BoshVMs{"some-instance-group": []string{"ip.from.bosh"}}))

			By("calling the adapter with the correct request params")

			Expect(adapter.CreateBinding().ReceivedRequestParameters()).To(Equal(map[string]interface{}{
				"plan_id":    bindingPlanID,
				"service_id": bindingServiceID,
				"app_guid":   appGUID,
				"bind_resource": map[string]interface{}{
					"app_guid": appGUID,
				},
				"parameters": bindingParams,
			}))

			By("calling the adapter with the bosh manifest")

			Expect(adapter.CreateBinding().ReceivedManifest()).To(Equal(manifestForFirstDeployment))

			By("logging the bind request with a request id")
			bindRequestRegex := logRegexpStringWithRequestIDCapture(`service adapter will create binding with ID`)
			Eventually(runningBroker).Should(gbytes.Say(bindRequestRegex))
			requestID := firstMatchInOutput(runningBroker, bindRequestRegex)
			Eventually(runningBroker).Should(gbytes.Say(requestID)) // It should use the same request ID again
		})

		Context("when the service adapter returns no syslog drain url and no route service url", func() {
			BeforeEach(func() {
				adapter.CreateBinding().ReturnsBinding(`{
									"credentials": {"secret": "dont-tell-anyone"}
								}`)
			})

			It("responds with 201 but doesn't include JSON keys for any missing optional fields", func() {
				Expect(bindingResponse.StatusCode).To(Equal(http.StatusCreated))

				defer bindingResponse.Body.Close()
				bodyBytes, err := ioutil.ReadAll(bindingResponse.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(bodyBytes).NotTo(ContainSubstring("syslog_drain_url"))
				Expect(bodyBytes).NotTo(ContainSubstring("route_service_url"))
			})
		})
	})

	Context("when the binding fails due to an adapter error", func() {
		var bindingResponse *http.Response

		JustBeforeEach(func() {
			boshDirector.VerifyAndMock(
				mockbosh.VMsForDeployment(deploymentName(instanceID)).RedirectsToTask(2015),
				mockbosh.Task(2015).RespondsWithTaskContainingState(boshdirector.TaskDone),
				mockbosh.TaskOutput(2015).RespondsWithVMsOutput([]boshdirector.BoshVMsOutput{{IPs: []string{"ip.from.bosh"}, InstanceGroup: "some-instance-group"}}),
				mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifestForFirstDeployment),
			)
			bindingReq, err := http.NewRequest("PUT",
				fmt.Sprintf("http://localhost:%d/v2/service_instances/%s/service_bindings/Gjklh45ljkhn", brokerPort, instanceID),
				strings.NewReader("{}"))
			Expect(err).ToNot(HaveOccurred())
			bindingReq = basicAuthBrokerRequest(bindingReq)

			bindingResponse, err = http.DefaultClient.Do(bindingReq)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("with code 49", func() {
			Context("but without a stderr message", func() {
				BeforeEach(func() {
					adapter.CreateBinding().FailsWithBindingAlreadyExistsError()
				})

				It("responds with 409 and a generic error message", func() {
					Expect(bindingResponse.StatusCode).To(Equal(409))

					defer bindingResponse.Body.Close()
					Expect(ioutil.ReadAll(bindingResponse.Body)).To(MatchJSON(`{"description":"binding already exists"}`))
				})
			})

			Context("with a stderr error message", func() {
				BeforeEach(func() {
					adapter.CreateBinding().FailsWithBindingAlreadyExistsErrorAndStderr("stderr error message")
				})

				It("responds with 409 and an appropriate message, and logs", func() {
					Expect(bindingResponse.StatusCode).To(Equal(409))

					defer bindingResponse.Body.Close()
					Expect(ioutil.ReadAll(bindingResponse.Body)).To(MatchJSON(`{"description":"binding already exists"}`))

					Eventually(runningBroker.Out).Should(gbytes.Say(`stderr error message`))
				})
			})
		})

		Context("with code 42", func() {
			Context("but without a stderr message", func() {
				BeforeEach(func() {
					adapter.CreateBinding().FailsWithAppGuidNotProvidedError()
				})

				It("responds with 422 and a generic error message", func() {
					Expect(bindingResponse.StatusCode).To(Equal(422))

					defer bindingResponse.Body.Close()
					Expect(ioutil.ReadAll(bindingResponse.Body)).To(MatchJSON(`{"description":"app_guid is a required field but was not provided"}`))
				})
			})

			Context("with stderr error messages", func() {
				BeforeEach(func() {
					adapter.CreateBinding().FailsWithAppGuidNotProvidedErrorAndStderr("stderr error message")
				})

				It("responds with 422 and an appropriate message, and logs", func() {
					Expect(bindingResponse.StatusCode).To(Equal(422))

					defer bindingResponse.Body.Close()
					Expect(ioutil.ReadAll(bindingResponse.Body)).To(MatchJSON(`{"description":"app_guid is a required field but was not provided"}`))

					Eventually(runningBroker.Out).Should(gbytes.Say(`stderr error message`))
				})
			})
		})

		Context("when the adapter does not implement binder, code 10", func() {
			var errorResponse brokerapi.ErrorResponse

			BeforeEach(func() {
				adapter.Binder().NotImplemented()
			})

			JustBeforeEach(func() {
				Expect(json.NewDecoder(bindingResponse.Body).Decode(&errorResponse)).To(Succeed())
			})

			AfterEach(func() {
				defer bindingResponse.Body.Close()
			})

			It("responds with a 500 error", func() {
				Expect(bindingResponse.StatusCode).To(Equal(http.StatusInternalServerError))
				Expect(errorResponse.Description).To(ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information: ",
				))
				Expect(errorResponse.Description).To(MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				))
				Expect(errorResponse.Description).To(ContainSubstring(
					fmt.Sprintf("service: %s", serviceName),
				))
				Expect(errorResponse.Description).To(ContainSubstring(
					fmt.Sprintf("service-instance-guid: %s", instanceID),
				))
				Expect(errorResponse.Description).To(ContainSubstring("operation: bind"))
				Expect(errorResponse.Description).NotTo(ContainSubstring("task-id:"))
				Eventually(runningBroker).Should(gbytes.Say(
					"creating binding: command not implemented",
				))
			})
		})

		Context("when binding fails due to unknown adapter error", func() {
			Context("when there is operator error message and no user error message", func() {
				var errorResponse brokerapi.ErrorResponse

				BeforeEach(func() {
					adapter.CreateBinding().FailsWithOperatorError("adapter completely failed")
				})

				JustBeforeEach(func() {
					Expect(json.NewDecoder(bindingResponse.Body).Decode(&errorResponse)).To(Succeed())
				})

				AfterEach(func() {
					defer bindingResponse.Body.Close()
				})

				It("responds with a 500 error", func() {
					Expect(bindingResponse.StatusCode).To(Equal(500))
					Expect(errorResponse.Description).To(ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information: ",
					))
					Expect(errorResponse.Description).To(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					))
					Expect(errorResponse.Description).To(ContainSubstring(
						fmt.Sprintf("service: %s", serviceName),
					))
					Expect(errorResponse.Description).To(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					))
					Expect(errorResponse.Description).To(ContainSubstring("operation: bind"))
					Expect(errorResponse.Description).NotTo(ContainSubstring("task-id:"))
					Eventually(runningBroker).Should(gbytes.Say("adapter completely failed"))
				})
			})

			Context("when there is an operator and user error message", func() {
				BeforeEach(func() {
					adapter.CreateBinding().FailsWithCFUserAndOperatorError("error message for user", "error message for operator")
				})

				It("responds with a 500 error, including the user's error message", func() {
					Expect(bindingResponse.StatusCode).To(Equal(500))

					defer bindingResponse.Body.Close()
					var errorResponse brokerapi.ErrorResponse
					Expect(json.NewDecoder(bindingResponse.Body).Decode(&errorResponse)).To(Succeed())
					Expect(errorResponse.Description).To(ContainSubstring("error message for user"))
					Expect(errorResponse.Description).NotTo(ContainSubstring("error message for operator"))

					Eventually(runningBroker.Out).Should(gbytes.Say("error message for user"))
					Eventually(runningBroker.Out).Should(gbytes.Say("error message for operator"))
				})
			})
		})
	})

	Context("when getting VMs for a deployment responds with an error", func() {
		var errorResponse brokerapi.ErrorResponse

		JustBeforeEach(func() {
			boshDirector.VerifyAndMock(mockbosh.VMsForDeployment(deploymentName(instanceID)).RespondsInternalServerErrorWith("bosh failed"))
			bindingReq, err := http.NewRequest("PUT",
				fmt.Sprintf("http://localhost:%d/v2/service_instances/%s/service_bindings/Gjklh45ljkhn", brokerPort, instanceID),
				strings.NewReader("{}"))
			Expect(err).ToNot(HaveOccurred())
			bindingReq = basicAuthBrokerRequest(bindingReq)

			bindingResponse, err = http.DefaultClient.Do(bindingReq)
			Expect(err).ToNot(HaveOccurred())

			Expect(json.NewDecoder(bindingResponse.Body).Decode(&errorResponse)).To(Succeed())
		})

		AfterEach(func() {
			defer bindingResponse.Body.Close()
		})

		It("responds with a 500 error", func() {
			Expect(bindingResponse.StatusCode).To(Equal(http.StatusInternalServerError))
			Expect(errorResponse.Description).To(ContainSubstring(
				"There was a problem completing your request. Please contact your operations team providing the following information: ",
			))
			Expect(errorResponse.Description).To(MatchRegexp(
				`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
			))
			Expect(errorResponse.Description).To(ContainSubstring(
				fmt.Sprintf("service: %s", serviceName),
			))
			Expect(errorResponse.Description).To(ContainSubstring(
				fmt.Sprintf("service-instance-guid: %s", instanceID),
			))
			Expect(errorResponse.Description).To(ContainSubstring(
				"operation: bind",
			))
			Expect(errorResponse.Description).NotTo(ContainSubstring(
				"task-id:",
			))
		})
	})

	Context("when the bosh director is unavailable", func() {
		JustBeforeEach(func() {
			boshDirector.Close()
			bindingReq, err := http.NewRequest("PUT",
				fmt.Sprintf("http://localhost:%d/v2/service_instances/%s/service_bindings/Gjklh45ljkhn", brokerPort, instanceID),
				strings.NewReader("{}"))
			Expect(err).ToNot(HaveOccurred())
			bindingReq = basicAuthBrokerRequest(bindingReq)

			bindingResponse, err = http.DefaultClient.Do(bindingReq)
			Expect(err).ToNot(HaveOccurred())
		})

		It("responds with a 500 error with a try-again-later message in the response", func() {
			Expect(bindingResponse.StatusCode).To(Equal(http.StatusInternalServerError))

			var errorResponse brokerapi.ErrorResponse
			Expect(json.NewDecoder(bindingResponse.Body).Decode(&errorResponse)).To(Succeed())
			Expect(errorResponse.Description).To(ContainSubstring("Currently unable to bind service instance, please try again later"))
		})
	})

	Context("when the instance being bound doesn't exist", func() {
		var bindingResponse *http.Response

		JustBeforeEach(func() {
			boshDirector.VerifyAndMock(mockbosh.VMsForDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""))
			bindingReq, err := http.NewRequest("PUT",
				fmt.Sprintf("http://localhost:%d/v2/service_instances/%s/service_bindings/Gjklh45ljkhn", brokerPort, instanceID),
				strings.NewReader("{}"))
			Expect(err).ToNot(HaveOccurred())
			bindingReq = basicAuthBrokerRequest(bindingReq)

			bindingResponse, err = http.DefaultClient.Do(bindingReq)
			Expect(err).ToNot(HaveOccurred())
		})

		It("responds with a 404 error", func() {
			Expect(bindingResponse.StatusCode).To(Equal(http.StatusNotFound))

			defer bindingResponse.Body.Close()
			Expect(ioutil.ReadAll(bindingResponse.Body)).To(MatchJSON(`{"description":"instance does not exist"}`))
		})
	})
})
