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
		cfClient     string
		boshDirector *mockbosh.MockBOSH
		cfAPI        *mockhttp.Server
		boshUAA      *mockuaa.ClientCredentialsServer
		cfUAA        *mockuaa.ClientCredentialsServer

		runningBroker   *gexec.Session
		bindingResponse *http.Response
		brokerConfig    config.Config

		instanceID                 = "some-binding-instance-ID"
		manifestForFirstDeployment = bosh.BoshManifest{
			Name:           deploymentName(instanceID),
			Releases:       []bosh.Release{},
			Stemcells:      []bosh.Stemcell{},
			InstanceGroups: []bosh.InstanceGroup{},
		}
	)

	BeforeEach(func() {
		cfClient = "cf"
	})

	JustBeforeEach(func() {

		boshUAA = mockuaa.NewClientCredentialsServer(boshClientID, boshClientSecret, "bosh uaa token")
		boshDirector = mockbosh.NewWithUAA(boshUAA.URL)
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")
		cfAPI = mockcfapi.New()

		brokerConfig = defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)

		switch cfClient {
		case "noopcf":
			brokerConfig.Broker.DisableCFStartupChecks = true
			runningBroker = startBrokerInNoopCFModeWithPassingStartupChecks(brokerConfig, boshDirector)
		default:
			runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)
		}

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

	Describe("successful bindings", func() {
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

		Context("when CF is disabled", func() {

			BeforeEach(func() {
				cfClient = "noopcf"
			})

			It("returns HTTP 201", func() {
				Expect(bindingResponse.StatusCode).To(Equal(http.StatusCreated))
			})

		})

		It("returns HTTP 201", func() {
			Expect(bindingResponse.StatusCode).To(Equal(http.StatusCreated))
		})

		It("returns credentials, syslog drain URL, and route service URL from service adapter", func() {
			var binding brokerapi.Binding
			defer bindingResponse.Body.Close()
			Expect(json.NewDecoder(bindingResponse.Body).Decode(&binding)).To(Succeed())

			credentials := binding.Credentials.(map[string]interface{})
			Expect(credentials).To(Equal(map[string]interface{}{"secret": "dont-tell-anyone"}))
			Expect(binding.RouteServiceURL).To(Equal("excellent route"))
			Expect(binding.SyslogDrainURL).To(Equal("syslog-url"))
		})

		It("calls the adapter with expected binding ID", func() {
			Expect(adapter.CreateBinding().ReceivedID()).To(Equal("Gjklh45ljkhn"))
		})

		It("calls the adapter with expected bosh VMS", func() {
			Expect(adapter.CreateBinding().ReceivedBoshVms()).To(Equal(bosh.BoshVMs{"some-instance-group": []string{"ip.from.bosh"}}))
		})

		It("calls the adapter with the correct request params", func() {
			Expect(adapter.CreateBinding().ReceivedRequestParameters()).To(Equal(map[string]interface{}{
				"plan_id":    bindingPlanID,
				"service_id": bindingServiceID,
				"app_guid":   appGUID,
				"bind_resource": map[string]interface{}{
					"app_guid": appGUID,
				},
				"parameters": bindingParams,
			}))
		})

		It("calls the adapter with the bosh manifest", func() {
			Expect(adapter.CreateBinding().ReceivedManifest()).To(Equal(manifestForFirstDeployment))
		})

		It("logs the bind request with a request id", func() {
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

			It("returns HTTP 201", func() {
				Expect(bindingResponse.StatusCode).To(Equal(http.StatusCreated))
			})

			It("does not send JSON keys for any missing optional fields", func() {
				defer bindingResponse.Body.Close()
				bodyBytes, err := ioutil.ReadAll(bindingResponse.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(bodyBytes).NotTo(ContainSubstring("syslog_drain_url"))
				Expect(bodyBytes).NotTo(ContainSubstring("route_service_url"))
			})
		})
	})

	Context("when the binding fails due to a adapter error", func() {
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

		Context("when binding fails due to adapter error, code 49", func() {
			Context("fails with binding already exists error message only", func() {
				BeforeEach(func() {
					adapter.CreateBinding().FailsWithBindingAlreadyExistsError()
				})

				It("returns HTTP 409", func() {
					Expect(bindingResponse.StatusCode).To(Equal(409))
				})

				It("returns a generic error response to the user", func() {
					defer bindingResponse.Body.Close()
					Expect(ioutil.ReadAll(bindingResponse.Body)).To(MatchJSON(`{"description":"binding already exists"}`))
				})
			})

			Context("fails with binding already exists and stderr error messages", func() {
				BeforeEach(func() {
					adapter.CreateBinding().FailsWithBindingAlreadyExistsErrorAndStderr("stderr error message")
				})

				It("returns HTTP 409", func() {
					Expect(bindingResponse.StatusCode).To(Equal(409))
				})

				It("returns a binding already exists error to the user", func() {
					defer bindingResponse.Body.Close()
					Expect(ioutil.ReadAll(bindingResponse.Body)).To(MatchJSON(`{"description":"binding already exists"}`))
				})

				It("logs the stdout and stderr error messages", func() {
					Eventually(runningBroker.Out).Should(gbytes.Say(`stderr error message`))
				})
			})
		})

		Context("when binding fails due to adapter error, code 42", func() {
			Context("fails with binding already exists error message only", func() {
				BeforeEach(func() {
					adapter.CreateBinding().FailsWithAppGuidNotProvidedError()
				})

				It("returns HTTP 422", func() {
					Expect(bindingResponse.StatusCode).To(Equal(422))
				})

				It("returns an error response to the user", func() {
					defer bindingResponse.Body.Close()
					Expect(ioutil.ReadAll(bindingResponse.Body)).To(MatchJSON(`{"description":"app_guid is a required field but was not provided"}`))
				})
			})

			Context("fails with binding already exists and stderr error messages", func() {
				BeforeEach(func() {
					adapter.CreateBinding().FailsWithAppGuidNotProvidedErrorAndStderr("stderr error message")
				})

				It("returns HTTP 422", func() {
					Expect(bindingResponse.StatusCode).To(Equal(422))
				})

				It("returns an error response to the user", func() {
					defer bindingResponse.Body.Close()
					Expect(ioutil.ReadAll(bindingResponse.Body)).To(MatchJSON(`{"description":"app_guid is a required field but was not provided"}`))
				})

				It("logs the stdout and stderr error messages", func() {
					Eventually(runningBroker.Out).Should(gbytes.Say(`stderr error message`))
				})
			})
		})

		Context("when the adapter does not implement binder, code 10", func() {
			BeforeEach(func() {
				adapter.Binder().NotImplemented()
			})

			It("returns HTTP 500", func() {
				Expect(bindingResponse.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			Describe("error message", func() {
				var errorResponse brokerapi.ErrorResponse

				JustBeforeEach(func() {
					Expect(json.NewDecoder(bindingResponse.Body).Decode(&errorResponse)).To(Succeed())
				})

				AfterEach(func() {
					defer bindingResponse.Body.Close()
				})

				It("contains a generic message", func() {
					Expect(errorResponse.Description).To(ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information: ",
					))
				})

				It("includes the request ID", func() {
					Expect(errorResponse.Description).To(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					))
				})

				It("includes the service name", func() {
					Expect(errorResponse.Description).To(ContainSubstring(
						fmt.Sprintf("service: %s", serviceName),
					))
				})

				It("includes a service instance guid", func() {
					Expect(errorResponse.Description).To(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					))
				})

				It("includes the operation type", func() {
					Expect(errorResponse.Description).To(ContainSubstring(
						"operation: bind",
					))
				})

				It("does NOT include a bosh task ID", func() {
					Expect(errorResponse.Description).NotTo(ContainSubstring(
						"task-id:",
					))
				})
			})

			It("logs that the adapter does not implement create-binding", func() {
				Eventually(runningBroker).Should(gbytes.Say("creating binding: command not implemented"))
			})
		})

		Context("when binding fails due to unknown adapter error", func() {
			Context("when there is operator error message and no user error message", func() {
				BeforeEach(func() {
					adapter.CreateBinding().FailsWithOperatorError("adapter completely failed")
				})

				It("returns HTTP 500", func() {
					Expect(bindingResponse.StatusCode).To(Equal(500))
				})

				Describe("error message", func() {
					var errorResponse brokerapi.ErrorResponse

					JustBeforeEach(func() {
						Expect(json.NewDecoder(bindingResponse.Body).Decode(&errorResponse)).To(Succeed())
					})

					AfterEach(func() {
						defer bindingResponse.Body.Close()
					})

					It("contains a generic message", func() {
						Expect(errorResponse.Description).To(ContainSubstring(
							"There was a problem completing your request. Please contact your operations team providing the following information: ",
						))
					})

					It("includes the request ID", func() {
						Expect(errorResponse.Description).To(MatchRegexp(
							`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						))
					})

					It("includes the service name", func() {
						Expect(errorResponse.Description).To(ContainSubstring(
							fmt.Sprintf("service: %s", serviceName),
						))
					})

					It("includes a service instance guid", func() {
						Expect(errorResponse.Description).To(ContainSubstring(
							fmt.Sprintf("service-instance-guid: %s", instanceID),
						))
					})

					It("includes the operation type", func() {
						Expect(errorResponse.Description).To(ContainSubstring(
							"operation: bind",
						))
					})

					It("does NOT include a bosh task ID", func() {
						Expect(errorResponse.Description).NotTo(ContainSubstring(
							"task-id:",
						))
					})
				})

				It("logs the adapter error", func() {
					Eventually(runningBroker).Should(gbytes.Say("adapter completely failed"))
				})
			})

			Context("when there is an operator and user error message", func() {
				BeforeEach(func() {
					adapter.CreateBinding().FailsWithCFUserAndOperatorError("error message for user", "error message for operator")
				})

				It("returns HTTP 500", func() {
					Expect(bindingResponse.StatusCode).To(Equal(500))
				})

				It("returns the user error message in the response", func() {
					defer bindingResponse.Body.Close()
					var errorResponse brokerapi.ErrorResponse
					Expect(json.NewDecoder(bindingResponse.Body).Decode(&errorResponse)).To(Succeed())
					Expect(errorResponse.Description).To(ContainSubstring("error message for user"))
					Expect(errorResponse.Description).NotTo(ContainSubstring("error message for operator"))
				})

				It("logs both error messages", func() {
					Eventually(runningBroker.Out).Should(gbytes.Say("error message for user"))
					Eventually(runningBroker.Out).Should(gbytes.Say("error message for operator"))
				})
			})
		})
	})

	Context("when getting VMs for a deployment responds with an error", func() {
		JustBeforeEach(func() {
			boshDirector.VerifyAndMock(mockbosh.VMsForDeployment(deploymentName(instanceID)).RespondsInternalServerErrorWith("bosh failed"))
			bindingReq, err := http.NewRequest("PUT",
				fmt.Sprintf("http://localhost:%d/v2/service_instances/%s/service_bindings/Gjklh45ljkhn", brokerPort, instanceID),
				strings.NewReader("{}"))
			Expect(err).ToNot(HaveOccurred())
			bindingReq = basicAuthBrokerRequest(bindingReq)

			bindingResponse, err = http.DefaultClient.Do(bindingReq)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns HTTP 500", func() {
			Expect(bindingResponse.StatusCode).To(Equal(http.StatusInternalServerError))
		})

		Describe("error message", func() {
			var errorResponse brokerapi.ErrorResponse

			JustBeforeEach(func() {
				Expect(json.NewDecoder(bindingResponse.Body).Decode(&errorResponse)).To(Succeed())
			})

			AfterEach(func() {
				defer bindingResponse.Body.Close()
			})

			It("contains a generic message", func() {
				Expect(errorResponse.Description).To(ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information: ",
				))
			})

			It("includes the request ID", func() {
				Expect(errorResponse.Description).To(MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				))
			})

			It("includes the service name", func() {
				Expect(errorResponse.Description).To(ContainSubstring(
					fmt.Sprintf("service: %s", serviceName),
				))
			})

			It("includes a service instance guid", func() {
				Expect(errorResponse.Description).To(ContainSubstring(
					fmt.Sprintf("service-instance-guid: %s", instanceID),
				))
			})

			It("includes the operation type", func() {
				Expect(errorResponse.Description).To(ContainSubstring(
					"operation: bind",
				))
			})

			It("does NOT include a bosh task ID", func() {
				Expect(errorResponse.Description).NotTo(ContainSubstring(
					"task-id:",
				))
			})
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

		It("responds with HTTP 500", func() {
			Expect(bindingResponse.StatusCode).To(Equal(http.StatusInternalServerError))
		})

		It("includes a try again later message in the response", func() {
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

		It("returns HTTP gone", func() {
			Expect(bindingResponse.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("returns an error response to the user", func() {
			defer bindingResponse.Body.Close()
			Expect(ioutil.ReadAll(bindingResponse.Body)).To(MatchJSON(`{"description":"instance does not exist"}`))
		})
	})
})
