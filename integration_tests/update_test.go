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
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	yaml "gopkg.in/yaml.v2"
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
			It("includes the operation data in the response", func() {
				boshDirector.VerifyAndMock(
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
					mockbosh.Deploy().WithManifest(manifest).WithoutContextID().RedirectsToTask(updateTaskID),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, highMemoryPlanID)

				Expect(updateResp.StatusCode).To(Equal(http.StatusAccepted))

				Expect(*operationDataFromUpdateResponse(updateResp)).To(Equal(
					broker.OperationData{
						OperationType: broker.OperationTypeUpdate,
						BoshTaskID:    updateTaskID,
					},
				))

				Expect(adapter.GenerateManifest().ReceivedPreviousPlan()).To(Equal(
					serviceadapter.Plan{
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
					},
				))

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

				Expect(adapter.GenerateManifest().ReceivedPlan()).To(Equal(
					serviceadapter.Plan{
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
					},
				))

				Expect(adapter.GenerateManifest().ReceivedRequestParams()).To(Equal(
					map[string]interface{}{
						"parameters": updateArbParams,
						"service_id": serviceID,
						"plan_id":    highMemoryPlanID,
						"previous_values": map[string]interface{}{
							"organization_id": organizationGUID,
							"service_id":      serviceID,
							"plan_id":         dedicatedPlanID,
							"space_id":        "space-guid",
						},
					},
				))

				Expect(*adapter.GenerateManifest().ReceivedPreviousManifest()).To(Equal(manifest))

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

			It("includes the operation data in the response", func() {
				boshDirector.VerifyAndMock(
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
					mockbosh.Deploy().WithManifest(manifest).WithAnyContextID().RedirectsToTask(taskID),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, postDeployErrandPlanID)

				Expect(updateResp.StatusCode).To(Equal(http.StatusAccepted))

				operationData := operationDataFromUpdateResponse(updateResp)
				Expect(operationData.BoshContextID).NotTo(BeEmpty())
				Expect(*operationData).To(Equal(
					broker.OperationData{
						OperationType:        broker.OperationTypeUpdate,
						BoshTaskID:           taskID,
						BoshContextID:        operationData.BoshContextID,
						PostDeployErrandName: "health-check",
					},
				))
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

			It("includes the operation data in the response", func() {
				boshDirector.VerifyAndMock(
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
					mockbosh.Deploy().WithManifest(manifest).WithoutContextID().RedirectsToTask(taskID),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, postDeployErrandPlanID, highMemoryPlanID)
				Expect(updateResp.StatusCode).To(Equal(http.StatusAccepted))

				Expect(*operationDataFromUpdateResponse(updateResp)).To(Equal(
					broker.OperationData{
						OperationType: broker.OperationTypeUpdate,
						BoshTaskID:    taskID,
					},
				))
			})
		})

		Context("and there are pending changes", func() {
			JustBeforeEach(func() {
				manifest.Properties = map[string]interface{}{"foo": "bar"}

				boshDirector.VerifyAndMock(
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, highMemoryPlanID)
			})

			Context("and the cf_user_triggered_upgrades feature is turned on", func() {
				BeforeEach(func() {
					conf.Features.UserTriggeredUpgrades = true
				})

				It("reports a pending change message", func() {
					Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
					Expect(descriptionFrom(updateResp)).To(ContainSubstring(broker.PendingChangesErrorMessage))
				})
			})

			Context("and the cf_user_triggered_upgrades feature is off", func() {
				BeforeEach(func() {
					conf.Features.UserTriggeredUpgrades = false
				})

				It("reports a apply changes disabled message", func() {
					Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
					Expect(descriptionFrom(updateResp)).To(ContainSubstring(broker.ApplyChangesDisabledMessage))
				})
			})
		})

		Context("and the new plan's quota has been reached already", func() {
			It("returns a quota message", func() {
				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsOKWith(listCFServiceOfferingsResponse(serviceID, "some-cf-service-offering-guid")),
					mockcfapi.ListServicePlans("some-cf-service-offering-guid").RespondsWithServicePlan(dedicatedPlanID, "some-cf-plan-guid"),
					mockcfapi.ListServiceInstances("some-cf-plan-guid").RespondsOKWith(listCFServiceInstanceCountForPlanResponse(dedicatedPlanQuota)),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, highMemoryPlanID, dedicatedPlanID)
				Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
				Expect(descriptionFrom(updateResp)).To(Equal("The quota for this service plan has been exceeded. Please contact your Operator for help."))
			})
		})

		Context("and the bosh deployment cannot be found", func() {
			It("fails with description", func() {
				boshDirector.VerifyAndMock(
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, highMemoryPlanID)
				Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
				Expect(descriptionFrom(updateResp)).To(reportGenericFailureFor(instanceID))

				Eventually(runningBroker.Out).Should(gbytes.Say(fmt.Sprintf("error deploying instance: bosh deployment '%s' not found.", deploymentName(instanceID))))
			})
		})

		Context("and the bosh director is unavailable", func() {
			JustBeforeEach(func() {
				boshDirector.Close()

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, highMemoryPlanID)
			})

			It("responds with a try again later message", func() {
				Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
				Expect(descriptionFrom(updateResp)).To(ContainSubstring("Currently unable to update service instance, please try again later"))
			})
		})

		Context("and the cf api is unavailable", func() {
			It("reports a failure", func() {
				cfAPI.Close()

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, highMemoryPlanID, dedicatedPlanID)

				Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
				Expect(descriptionFrom(updateResp)).To(reportGenericFailureFor(instanceID))
			})
		})

		Context("and service adapter returns an error", func() {
			JustBeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, highMemoryPlanID)
			})

			Context("and the adapter does not output a message for the cf user", func() {
				BeforeEach(func() {
					adapter.GenerateManifest().ToFailWithOperatorError("something has gone wrong in adapter")
				})

				It("reports the failure", func() {
					Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))

					Expect(descriptionFrom(updateResp)).To(reportGenericFailureFor(instanceID))
					Eventually(runningBroker.Out).Should(gbytes.Say("something has gone wrong in adapter"))
				})
			})

			Context("and the adapter outputs a message for the cf user", func() {
				BeforeEach(func() {
					adapter.GenerateManifest().ToFailWithCFUserAndOperatorError("error message for cf user", "error message for operator")
				})

				It("reports the error message for the cf user in the response", func() {
					Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
					Expect(descriptionFrom(updateResp)).To(ContainSubstring("error message for cf user"))

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

			It("includes the operation data in the response", func() {
				boshDirector.VerifyAndMock(
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
					mockbosh.Deploy().WithManifest(manifest).WithoutContextID().RedirectsToTask(updateTaskID),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, dedicatedPlanID)
				Expect(updateResp.StatusCode).To(Equal(http.StatusAccepted))

				Expect(*operationDataFromUpdateResponse(updateResp)).To(Equal(
					broker.OperationData{
						OperationType: broker.OperationTypeUpdate,
						BoshTaskID:    updateTaskID,
					},
				))
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
						conf.Features.UserTriggeredUpgrades = true
					})

					It("returns the task ID and operation type in operation data", func() {
						boshDirector.VerifyAndMock(
							mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
							mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
							mockbosh.Deploy().WithManifest(generatedManifest).WithoutContextID().RedirectsToTask(updateTaskID),
						)

						resp := updateServiceInstanceRequest(arbitraryParams, instanceID, dedicatedPlanID, dedicatedPlanID)
						Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

						Expect(*operationDataFromUpdateResponse(resp)).To(Equal(
							broker.OperationData{
								OperationType: broker.OperationTypeUpdate,
								BoshTaskID:    updateTaskID,
							},
						))
					})
				})

				Context("and the cf_user_triggered_upgrades feature is off", func() {
					BeforeEach(func() {
						conf.Features.UserTriggeredUpgrades = false
					})

					It("returns an apply changes not permitted message", func() {
						resp := updateServiceInstanceRequest(arbitraryParams, instanceID, dedicatedPlanID, dedicatedPlanID)
						Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

						Expect(descriptionFrom(resp)).To(ContainSubstring(broker.ApplyChangesNotPermittedMessage))
					})
				})
			})

			Context("and the request params are apply-changes and a plan change", func() {
				It("fails with an apply changes disabled message", func() {
					parameters := map[string]interface{}{"apply-changes": true}
					resp := updateServiceInstanceRequest(parameters, instanceID, dedicatedPlanID, highMemoryPlanID)
					Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

					Expect(descriptionFrom(resp)).To(ContainSubstring(broker.ApplyChangesNotPermittedMessage))
				})
			})

			Context("and the request params are apply-changes and anything else", func() {
				It("returns an apply changes not permitted message", func() {
					parameters := map[string]interface{}{"apply-changes": true, "foo": "bar"}
					resp := updateServiceInstanceRequest(parameters, instanceID, dedicatedPlanID, dedicatedPlanID)
					Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

					Expect(descriptionFrom(resp)).To(ContainSubstring(broker.ApplyChangesNotPermittedMessage))
				})
			})

			Context("and the request params are anything else", func() {
				It("returns an apply changes disabled message", func() {
					parameters := map[string]interface{}{"foo": "bar"}

					boshDirector.VerifyAndMock(
						mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
					)

					resp := updateServiceInstanceRequest(parameters, instanceID, dedicatedPlanID, dedicatedPlanID)
					Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

					Expect(descriptionFrom(resp)).To(ContainSubstring(broker.ApplyChangesDisabledMessage))
				})
			})
		})

		Context("and there is an operation in progress", func() {
			BeforeEach(func() {
				adapter.GenerateManifest().ToReturnManifest(rawManifestWithDeploymentName(instanceID))
			})

			It("returns a operation in progress message", func() {
				boshDirector.VerifyAndMock(
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithATaskContainingState(boshdirector.TaskProcessing, "some task"),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, dedicatedPlanID, dedicatedPlanID)
				Expect(updateResp.StatusCode).To(Equal(http.StatusInternalServerError))
				Expect(descriptionFrom(updateResp)).To(ContainSubstring(broker.OperationInProgressMessage))
			})
		})

		Context("and the plan has a post-deploy errand", func() {
			const postDeployErrandPlanID = "post-deploy-errand-id"

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

			It("includes the operation data in the response", func() {
				boshDirector.VerifyAndMock(
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(manifest),
					mockbosh.Deploy().WithManifest(manifest).WithAnyContextID().RedirectsToTask(taskID),
				)

				updateResp = updateServiceInstanceRequest(updateArbParams, instanceID, postDeployErrandPlanID, postDeployErrandPlanID)
				Expect(updateResp.StatusCode).To(Equal(http.StatusAccepted))

				operationData := operationDataFromUpdateResponse(updateResp)
				Expect(operationData.BoshContextID).NotTo(BeEmpty())
				Expect(*operationData).To(Equal(broker.OperationData{
					OperationType:        broker.OperationTypeUpdate,
					BoshTaskID:           taskID,
					BoshContextID:        operationData.BoshContextID,
					PostDeployErrandName: "health-check",
				}))
			})
		})
	})
})

func reportGenericFailureFor(instanceID string) types.GomegaMatcher {
	return SatisfyAll(
		Not(ContainSubstring("task-id:")),
		ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information: "),
		MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
		ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
		ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
		ContainSubstring("operation: update"),
	)
}

func descriptionFrom(resp *http.Response) string {
	Expect(resp).NotTo(BeNil())

	var body brokerapi.ErrorResponse
	defer resp.Body.Close()
	err := json.NewDecoder(resp.Body).Decode(&body)
	Expect(err).ToNot(HaveOccurred())

	return body.Description
}

func operationDataFromUpdateResponse(resp *http.Response) *broker.OperationData {
	Expect(resp).NotTo(BeNil())

	var updateResponse brokerapi.UpdateResponse
	defer resp.Body.Close()
	err := json.NewDecoder(resp.Body).Decode(&updateResponse)
	Expect(err).NotTo(HaveOccurred())

	var operationData broker.OperationData
	err = json.NewDecoder(strings.NewReader(updateResponse.OperationData)).Decode(&operationData)
	Expect(err).NotTo(HaveOccurred())

	return &operationData
}

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
