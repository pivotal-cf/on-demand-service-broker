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

	yaml "gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshclient"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("updating a service instance", func() {
	const (
		taskID       = 4
		updateTaskID = 712
		instanceID   = "first-deployment-instance-id"
	)
	var (
		manifest        bosh.BoshManifest
		updateResp      *http.Response
		updateArbParams map[string]interface{}
		conf            config.Config
		runningBroker   *gexec.Session
		boshDirector    *mockhttp.Server
		cfAPI           *mockhttp.Server
		boshUAA         *mockuaa.ClientCredentialsServer
		cfUAA           *mockuaa.ClientCredentialsServer
	)

	BeforeEach(func() {
		manifest = bosh.BoshManifest{
			Name:           deploymentName(instanceID),
			Releases:       []bosh.Release{},
			Stemcells:      []bosh.Stemcell{},
			InstanceGroups: []bosh.InstanceGroup{},
		}
		updateArbParams = map[string]interface{}{"foo": "bar"}
		boshUAA = mockuaa.NewClientCredentialsServer(boshClientID, boshClientSecret, "bosh uaa token")
		boshDirector = mockbosh.New()
		boshDirector.ExpectedAuthorizationHeader(boshUAA.ExpectedAuthorizationHeader())
		cfAPI = mockcfapi.New()
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")
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

	Context("when switching plans", func() {
		BeforeEach(func() {
			adapter.GenerateManifest().ToReturnManifest(rawManifestWithDeploymentName(instanceID))
		})

		Context("and there are no pending changes", func() {
			JustBeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.Deploy().WithManifest(manifest).WithoutContextID().RedirectsToTask(updateTaskID),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, highMemoryPlanID)
			})

			It("returns HTTP 202", func() {
				Expect(updateResp.StatusCode).To(Equal(http.StatusAccepted))
			})

			It("includes the operation data in the response", func() {
				body, err := ioutil.ReadAll(updateResp.Body)
				Expect(err).NotTo(HaveOccurred())

				var updateResponse brokerapi.UpdateResponse
				err = json.Unmarshal(body, &updateResponse)
				Expect(err).NotTo(HaveOccurred())

				var operationData broker.OperationData
				err = json.Unmarshal([]byte(updateResponse.OperationData), &operationData)
				Expect(err).NotTo(HaveOccurred())

				By("including the operation type")
				Expect(operationData.OperationType).To(Equal(broker.OperationTypeUpdate))
				By("including the bosh task ID")
				Expect(operationData.BoshTaskID).To(Equal(updateTaskID))
				By("not including a context ID")
				Expect(operationData.BoshContextID).To(BeEmpty())
				By("not including the plan ID")
				Expect(operationData.PlanID).To(BeEmpty())
			})

			It("has the previous plan", func() {
				Expect(adapter.GenerateManifest().ReceivedPreviousPlan()).To(Equal(serviceadapter.Plan{
					Properties: serviceadapter.Properties{
						"type":            "dedicated-plan-property",
						"global_property": "global_value",
					},
					Update: dedicatedPlanUpdateBlock,
					InstanceGroups: []serviceadapter.InstanceGroup{
						{
							Name:               "instance-group-name",
							VMType:             dedicatedPlanVMType,
							VMExtensions:       dedicatedPlanVMExtensions,
							PersistentDiskType: dedicatedPlanDisk,
							Instances:          dedicatedPlanInstances,
							Networks:           dedicatedPlanNetworks,
							AZs:                dedicatedPlanAZs},
						{
							Name:               "instance-group-errand",
							Lifecycle:          "errand",
							VMType:             dedicatedPlanVMType,
							PersistentDiskType: dedicatedPlanDisk,
							Instances:          dedicatedPlanInstances,
							Networks:           dedicatedPlanNetworks,
							AZs:                dedicatedPlanAZs,
						},
					},
				}))
			})

			It("calls the adapter with service releases", func() {
				Expect(adapter.GenerateManifest().ReceivedDeployment()).To(Equal(
					serviceadapter.ServiceDeployment{
						DeploymentName: deploymentName(instanceID),
						Releases: serviceadapter.ServiceReleases{
							{
								Name:    serviceReleaseName,
								Version: serviceReleaseVersion,
								Jobs:    []string{"job-name"},
							}},
						Stemcell: serviceadapter.Stemcell{
							OS:      stemcellOS,
							Version: stemcellVersion,
						},
					},
				))
			})

			It("calls the adapter with the correct plan", func() {
				Expect(adapter.GenerateManifest().ReceivedPlan()).To(Equal(serviceadapter.Plan{
					Properties: serviceadapter.Properties{
						"type":            "high-memory-plan-property",
						"global_property": "overrides_global_value",
					},
					InstanceGroups: []serviceadapter.InstanceGroup{
						{
							Name:         "instance-group-name",
							VMType:       highMemoryPlanVMType,
							VMExtensions: highMemoryPlanVMExtensions,
							Instances:    highMemoryPlanInstances,
							Networks:     highMemoryPlanNetworks,
							AZs:          highMemoryPlanAZs,
						},
					},
				}))
			})

			It("calls the adapter with the correct request params", func() {
				Expect(adapter.GenerateManifest().ReceivedRequestParams()).To(Equal(map[string]interface{}{
					"parameters": updateArbParams,
					"service_id": serviceID,
					"plan_id":    highMemoryPlanID,
					"previous_values": map[string]interface{}{
						"organization_id": organizationGUID,
						"service_id":      serviceID,
						"plan_id":         dedicatedPlanID,
						"space_id":        "space-guid",
					},
				}))
			})

			It("calls the adapter with the correct previous manifest", func() {
				Expect(*adapter.GenerateManifest().ReceivedPreviousManifest()).To(Equal(manifest))
			})

			It("logs the update request with a request id", func() {
				updateRequestRegex := logRegexpStringWithRequestIDCapture(
					`updating instance`,
				)
				Eventually(runningBroker).Should(gbytes.Say(updateRequestRegex))
				requestID := firstMatchInOutput(runningBroker, updateRequestRegex)
				Eventually(runningBroker).Should(gbytes.Say(requestID)) // It should use the same request ID again
			})
		})

		Context("and the new plan has a post-deploy errand", func() {
			var postDeployErrandPlanID = "post-deploy-errand-id"

			BeforeEach(func() {
				postDeployErrandPlan := config.Plan{
					Name: "post-deploy-errand-plan",
					ID:   postDeployErrandPlanID,
					InstanceGroups: []serviceadapter.InstanceGroup{
						{
							Name:      "instance-group-name",
							VMType:    "post-deploy-errand-vm-type",
							Instances: 1,
							Networks:  []string{"net1"},
							AZs:       []string{"az1"},
						},
					},
					LifecycleErrands: &config.LifecycleErrands{
						PostDeploy: "health-check",
					},
				}
				conf.ServiceCatalog.Plans = append(conf.ServiceCatalog.Plans, postDeployErrandPlan)

				adapter.GenerateManifest().ToReturnManifest(rawManifestWithDeploymentName(instanceID))
			})

			JustBeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.Deploy().WithManifest(manifest).WithAnyContextID().RedirectsToTask(taskID),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, postDeployErrandPlanID)
			})

			It("responds with 202", func() {
				Expect(updateResp.StatusCode).To(Equal(http.StatusAccepted))
			})

			It("includes the operation data in the response", func() {
				body, err := ioutil.ReadAll(updateResp.Body)
				Expect(err).NotTo(HaveOccurred())

				var updateResponse brokerapi.UpdateResponse
				err = json.Unmarshal(body, &updateResponse)
				Expect(err).NotTo(HaveOccurred())

				var operationData broker.OperationData
				err = json.Unmarshal([]byte(updateResponse.OperationData), &operationData)
				Expect(err).NotTo(HaveOccurred())

				By("including the operation type")
				Expect(operationData.OperationType).To(Equal(broker.OperationTypeUpdate))
				By("including the bosh task ID")
				Expect(operationData.BoshTaskID).To(Equal(taskID))
				By("including a context ID")
				Expect(operationData.BoshContextID).NotTo(BeEmpty())
				By("including the new plan ID")
				Expect(operationData.PlanID).To(Equal(postDeployErrandPlanID))
			})
		})

		Context("and the new plan does not have a post-deploy errand", func() {
			var postDeployErrandPlanID = "post-deploy-errand-id"

			BeforeEach(func() {
				postDeployErrandPlan := config.Plan{
					Name: "post-deploy-errand-plan",
					ID:   postDeployErrandPlanID,
					InstanceGroups: []serviceadapter.InstanceGroup{
						{
							Name:      "instance-group-name",
							VMType:    "post-deploy-errand-vm-type",
							Instances: 1,
							Networks:  []string{"net1"},
							AZs:       []string{"az1"},
						},
					},
					LifecycleErrands: &config.LifecycleErrands{
						PostDeploy: "health-check",
					},
				}
				conf.ServiceCatalog.Plans = append(conf.ServiceCatalog.Plans, postDeployErrandPlan)

				adapter.GenerateManifest().ToReturnManifest(rawManifestWithDeploymentName(instanceID))
			})

			JustBeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.Deploy().WithManifest(manifest).WithoutContextID().RedirectsToTask(taskID),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, postDeployErrandPlanID, highMemoryPlanID)
			})

			It("responds with 202", func() {
				Expect(updateResp.StatusCode).To(Equal(http.StatusAccepted))
			})

			It("includes the operation data in the response", func() {
				body, err := ioutil.ReadAll(updateResp.Body)
				Expect(err).NotTo(HaveOccurred())

				var updateResponse brokerapi.UpdateResponse
				err = json.Unmarshal(body, &updateResponse)
				Expect(err).NotTo(HaveOccurred())

				var operationData broker.OperationData
				err = json.Unmarshal([]byte(updateResponse.OperationData), &operationData)
				Expect(err).NotTo(HaveOccurred())

				By("including the operation type")
				Expect(operationData.OperationType).To(Equal(broker.OperationTypeUpdate))
				By("including the bosh task ID")
				Expect(operationData.BoshTaskID).To(Equal(taskID))
				By("not including a context ID")
				Expect(operationData.BoshContextID).To(BeEmpty())
				By("not including the new plan ID")
				Expect(operationData.PlanID).To(BeEmpty())
			})
		})

		Context("and there are pending changes", func() {
			JustBeforeEach(func() {
				manifest.Properties = map[string]interface{}{"foo": "bar"}

				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, highMemoryPlanID)
			})

			Context("and the cf_user_triggered_upgrades feature is turned on", func() {
				BeforeEach(func() {
					conf.Features.CFUserTriggeredUpgrades = true
				})

				It("returns HTTP 500", func() {
					Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("returns a pending change message", func() {
					var errorResponse brokerapi.ErrorResponse
					Expect(json.NewDecoder(updateResp.Body).Decode(&errorResponse)).To(Succeed())
					Expect(errorResponse.Description).To(ContainSubstring(`There is a pending change to your service instance, you must first run cf update-service <service_name> -c '{"apply-changes": true}', no other arbitrary parameters are allowed`))
				})
			})

			Context("and the cf_user_triggered_upgrades feature is off", func() {
				It("returns HTTP 500", func() {
					Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("returns an apply changes disabled message", func() {
					var errorResponse brokerapi.ErrorResponse
					Expect(json.NewDecoder(updateResp.Body).Decode(&errorResponse)).To(Succeed())
					Expect(errorResponse.Description).To(ContainSubstring(`Service cannot be updated at this time, please try again later or contact your operator for more information`))
				})
			})
		})

		Context("and the new plan's quota has been reached already", func() {
			JustBeforeEach(func() {
				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsWith(listCFServiceOfferingsResponse(serviceID, "some-cf-service-offering-guid")),
					mockcfapi.ListServicePlans("some-cf-service-offering-guid").RespondsWithServicePlan(dedicatedPlanID, "some-cf-plan-guid"),
					mockcfapi.ListServiceInstances("some-cf-plan-guid").RespondsWith(listCFServiceInstanceCountForPlanResponse(dedicatedPlanQuota)),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, highMemoryPlanID, dedicatedPlanID)
			})

			It("returns an error when updating", func() {
				Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("returns an appropriate message", func() {
				Expect(readJSONResponse(updateResp.Body)).To(Equal(map[string]string{"description": "The quota for this service plan has been exceeded. Please contact your Operator for help."}))
			})
		})

		Context("and the bosh deployment cannot be found", func() {
			JustBeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).NotFound(),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, highMemoryPlanID)
			})

			It("responds with HTTP 500", func() {
				Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			Describe("error message", func() {
				var errorResponse brokerapi.ErrorResponse

				JustBeforeEach(func() {
					Expect(json.NewDecoder(updateResp.Body).Decode(&errorResponse)).To(Succeed())
				})

				AfterEach(func() {
					defer updateResp.Body.Close()
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
						"operation: update",
					))
				})

				It("does NOT include a bosh task ID", func() {
					Expect(errorResponse.Description).NotTo(ContainSubstring(
						"task-id:",
					))
				})
			})

			It("logs the operator error message", func() {
				Eventually(runningBroker.Out).Should(gbytes.Say(fmt.Sprintf("error deploying instance: bosh deployment '%s' not found.", deploymentName(instanceID))))
			})
		})

		Context("and the bosh director is unavailable", func() {
			JustBeforeEach(func() {
				boshDirector.Close()

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, highMemoryPlanID)
			})

			It("responds with HTTP 500", func() {
				Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("responds with a try again later message", func() {
				var errorResponse brokerapi.ErrorResponse
				Expect(json.NewDecoder(updateResp.Body).Decode(&errorResponse)).To(Succeed())
				Expect(errorResponse.Description).To(ContainSubstring("Currently unable to update service instance, please try again later"))
			})
		})

		Context("and the cf api is unavailable", func() {
			JustBeforeEach(func() {
				cfAPI.Close()
				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, highMemoryPlanID, dedicatedPlanID)
			})

			It("returns HTTP 500", func() {
				Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			Describe("error message", func() {
				var errorResponse brokerapi.ErrorResponse

				JustBeforeEach(func() {
					Expect(json.NewDecoder(updateResp.Body).Decode(&errorResponse)).To(Succeed())
				})

				AfterEach(func() {
					defer updateResp.Body.Close()
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
						"operation: update",
					))
				})

				It("does NOT include a bosh task ID", func() {
					Expect(errorResponse.Description).NotTo(ContainSubstring(
						"task-id:",
					))
				})
			})
		})

		Context("and service adapter returns an error", func() {
			JustBeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, highMemoryPlanID)
			})

			Context("and the adapter does not output a message for the cf user", func() {
				BeforeEach(func() {
					adapter.GenerateManifest().ToFailWithOperatorError("something has gone wrong in adapter")
				})

				It("responds with HTTP 500", func() {
					Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				Describe("error message", func() {
					var errorResponse brokerapi.ErrorResponse

					JustBeforeEach(func() {
						Expect(json.NewDecoder(updateResp.Body).Decode(&errorResponse)).To(Succeed())
					})

					AfterEach(func() {
						defer updateResp.Body.Close()
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
							"operation: update",
						))
					})

					It("does NOT include the bosh task ID", func() {
						Expect(errorResponse.Description).NotTo(ContainSubstring(
							"task-id:",
						))
					})
				})

				It("logs the actual error", func() {
					Eventually(runningBroker.Out).Should(gbytes.Say("something has gone wrong in adapter"))
				})
			})

			Context("and the adapter outputs a message for the cf user", func() {
				BeforeEach(func() {
					adapter.GenerateManifest().ToFailWithCFUserAndOperatorError("error message for cf user", "error message for operator")
				})

				It("responds with HTTP 500", func() {
					Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("includes the error message for the cf user in the response", func() {
					var errorResponse brokerapi.ErrorResponse
					Expect(json.NewDecoder(updateResp.Body).Decode(&errorResponse)).To(Succeed())
					Expect(errorResponse.Description).To(ContainSubstring("error message for cf user"))
				})

				It("logs both error messages", func() {
					Eventually(runningBroker.Out).Should(gbytes.Say("error message for cf user"))
					Eventually(runningBroker.Out).Should(gbytes.Say("error message for operator"))
				})
			})
		})
	})

	Context("when updating arbitrary params", func() {
		Context("and there are no pending changes", func() {
			BeforeEach(func() {
				adapter.GenerateManifest().ToReturnManifest(rawManifestWithDeploymentName(instanceID))
			})

			JustBeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.Deploy().WithManifest(manifest).WithoutContextID().RedirectsToTask(updateTaskID),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, dedicatedPlanID)
			})

			It("returns HTTP 202", func() {
				Expect(updateResp.StatusCode).To(Equal(http.StatusAccepted))
			})

			It("returns the task ID and operation type in operation data", func() {
				operationData, err := json.Marshal(broker.OperationData{BoshTaskID: updateTaskID, OperationType: broker.OperationTypeUpdate})
				Expect(err).NotTo(HaveOccurred())
				expectedResponseBody, err := json.Marshal(brokerapi.UpdateResponse{OperationData: string(operationData)})
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.ReadAll(updateResp.Body)).To(MatchJSON(expectedResponseBody))
			})

			It("includes the operation data in the response", func() {
				body, err := ioutil.ReadAll(updateResp.Body)
				Expect(err).NotTo(HaveOccurred())

				var updateResponse brokerapi.UpdateResponse
				err = json.Unmarshal(body, &updateResponse)
				Expect(err).NotTo(HaveOccurred())

				var operationData broker.OperationData
				err = json.Unmarshal([]byte(updateResponse.OperationData), &operationData)
				Expect(err).NotTo(HaveOccurred())

				By("including the operation type")
				Expect(operationData.OperationType).To(Equal(broker.OperationTypeUpdate))
				By("including the bosh task ID")
				Expect(operationData.BoshTaskID).To(Equal(updateTaskID))
				By("not including a context ID")
				Expect(operationData.BoshContextID).To(BeEmpty())
				By("not including the plan ID")
				Expect(operationData.PlanID).To(BeEmpty())
			})
		})

		Context("and there are pending changes", func() {
			var (
				generatedManifest = bosh.BoshManifest{
					Name: deploymentName(instanceID),
					Releases: []bosh.Release{{
						Name:    "foo",
						Version: "1.0",
					}},
					Stemcells:      []bosh.Stemcell{},
					InstanceGroups: []bosh.InstanceGroup{},
				}
			)

			BeforeEach(func() {
				var err error
				generatedManifestBytes, err := yaml.Marshal(generatedManifest)
				Expect(err).NotTo(HaveOccurred())

				adapter.GenerateManifest().ToReturnManifest(string(generatedManifestBytes))
			})

			Context("and the arbitrary param apply-changes is set to true", func() {
				var arbitraryParams = map[string]interface{}{"apply-changes": true}

				Context("and the cf_user_triggered_upgrades feature is turned on", func() {
					BeforeEach(func() {
						conf.Features.CFUserTriggeredUpgrades = true
					})

					JustBeforeEach(func() {
						boshDirector.VerifyAndMock(
							mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
							mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
							mockbosh.Deploy().WithManifest(generatedManifest).WithoutContextID().RedirectsToTask(updateTaskID),
						)
					})

					It("returns HTTP 202", func() {
						resp := updateServiceInstanceRequest(arbitraryParams, instanceID, dedicatedPlanID, dedicatedPlanID)
						Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
					})

					It("returns the task ID and operation type in operation data", func() {
						operationData, err := json.Marshal(broker.OperationData{BoshTaskID: updateTaskID, OperationType: broker.OperationTypeUpdate})
						Expect(err).NotTo(HaveOccurred())

						expectedResponseBody, err := json.Marshal(brokerapi.UpdateResponse{OperationData: string(operationData)})
						Expect(err).NotTo(HaveOccurred())

						resp := updateServiceInstanceRequest(arbitraryParams, instanceID, dedicatedPlanID, dedicatedPlanID)
						Expect(ioutil.ReadAll(resp.Body)).To(MatchJSON(expectedResponseBody))
					})
				})

				Context("and the cf_user_triggered_upgrades feature is off", func() {
					BeforeEach(func() {
						conf.Features.CFUserTriggeredUpgrades = false
					})

					JustBeforeEach(func() {
						boshDirector.VerifyAndMock(
							mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
							mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
						)
					})

					It("returns HTTP 500", func() {
						resp := updateServiceInstanceRequest(arbitraryParams, instanceID, dedicatedPlanID, dedicatedPlanID)
						Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
					})

					It("returns an apply changes not permitted message", func() {
						resp := updateServiceInstanceRequest(arbitraryParams, instanceID, dedicatedPlanID, dedicatedPlanID)

						var body brokerapi.ErrorResponse
						err := json.NewDecoder(resp.Body).Decode(&body)
						Expect(err).ToNot(HaveOccurred())

						Expect(body.Description).To(ContainSubstring(
							`'apply-changes' is not permitted. Contact your operator for more information`,
						))
					})
				})
			})

			Context("and the request params are apply-changes and a plan change", func() {
				var arbitraryParams = map[string]interface{}{"apply-changes": true}

				JustBeforeEach(func() {
					boshDirector.VerifyAndMock(
						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
						mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					)
				})

				It("returns HTTP 500", func() {
					resp := updateServiceInstanceRequest(arbitraryParams, instanceID, dedicatedPlanID, highMemoryPlanID)
					Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("returns an apply changes disabled message", func() {
					resp := updateServiceInstanceRequest(arbitraryParams, instanceID, dedicatedPlanID, highMemoryPlanID)

					var body brokerapi.ErrorResponse
					err := json.NewDecoder(resp.Body).Decode(&body)
					Expect(err).ToNot(HaveOccurred())

					Expect(body.Description).To(ContainSubstring(
						"'apply-changes' is not permitted. Contact your operator for more information",
					))
				})
			})

			Context("and the request params are apply-changes and anything else", func() {
				var arbitraryParams = map[string]interface{}{"apply-changes": true, "foo": "bar"}

				JustBeforeEach(func() {
					boshDirector.VerifyAndMock(
						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
						mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					)
				})

				It("returns HTTP 500", func() {
					resp := updateServiceInstanceRequest(arbitraryParams, instanceID, dedicatedPlanID, dedicatedPlanID)
					Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("returns an apply changes not permitted message", func() {
					resp := updateServiceInstanceRequest(arbitraryParams, instanceID, dedicatedPlanID, dedicatedPlanID)

					var body brokerapi.ErrorResponse
					err := json.NewDecoder(resp.Body).Decode(&body)
					Expect(err).ToNot(HaveOccurred())

					Expect(body.Description).To(ContainSubstring(
						`'apply-changes' is not permitted. Contact your operator for more information`,
					))
				})
			})

			Context("and the request params are anything else", func() {
				var arbitraryParams = map[string]interface{}{"foo": "bar"}

				JustBeforeEach(func() {
					boshDirector.VerifyAndMock(
						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
						mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					)
				})

				It("returns HTTP 500", func() {
					resp := updateServiceInstanceRequest(arbitraryParams, instanceID, dedicatedPlanID, dedicatedPlanID)
					Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("returns an apply changes disabled message", func() {
					resp := updateServiceInstanceRequest(arbitraryParams, instanceID, dedicatedPlanID, dedicatedPlanID)

					var body brokerapi.ErrorResponse
					err := json.NewDecoder(resp.Body).Decode(&body)
					Expect(err).ToNot(HaveOccurred())

					Expect(body.Description).To(ContainSubstring(
						`Service cannot be updated at this time, please try again later or contact your operator for more information`,
					))
				})
			})
		})

		Context("and there is an operation in progress", func() {
			BeforeEach(func() {
				adapter.GenerateManifest().ToReturnManifest(rawManifestWithDeploymentName(instanceID))
			})

			JustBeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithATaskContainingState(boshclient.BoshTaskProcessing, "some task"),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, dedicatedPlanID)
			})

			It("returns HTTP 500", func() {
				Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("returns a operation in progress message", func() {
				var errorResponse brokerapi.ErrorResponse
				Expect(json.NewDecoder(updateResp.Body).Decode(&errorResponse)).To(Succeed())
				Expect(errorResponse.Description).To(ContainSubstring("An operation is in progress for your service instance. Please try again later."))
			})
		})

		Context("and the plan has a post-deploy errand", func() {
			var postDeployErrandPlanID = "post-deploy-errand-id"

			BeforeEach(func() {
				postDeployErrandPlan := config.Plan{
					Name: "post-deploy-errand-plan",
					ID:   postDeployErrandPlanID,
					InstanceGroups: []serviceadapter.InstanceGroup{
						{
							Name:      "instance-group-name",
							VMType:    "post-deploy-errand-vm-type",
							Instances: 1,
							Networks:  []string{"net1"},
							AZs:       []string{"az1"},
						},
					},
					LifecycleErrands: &config.LifecycleErrands{
						PostDeploy: "health-check",
					},
				}
				conf.ServiceCatalog.Plans = config.Plans{postDeployErrandPlan}

				adapter.GenerateManifest().ToReturnManifest(rawManifestWithDeploymentName(instanceID))
			})

			JustBeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.Deploy().WithManifest(manifest).WithAnyContextID().RedirectsToTask(taskID),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, postDeployErrandPlanID, postDeployErrandPlanID)
			})

			It("responds with 202", func() {
				Expect(updateResp.StatusCode).To(Equal(http.StatusAccepted))
			})

			It("includes the operation data in the response", func() {
				body, err := ioutil.ReadAll(updateResp.Body)
				Expect(err).NotTo(HaveOccurred())

				var updateResponse brokerapi.UpdateResponse
				err = json.Unmarshal(body, &updateResponse)
				Expect(err).NotTo(HaveOccurred())

				var operationData broker.OperationData
				err = json.Unmarshal([]byte(updateResponse.OperationData), &operationData)
				Expect(err).NotTo(HaveOccurred())

				By("including the operation type")
				Expect(operationData.OperationType).To(Equal(broker.OperationTypeUpdate))
				By("including the bosh task ID")
				Expect(operationData.BoshTaskID).To(Equal(taskID))
				By("including a context ID")
				Expect(operationData.BoshContextID).NotTo(BeEmpty())
				By("including the plan ID")
				Expect(operationData.PlanID).To(Equal(postDeployErrandPlanID))
			})
		})
	})
})

func updateServiceInstanceRequest(updateArbParams map[string]interface{}, instanceID, oldPlanID, newPlanID string) *http.Response {
	reqBody := map[string]interface{}{
		"plan_id":    newPlanID,
		"parameters": updateArbParams,
		"service_id": serviceID,
		"previous_values": map[string]interface{}{
			"organization_id": organizationGUID,
			"service_id":      serviceID,
			"plan_id":         oldPlanID,
			"space_id":        "space-guid",
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	Expect(err).ToNot(HaveOccurred())

	req, err := http.NewRequest(
		"PATCH",
		fmt.Sprintf("http://localhost:%d/v2/service_instances/%s?accepts_incomplete=true", brokerPort, instanceID),
		bytes.NewReader(bodyBytes),
	)
	Expect(err).NotTo(HaveOccurred())
	req = basicAuthBrokerRequest(req)
	updateResp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	return updateResp
}
