// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_tests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("unbinding service instances", func() {
	var (
		boshDirector *mockbosh.MockBOSH
		cfAPI        *mockhttp.Server
		boshUAA      *mockuaa.ClientCredentialsServer
		cfUAA        *mockuaa.ClientCredentialsServer

		runningBroker *gexec.Session

		instanceID                 = "some-instance-being-unbound"
		bindingPlanID              = "plan-guid-from-cc"
		bindingServiceID           = "service-guid-from-cc"
		manifestForFirstDeployment = bosh.BoshManifest{
			Name:           deploymentName(instanceID),
			Releases:       []bosh.Release{},
			Stemcells:      []bosh.Stemcell{},
			InstanceGroups: []bosh.InstanceGroup{},
		}
		unbindResponse *http.Response

		beforeUnbinding func()
	)

	BeforeEach(func() {
		beforeUnbinding = func() {}
		boshUAA = mockuaa.NewClientCredentialsServer(boshClientID, boshClientSecret, "bosh uaa token")
		boshDirector = mockbosh.NewWithUAA(boshUAA.URL)
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")
		cfAPI = mockcfapi.New()
	})

	JustBeforeEach(func() {
		runningBroker = startBrokerWithPassingStartupChecks(defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL), cfAPI, boshDirector)
		beforeUnbinding()
		unbindResponse = unbind(brokerPort, instanceID, bindingServiceID, bindingPlanID)
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

	Describe("successful unbindings", func() {
		BeforeEach(func() {
			beforeUnbinding = func() {
				boshDirector.VerifyAndMock(
					mockbosh.VMsForDeployment(deploymentName(instanceID)).RedirectsToTask(2015),
					mockbosh.Task(2015).RespondsWithTaskContainingState(boshdirector.TaskDone),
					mockbosh.TaskOutput(2015).RespondsWithVMsOutput([]boshdirector.BoshVMsOutput{{IPs: []string{"ip.from.bosh"}, InstanceGroup: "some-instance-group"}}),
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifestForFirstDeployment),
				)
			}
		})

		It("returns HTTP 200", func() {
			Expect(unbindResponse.StatusCode).To(Equal(http.StatusOK))
		})

		It("calls the adapter with expected binding ID", func() {
			Expect(adapter.DeleteBinding().ReceivedBindingID()).To(Equal("Gjklh45ljkhn"))
		})

		It("calls the adapter with expected bosh VMS", func() {
			Expect(adapter.DeleteBinding().ReceivedBoshVms()).To(Equal(bosh.BoshVMs{"some-instance-group": []string{"ip.from.bosh"}}))
		})

		It("calls the adapter with the bosh manifest", func() {
			Expect(adapter.DeleteBinding().ReceivedManifest()).To(Equal(manifestForFirstDeployment))
		})

		It("calls the adapter with the request params", func() {
			Expect(adapter.DeleteBinding().ReceivedRequestParameters()).To(Equal(serviceadapter.RequestParameters{
				"plan_id":    bindingPlanID,
				"service_id": bindingServiceID,
			}))
		})

		It("logs the unbind request with a request id", func() {
			unbindRequestRegex := logRegexpStringWithRequestIDCapture(
				`service adapter will delete binding with ID`,
			)
			Eventually(runningBroker).Should(gbytes.Say(unbindRequestRegex))
			requestID := firstMatchInOutput(runningBroker, unbindRequestRegex)
			Eventually(runningBroker).Should(gbytes.Say(requestID)) // It should use the same request ID again
		})
	})

	Describe("unsuccessful bindings", func() {
		Context("due to an adapter error", func() {
			BeforeEach(func() {
				beforeUnbinding = func() {
					boshDirector.VerifyAndMock(
						mockbosh.VMsForDeployment(deploymentName(instanceID)).RedirectsToTask(2015),
						mockbosh.Task(2015).RespondsWithTaskContainingState(boshdirector.TaskDone),
						mockbosh.TaskOutput(2015).RespondsWithVMsOutput([]boshdirector.BoshVMsOutput{{IPs: []string{"ip.from.bosh"}, InstanceGroup: "some-instance-group"}}),
						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifestForFirstDeployment),
					)
				}
			})

			Context("fails with operator error only", func() {
				BeforeEach(func() {
					adapter.DeleteBinding().FailsWithOperatorError("adapter completely failed")
				})

				It("returns HTTP 500", func() {
					Expect(unbindResponse.StatusCode).To(Equal(500))
				})

				Describe("error message", func() {
					var errorResponse brokerapi.ErrorResponse

					JustBeforeEach(func() {
						Expect(json.NewDecoder(unbindResponse.Body).Decode(&errorResponse)).To(Succeed())
					})

					AfterEach(func() {
						defer unbindResponse.Body.Close()
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
							"operation: unbind",
						))
					})

					It("does NOT include a bosh task ID", func() {
						Expect(errorResponse.Description).NotTo(ContainSubstring(
							"task-id:",
						))
					})
				})

				It("logs the operator error message", func() {
					Eventually(runningBroker.Out).Should(gbytes.Say("adapter completely failed"))
				})
			})

			Context("fails with cf user and operator error", func() {
				BeforeEach(func() {
					adapter.DeleteBinding().FailsWithCFUserAndOperatorError("error message for user", "error message for operator")
				})

				It("returns HTTP 500", func() {
					Expect(unbindResponse.StatusCode).To(Equal(500))
				})

				It("returns the user error message in the response", func() {
					defer unbindResponse.Body.Close()
					var errorResponse brokerapi.ErrorResponse
					Expect(json.NewDecoder(unbindResponse.Body).Decode(&errorResponse)).To(Succeed())
					Expect(errorResponse.Description).To(ContainSubstring("error message for user"))
					Expect(errorResponse.Description).NotTo(ContainSubstring("error message for operator"))
				})

				It("logs both error messages", func() {
					Eventually(runningBroker.Out).Should(gbytes.Say("error message for user"))
					Eventually(runningBroker.Out).Should(gbytes.Say("error message for operator"))
				})
			})

			Context("when the adapter cannot find the binding, exit code 41", func() {
				BeforeEach(func() {
					adapter.DeleteBinding().FailsWithBindingNotFoundError()
				})

				It("returns HTTP 410", func() {
					Expect(unbindResponse.StatusCode).To(Equal(410))
				})

				It("returns empty json body", func() {
					defer unbindResponse.Body.Close()
					Expect(ioutil.ReadAll(unbindResponse.Body)).To(MatchJSON(`{}`))
				})
			})

			Context("when the adapter does not implement binder", func() {
				BeforeEach(func() {
					adapter.Binder().NotImplemented()
				})

				It("returns HTTP 500", func() {
					Expect(unbindResponse).NotTo(BeNil())
					Expect(unbindResponse.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				Describe("error message", func() {
					var errorResponse brokerapi.ErrorResponse

					JustBeforeEach(func() {
						Expect(json.NewDecoder(unbindResponse.Body).Decode(&errorResponse)).To(Succeed())
					})

					AfterEach(func() {
						defer unbindResponse.Body.Close()
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
							"operation: unbind",
						))
					})

					It("does NOT include a bosh task ID", func() {
						Expect(errorResponse.Description).NotTo(ContainSubstring(
							"task-id:",
						))
					})
				})

				It("logs that the adapter does not implement delete-binding", func() {
					Eventually(runningBroker).Should(gbytes.Say("delete binding: command not implemented by service adapter"))
				})
			})
		})

		Context("due to a BOSH error", func() {
			BeforeEach(func() {
				beforeUnbinding = func() {
					boshDirector.VerifyAndMock(
						mockbosh.VMsForDeployment(deploymentName(instanceID)).RespondsInternalServerErrorWith("bosh error"),
					)
				}
			})

			It("returns HTTP 500", func() {
				Expect(unbindResponse.StatusCode).To(Equal(500))
			})

			Describe("error message", func() {
				var errorResponse brokerapi.ErrorResponse

				JustBeforeEach(func() {
					Expect(json.NewDecoder(unbindResponse.Body).Decode(&errorResponse)).To(Succeed())
				})

				AfterEach(func() {
					defer unbindResponse.Body.Close()
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
						"operation: unbind",
					))
				})
			})
		})

		Context("when BOSH is unavailable", func() {
			BeforeEach(func() {
				beforeUnbinding = func() {
					boshDirector.Close()
				}
			})

			It("responds with HTTP 500", func() {
				Expect(unbindResponse.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("includes a try again later message in the response", func() {
				var errorResponse brokerapi.ErrorResponse
				Expect(json.NewDecoder(unbindResponse.Body).Decode(&errorResponse)).To(Succeed())
				Expect(errorResponse.Description).To(ContainSubstring(
					"Currently unable to unbind service instance, please try again later",
				))
			})
		})

		Context("when a non existing instance is unbound", func() {
			BeforeEach(func() {
				beforeUnbinding = func() {
					boshDirector.VerifyAndMock(
						mockbosh.VMsForDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""),
					)
				}
			})

			It("returns HTTP 410", func() {
				Expect(unbindResponse.StatusCode).To(Equal(http.StatusGone))
			})
		})
	})
})

func unbind(brokerPort int, instanceID, serviceID, planID string) *http.Response {
	path := fmt.Sprintf("http://localhost:%d/v2/service_instances/%s/service_bindings/Gjklh45ljkhn?service_id=%s&plan_id=%s", brokerPort, instanceID, serviceID, planID)
	unbindingReq, err := http.NewRequest("DELETE", path, nil)
	Expect(err).ToNot(HaveOccurred())
	unbindingReq = basicAuthBrokerRequest(unbindingReq)

	unbindingResponse, err := http.DefaultClient.Do(unbindingReq)
	Expect(err).ToNot(HaveOccurred())
	return unbindingResponse
}
