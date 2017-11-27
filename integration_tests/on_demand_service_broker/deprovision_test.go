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
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("deprovisioning service instances", func() {
	const instanceID = "some-deprovisioning-instance"

	var (
		boshDirector *mockbosh.MockBOSH
		cfAPI        *mockhttp.Server
		boshUAA      *mockuaa.ClientCredentialsServer
		cfUAA        *mockuaa.ClientCredentialsServer

		runningBroker *gexec.Session
		delResp       *http.Response

		conf config.Config
	)

	BeforeEach(func() {
		boshUAA = mockuaa.NewClientCredentialsServer(boshClientID, boshClientSecret, "bosh uaa token")
		boshDirector = mockbosh.NewWithUAA(boshUAA.URL)
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")
		cfAPI = mockcfapi.New()
		conf = defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)
	})

	JustBeforeEach(func() {
		runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)
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

	Context("when CF integration is disabled", func() {
		const deleteTaskID = 2015

		JustBeforeEach(func() {
			conf.Broker.DisableCFStartupChecks = true

			boshDirector.VerifyAndMock(
				mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithRawManifest([]byte(`a: b`)),
				mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
				mockbosh.DeleteDeployment(deploymentName(instanceID)).
					WithoutContextID().RedirectsToTask(deleteTaskID),
			)

			delResp = deprovisionInstance(instanceID, "planID", "serviceID", true)
		})

		It("returns HTTP 202", func() {
			Expect(delResp.StatusCode).To(Equal(http.StatusAccepted))
		})
	})

	Context("when the service is deprovisioned with async flag", func() {
		Context("when there is no pre-delete errand", func() {
			const deleteTaskID = 2015

			JustBeforeEach(func() {

				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithRawManifest([]byte(`a: b`)),
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.DeleteDeployment(deploymentName(instanceID)).
						WithoutContextID().RedirectsToTask(deleteTaskID),
				)

				delResp = deprovisionInstance(instanceID, "planID", "serviceID", true)
			})

			It("returns HTTP 202", func() {
				Expect(delResp.StatusCode).To(Equal(http.StatusAccepted))
			})

			It("includes the operation data in the response", func() {
				body, err := ioutil.ReadAll(delResp.Body)
				Expect(err).NotTo(HaveOccurred())

				var deprovisionResponse brokerapi.DeprovisionResponse
				err = json.Unmarshal(body, &deprovisionResponse)
				Expect(err).NotTo(HaveOccurred())

				var operationData broker.OperationData
				err = json.Unmarshal([]byte(deprovisionResponse.OperationData), &operationData)
				Expect(err).NotTo(HaveOccurred())

				By("including the operation type")
				Expect(operationData.OperationType).To(Equal(broker.OperationTypeDelete))
				By("including the bosh task ID")
				Expect(operationData.BoshTaskID).To(Equal(deleteTaskID))
				By("not including a context ID")
				Expect(operationData.BoshContextID).To(BeEmpty())
			})

			It("logs the delete request with a request id", func() {
				deleteRequestRegex := logRegexpStringWithRequestIDCapture(`deleting deployment for instance`)
				Eventually(runningBroker).Should(gbytes.Say(deleteRequestRegex))
				requestID := firstMatchInOutput(runningBroker, deleteRequestRegex)
				Eventually(runningBroker).Should(gbytes.Say(requestID)) // It should use the same request ID again
			})
		})

		Context("when the plan has a pre-delete errand", func() {
			const boshErrandTaskID = 42
			var planID, errandName string

			BeforeEach(func() {
				planID = "pre-delete-errand-id"
				errandName = "cleanup-resources"

				preDeleteErrandPlan := config.Plan{
					Name: "pre-delete-errand-plan",
					ID:   planID,
					InstanceGroups: []serviceadapter.InstanceGroup{
						{
							Name:      "instance-group-name",
							VMType:    "pre-delete-errand-vm-type",
							Instances: 1,
							Networks:  []string{"net1"},
							AZs:       []string{"az1"},
						},
					},
					LifecycleErrands: &config.LifecycleErrands{
						PreDelete: config.Errand{Name: errandName},
					},
				}
				conf.ServiceCatalog.Plans = config.Plans{preDeleteErrandPlan}
			})

			Context("and the deployment exists", func() {
				JustBeforeEach(func() {

					boshDirector.VerifyAndMock(
						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithRawManifest([]byte(`a: b`)),
						mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
						mockbosh.Errand(deploymentName(instanceID), errandName, `{}`).
							WithAnyContextID().RedirectsToTask(boshErrandTaskID),
					)

					delResp = deprovisionInstance(instanceID, planID, "serviceID", true)
				})

				It("returns HTTP 202", func() {
					Expect(delResp.StatusCode).To(Equal(http.StatusAccepted))
				})

				It("logs that is running pre-delete errand", func() {
					Eventually(runningBroker).Should(gbytes.Say(
						fmt.Sprintf("running pre-delete errand for instance %s", instanceID),
					))
				})

				It("includes the operation data in the response", func() {
					body, err := ioutil.ReadAll(delResp.Body)
					Expect(err).NotTo(HaveOccurred())

					var deprovisionResponse brokerapi.DeprovisionResponse
					err = json.Unmarshal(body, &deprovisionResponse)
					Expect(err).NotTo(HaveOccurred())

					var operationData broker.OperationData
					err = json.Unmarshal([]byte(deprovisionResponse.OperationData), &operationData)
					Expect(err).NotTo(HaveOccurred())

					By("including the operation type")
					Expect(operationData.OperationType).To(Equal(broker.OperationTypeDelete))
					By("including the bosh errand task ID")
					Expect(operationData.BoshTaskID).To(Equal(boshErrandTaskID))
					By("including a UUID context ID")
					Expect(operationData.BoshContextID).To(MatchRegexp(
						`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					))
				})
			})
		})
	})

	Context("when the service is deprovisioned without async flag", func() {
		JustBeforeEach(func() {
			delResp = deprovisionInstance(instanceID, "planID", "serviceID", false)
		})

		It("returns HTTP 422", func() {
			Expect(delResp.StatusCode).To(Equal(422))
		})

		It("returns an informative error", func() {
			var respStructure map[string]interface{}
			Expect(json.NewDecoder(delResp.Body).Decode(&respStructure)).To(Succeed())

			Expect(respStructure).To(Equal(map[string]interface{}{
				"error":       "AsyncRequired",
				"description": "This service plan requires client support for asynchronous service operations.",
			}))
		})
	})

	Context("when the deployment does not exist", func() {
		JustBeforeEach(func() {
			boshDirector.VerifyAndMock(
				mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""),
			)

			delResp = deprovisionInstance(instanceID, "planID", "serviceID", true)
		})

		It("returns HTTP 410", func() {
			Expect(delResp.StatusCode).To(Equal(http.StatusGone))
		})

		It("returns no body", func() {
			Expect(ioutil.ReadAll(delResp.Body)).To(MatchJSON("{}"))
		})

		It("logs an error message", func() {
			Eventually(runningBroker.Out).Should(
				gbytes.Say(fmt.Sprintf("error deprovisioning: instance %s, not found.", instanceID)),
			)
		})
	})

	Context("when a bosh task is in flight for the service instance", func() {
		var errorResponse brokerapi.ErrorResponse

		JustBeforeEach(func() {
			boshDirector.VerifyAndMock(
				mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithRawManifest([]byte(`a: b`)),
				mockbosh.Tasks(deploymentName(instanceID)).RespondsWithATaskContainingState("processing", ""),
			)

			delResp = deprovisionInstance(instanceID, "planID", "serviceID", true)
			defer delResp.Body.Close()
			Expect(json.NewDecoder(delResp.Body).Decode(&errorResponse)).To(Succeed())
		})

		It("responds with HTTP 500", func() {
			Expect(delResp.StatusCode).To(Equal(http.StatusInternalServerError))
		})

		It("contains an operation in progress message", func() {
			Expect(errorResponse.Description).To(ContainSubstring(
				"An operation is in progress for your service instance. Please try again later.",
			))
		})

		It("logs an error message", func() {
			Eventually(runningBroker.Out).Should(gbytes.Say(fmt.Sprintf("deployment service-instance_%s is still in progress:", instanceID)))
		})
	})

	Context("when the response from bosh is not a redirect", func() {
		JustBeforeEach(func() {

			boshDirector.VerifyAndMock(
				mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithRawManifest([]byte(`a: b`)),
				mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
				mockbosh.DeleteDeployment(deploymentName(instanceID)).WithoutContextID().RespondsOKWith("not a redirect"),
			)

			delResp = deprovisionInstance(instanceID, "planID", "serviceID", true)
		})

		It("returns HTTP 500", func() {
			Expect(delResp.StatusCode).To(Equal(http.StatusInternalServerError))
		})

		Describe("error message", func() {
			var errorResponse brokerapi.ErrorResponse

			JustBeforeEach(func() {
				Expect(json.NewDecoder(delResp.Body).Decode(&errorResponse)).To(Succeed())
			})

			AfterEach(func() {
				defer delResp.Body.Close()
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
					"operation: delete",
				))
			})

			It("does NOT include the bosh task ID", func() {
				Expect(errorResponse.Description).NotTo(ContainSubstring(
					"task-id:",
				))
			})
		})

		It("logs the operator error message", func() {
			Eventually(runningBroker.Out).Should(gbytes.Say("expected status 302, was 200"))
		})
	})

	Context("when the bosh director is unavailable", func() {
		JustBeforeEach(func() {
			boshDirector.Close()

			delResp = deprovisionInstance(instanceID, "planID", "serviceID", true)
		})

		It("returns HTTP 500", func() {
			Expect(delResp.StatusCode).To(Equal(http.StatusInternalServerError))
		})

		It("returns a try again later message", func() {
			var errorResponse brokerapi.ErrorResponse
			Expect(json.NewDecoder(delResp.Body).Decode(&errorResponse)).To(Succeed())
			Expect(errorResponse.Description).To(ContainSubstring("Currently unable to delete service instance, please try again later"))
		})
	})
})
