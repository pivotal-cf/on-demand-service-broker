// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_tests

import (
	"fmt"
	"os/exec"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/boshclient"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"gopkg.in/yaml.v2"
)

var _ = Describe("running the tool to upgrade all service instances", func() {
	const (
		UpgradesCompleted             = "Number of successful upgrades: %d"
		OrphanNumberOfInstances       = "Number of CF service instance orphans detected: %d"
		InstancesDeletedBeforeUpgrade = "Number of deleted instances before upgrade could occur: %d"
		instanceID                    = "an-instance-to-be-upgraded"
	)

	var (
		boshDirector    *mockhttp.Server
		cfAPI           *mockhttp.Server
		runningBroker   *gexec.Session
		boshUAA         *mockuaa.ClientCredentialsServer
		cfUAA           *mockuaa.ClientCredentialsServer
		upgradingTaskID = 20012
		manifest        = []byte(rawManifestWithDeploymentName(instanceID))
		runningTool     *gexec.Session
	)

	BeforeEach(func() {
		boshDirector = mockbosh.New()
		cfAPI = mockcfapi.New()
		boshUAA = mockuaa.NewClientCredentialsServer(boshClientID, boshClientSecret, "bosh uaa token")
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")
		boshDirector.ExpectedAuthorizationHeader(boshUAA.ExpectedAuthorizationHeader())
		adapter.DashboardUrlGenerator().NotImplemented()
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

	Context("when there are no service instances", func() {
		BeforeEach(func() {
			runningBroker = startBrokerWithPassingStartupChecks(defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL), cfAPI, boshDirector)

			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithNoServiceInstances(),
			)

			params := []string{
				"-brokerUsername", brokerUsername,
				"-brokerPassword", brokerPassword,
				"-brokerUrl", fmt.Sprintf("http://localhost:%d", brokerPort),
				"-pollingInterval", "1",
			}
			cmd := exec.Command(upgradeToolPath, params...)
			var err error
			runningTool, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		It("prints that it upgraded zero instances", func() {
			Eventually(runningTool, 10*time.Second).Should(gbytes.Say(UpgradesCompleted, 0))
		})

		It("exits with success", func() {
			Eventually(runningTool, 10*time.Second).Should(gexec.Exit(0))
		})
	})

	Context("when there is a service with a plan", func() {
		BeforeEach(func() {
			cfAPI.VerifyAndMock(
				mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
				mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(), // skip startup check

				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
			)

			boshDirector.VerifyAndMock(
				mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(), // startup check
			)
		})

		JustBeforeEach(func() {
			runningBroker = startBroker(defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL))

			params := []string{
				"-brokerUsername", brokerUsername,
				"-brokerPassword", brokerPassword,
				"-brokerUrl", fmt.Sprintf("http://localhost:%d", brokerPort),
				"-pollingInterval", "1",
			}
			cmd := exec.Command(upgradeToolPath, params...)
			var err error
			runningTool, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(runningTool, 10*time.Second).Should(gexec.Exit())
		})

		Context("which has one service instance", func() {
			BeforeEach(func() {
				cfAPI.AppendMocks(
					mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithServiceInstances(instanceID),
				)
			})

			Context("and the previous deployment has finished deploying", func() {
				var boshManifest = bosh.BoshManifest{}

				BeforeEach(func() {
					err := yaml.Unmarshal(manifest, &boshManifest)
					Expect(err).NotTo(HaveOccurred())

					adapter.GenerateManifest().ToReturnManifest(string(manifest))

					cfAPI.AppendMocks(
						mockcfapi.GetServiceInstance(instanceID).RespondsWith(getServiceInstanceResponse(
							"some-cc-plan-guid",
							"succeeded",
						)),
						mockcfapi.GetServicePlan("some-cc-plan-guid").RespondsWith(getServicePlanResponse(dedicatedPlanID)),
					)
				})

				Describe("the new manifest that is deployed", func() {
					BeforeEach(func() {
						boshDirector.AppendMocks(
							mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWith(manifest),
							mockbosh.Tasks(deploymentName(instanceID)).
								RespondsWithATaskContainingState(boshclient.BoshTaskDone, "it's a task"),

							mockbosh.Deploy().WithManifest(boshManifest).RedirectsToTask(upgradingTaskID),
							mockbosh.Task(upgradingTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskProcessing),
							mockbosh.Task(upgradingTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskProcessing),
							mockbosh.Task(upgradingTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskDone),
						)
					})

					It("prints details about starting the instance upgrade", func() {
						Expect(runningTool).To(gbytes.Say(
							"Service instance: %s, upgrade attempt starting \\(1 of 1\\)", instanceID,
						))
						Expect(runningTool).To(gbytes.Say("Result: accepted upgrade"))
					})

					It("prints that it upgraded one instance", func() {
						Expect(runningTool).To(gbytes.Say(UpgradesCompleted, 1))
					})

					It("calls the adapter with deployment info", func() {
						Expect(adapter.GenerateManifest().ReceivedDeployment()).To(Equal(
							serviceadapter.ServiceDeployment{
								DeploymentName: deploymentName(instanceID),
								Releases: serviceadapter.ServiceReleases{
									{
										Name:    serviceReleaseName,
										Version: serviceReleaseVersion,
										Jobs:    []string{"job-name"},
									},
								},
								Stemcell: serviceadapter.Stemcell{
									OS:      stemcellOS,
									Version: stemcellVersion,
								},
							}))
					})

					It("calls the adapter with the correct plan", func() {
						Expect(adapter.GenerateManifest().ReceivedPlan()).To(Equal(serviceadapter.Plan{
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
									AZs:                dedicatedPlanAZs,
								},
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

					It("calls the adapter with the correct request params", func() {
						Expect(adapter.GenerateManifest().ReceivedRequestParams()).To(BeNil())
					})

					It("calls the adapter with the correct previous manifest", func() {
						Expect(adapter.GenerateManifest().ReceivedPreviousManifest()).NotTo(BeNil())
						Expect(*adapter.GenerateManifest().ReceivedPreviousManifest()).To(Equal(boshManifest))
					})

					It("calls the adapter with the correct previous plan", func() {
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
				})

				Context("when the manifest generation fails with only an operator error", func() {
					BeforeEach(func() {
						adapter.GenerateManifest().ToFailWithOperatorError("adapter can't generate manifest")
						boshDirector.AppendMocks(
							mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWith(manifest),
							mockbosh.Tasks(deploymentName(instanceID)).
								RespondsWithATaskContainingState(
									boshclient.BoshTaskDone,
									"it's a task",
								).For("the last tasks before the upgrade has already finished, so the upgrade can go ahead"),
						)
					})

					It("logs a generic user error message", func() {
						Eventually(runningTool, 10*time.Second).Should(gexec.Exit(1))
						Expect(runningTool).Should(
							gbytes.Say(
								fmt.Sprintf("There was a problem completing your request. Please contact your operations team providing the following information: service: %s, service-instance-guid: %s",
									serviceName,
									instanceID,
								),
							),
						)
					})
				})

				Context("when the manifest generation fails with an errors for the CF user and operator", func() {
					BeforeEach(func() {
						adapter.GenerateManifest().ToFailWithCFUserAndOperatorError("error for cf user", "error for operator")
						boshDirector.AppendMocks(
							mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWith(manifest),
							mockbosh.Tasks(deploymentName(instanceID)).
								RespondsWithATaskContainingState(
									boshclient.BoshTaskDone,
									"it's a task",
								).For("the last tasks before the upgrade has already finished, so the upgrade can go ahead"),
						)
					})

					It("logs the cf user error message", func() {
						Eventually(runningTool, 10*time.Second).Should(gexec.Exit(1))
						Expect(runningTool).Should(gbytes.Say("error for cf user"))
					})
				})

				Context("when the bosh deploy command fails", func() {
					BeforeEach(func() {
						boshDirector.AppendMocks(
							mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWith(manifest),
							mockbosh.Tasks(deploymentName(instanceID)).
								RespondsWithATaskContainingState(boshclient.BoshTaskDone, "it's a task"),
							mockbosh.Deploy().Fails("did not work"),
						)
					})

					It("prints an error message", func() {
						Eventually(runningTool, 10*time.Second).Should(gbytes.Say("Upgrade failed for service instance"))
					})

					It("exits with error", func() {
						Eventually(runningTool, 10*time.Second).Should(gexec.Exit(1))
					})
				})

				Context("when the bosh task fails", func() {
					BeforeEach(func() {
						boshDirector.AppendMocks(
							mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWith(manifest),
							mockbosh.Tasks(deploymentName(instanceID)).
								RespondsWithATaskContainingState(
									boshclient.BoshTaskDone,
									"it's a task",
								).For("the last tasks before the upgrade has already finished, so the upgrade can go ahead"),
							mockbosh.Deploy().RedirectsToTask(upgradingTaskID),
							mockbosh.Task(upgradingTaskID).RespondsWithJson(boshclient.BoshTask{ID: upgradingTaskID, State: boshclient.BoshTaskError}),
						)
					})

					It("prints an error message", func() {
						Eventually(runningTool, 10*time.Second).Should(gbytes.Say("Upgrade failed for service instance %s: bosh task id %d", instanceID, upgradingTaskID))
					})

					It("exits with failure", func() {
						Eventually(runningTool, 10*time.Second).Should(gexec.Exit(1))
					})
				})
			})

			Context("when the bosh deployment cannot start, because of existing deployments", func() {
				BeforeEach(func() {
					cfMockCalls := []mockhttp.MockedResponseBuilder{}
					for i := 0; i < 4; i++ {
						cfMockCalls = append(
							cfMockCalls,
							mockcfapi.GetServiceInstance(instanceID).RespondsWith(getServiceInstanceResponse("some-cc-plan-guid", "succeeded")),
							mockcfapi.GetServicePlan("some-cc-plan-guid").RespondsWith(getServicePlanResponse(dedicatedPlanID)),
						)
					}
					cfAPI.AppendMocks(cfMockCalls...)

					adapter.GenerateManifest().ToReturnManifest(string(manifest))

					upgradeTaskID := 12
					boshDirector.AppendMocks(
						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWith(manifest),
						mockbosh.Tasks(deploymentName(instanceID)).
							RespondsWithATaskContainingState(boshclient.BoshTaskProcessing, "it's a task"),

						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWith(manifest),
						mockbosh.Tasks(deploymentName(instanceID)).
							RespondsWithATaskContainingState(boshclient.BoshTaskProcessing, "it's a task"),

						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWith(manifest),
						mockbosh.Tasks(deploymentName(instanceID)).
							RespondsWithATaskContainingState(boshclient.BoshTaskProcessing, "it's a task"),

						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWith(manifest),
						mockbosh.Tasks(deploymentName(instanceID)).
							RespondsWithATaskContainingState(boshclient.BoshTaskDone, "it's a task"),

						mockbosh.Deploy().RedirectsToTask(upgradeTaskID),
						mockbosh.Task(upgradeTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskDone),
					)
				})

				It("prints a success message", func() {
					Eventually(runningTool, 10*time.Second).Should(gbytes.Say(UpgradesCompleted, 1))
				})

				It("exits with success", func() {
					Eventually(runningTool, 10*time.Second).Should(gexec.Exit(0))
				})
			})

			Context("when the service instance has an operation in progress, according to Cloud Controller", func() {
				BeforeEach(func() {
					cfMockCalls := []mockhttp.MockedResponseBuilder{}
					for i := 0; i < 3; i++ {
						cfMockCalls = append(
							cfMockCalls,
							mockcfapi.GetServiceInstance(instanceID).RespondsWith(getServiceInstanceResponse("some-cc-plan-guid", "in progress")),
							mockcfapi.GetServicePlan("some-cc-plan-guid").RespondsWith(getServicePlanResponse(dedicatedPlanID)),
						)
					}
					cfMockCalls = append(
						cfMockCalls,
						mockcfapi.GetServiceInstance(instanceID).RespondsWith(getServiceInstanceResponse("some-cc-plan-guid", "succeeded")),
						mockcfapi.GetServicePlan("some-cc-plan-guid").RespondsWith(getServicePlanResponse(dedicatedPlanID)),
					)
					cfAPI.AppendMocks(cfMockCalls...)

					adapter.GenerateManifest().ToReturnManifest(string(manifest))

					upgradeTaskID := 12
					boshDirector.AppendMocks(
						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWith(manifest),
						mockbosh.Tasks(deploymentName(instanceID)).
							RespondsWithATaskContainingState(boshclient.BoshTaskDone, "it's a task"),
						mockbosh.Deploy().RedirectsToTask(upgradeTaskID),
						mockbosh.Task(upgradeTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskDone),
					)
				})

				It("prints a success message", func() {
					Eventually(runningTool, 10*time.Second).Should(gbytes.Say(UpgradesCompleted, 1))
				})

				It("exits with success", func() {
					Eventually(runningTool, 10*time.Second).Should(gexec.Exit(0))
				})
			})

			Context("when the instance has been deleted from CF, ignore it", func() {
				BeforeEach(func() {
					cfAPI.AppendMocks(
						mockcfapi.GetServiceInstance(instanceID).NotFound(),
					)
				})

				JustBeforeEach(func() {
					Eventually(runningTool, 10*time.Second).Should(gexec.Exit(0))
				})

				It("does not upgrade any instances", func() {
					Eventually(runningTool, 10*time.Second).Should(gbytes.Say(InstancesDeletedBeforeUpgrade, 1))
				})
			})

			Context("when the existing instance is still deploying", func() {
				BeforeEach(func() {
					adapter.GenerateManifest().ToReturnManifest(string(manifest))

					cfMockCalls := []mockhttp.MockedResponseBuilder{}
					for i := 0; i < 3; i++ {
						cfMockCalls = append(
							cfMockCalls,
							mockcfapi.GetServiceInstance(instanceID).RespondsWith(getServiceInstanceResponse("some-cc-plan-guid", "succeeded")),
							mockcfapi.GetServicePlan("some-cc-plan-guid").RespondsWith(getServicePlanResponse(dedicatedPlanID)),
						)
					}
					cfAPI.AppendMocks(cfMockCalls...)

					boshDirector.AppendMocks(
						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(bosh.BoshManifest{}),
						mockbosh.Tasks(deploymentName(instanceID)).
							RespondsWithATaskContainingState(boshclient.BoshTaskProcessing, "it's a task"),

						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(bosh.BoshManifest{}),
						mockbosh.Tasks(deploymentName(instanceID)).
							RespondsWithATaskContainingState(boshclient.BoshTaskProcessing, "it's a task"),

						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithManifest(bosh.BoshManifest{}),
						mockbosh.Tasks(deploymentName(instanceID)).
							RespondsWithATaskContainingState(boshclient.BoshTaskDone, "it's a task"),

						mockbosh.Deploy().RedirectsToTask(upgradingTaskID),
						mockbosh.Task(upgradingTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskDone),
					)
				})

				It("prints a success message", func() {
					Eventually(runningTool, 10*time.Second).Should(gbytes.Say(UpgradesCompleted, 1))
				})

				It("exits with success", func() {
					Eventually(runningTool, 10*time.Second).Should(gexec.Exit(0))
				})
			})

			Context("when the bosh deployment is not found", func() {
				BeforeEach(func() {
					cfAPI.AppendMocks(
						mockcfapi.GetServiceInstance(instanceID).RespondsWith(getServiceInstanceResponse("some-cc-plan-guid", "succeeded")),
						mockcfapi.GetServicePlan("some-cc-plan-guid").RespondsWith(getServicePlanResponse(dedicatedPlanID)),
					)

					boshDirector.AppendMocks(
						mockbosh.GetDeployment(deploymentName(instanceID)).NotFound(),
					)
				})

				It("prints a warning message", func() {
					Eventually(runningTool, 10*time.Second).Should(gbytes.Say(fmt.Sprintf(OrphanNumberOfInstances, 1)))
				})

				It("exits with success", func() {
					Eventually(runningTool, 10*time.Second).Should(gexec.Exit(0))
				})
			})
		})

		Context("which has two instances", func() {
			const secondInstanceID = "another-instance"
			const secondTaskID = 10012
			secondManifest := []byte(rawManifestWithDeploymentName(secondInstanceID))

			Context("and the instances are across multiple pages returned by CF", func() {
				BeforeEach(func() {

					adapter.GenerateManifest().ToReturnManifests(map[string]string{
						deploymentName(instanceID):       string(manifest),
						deploymentName(secondInstanceID): string(secondManifest),
					})

					cfAPI.AppendMocks(
						mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithPaginatedServiceInstances(
							"some-cc-plan-guid",
							1,
							100, //Match constant in implementation
							2,
							instanceID,
						),
						mockcfapi.ListServiceInstancesForPage("some-cc-plan-guid", 2).RespondsWithPaginatedServiceInstances(
							"some-cc-plan-guid",
							2,
							100, //Match constant in implementation
							2,
							secondInstanceID,
						),
						mockcfapi.GetServiceInstance(instanceID).RespondsWith(getServiceInstanceResponse("some-cc-plan-guid", "succeeded")),
						mockcfapi.GetServicePlan("some-cc-plan-guid").RespondsWith(getServicePlanResponse(dedicatedPlanID)),
						mockcfapi.GetServiceInstance(secondInstanceID).RespondsWith(getServiceInstanceResponse("some-cc-plan-guid", "succeeded")),
						mockcfapi.GetServicePlan("some-cc-plan-guid").RespondsWith(getServicePlanResponse(dedicatedPlanID)),
					)

					boshDirector.AppendMocks(
						mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWith(manifest),
						mockbosh.Tasks(deploymentName(instanceID)).
							RespondsWithATaskContainingState(boshclient.BoshTaskDone, "it's a task"),

						mockbosh.Deploy().RedirectsToTask(upgradingTaskID),
						mockbosh.Task(upgradingTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskProcessing),
						mockbosh.Task(upgradingTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskProcessing),
						mockbosh.Task(upgradingTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskDone),

						mockbosh.GetDeployment(deploymentName(secondInstanceID)).RespondsWith(manifest),
						mockbosh.Tasks(deploymentName(secondInstanceID)).
							RespondsWithATaskContainingState(boshclient.BoshTaskDone, "another task"),

						mockbosh.Deploy().RedirectsToTask(secondTaskID),
						mockbosh.Task(secondTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskProcessing),
						mockbosh.Task(secondTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskProcessing),
						mockbosh.Task(secondTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskDone),
					)
				})

				It("calls the expected CF and BOSH endpoints", func() {
					Eventually(runningTool, 10*time.Second).Should(gexec.Exit(0))
					cfAPI.VerifyMocks()
					boshDirector.VerifyMocks()
				})
			})

			Context("when one of the instances does not exist in bosh", func() {
				BeforeEach(func() {
					adapter.GenerateManifest().ToReturnManifest(rawManifestWithDeploymentName(secondInstanceID))

					cfAPI.AppendMocks(
						mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithServiceInstances(instanceID, secondInstanceID),

						mockcfapi.GetServiceInstance(instanceID).RespondsWith(getServiceInstanceResponse("some-cc-plan-guid", "succeeded")),
						mockcfapi.GetServicePlan("some-cc-plan-guid").RespondsWith(getServicePlanResponse(dedicatedPlanID)),
						mockcfapi.GetServiceInstance(secondInstanceID).RespondsWith(getServiceInstanceResponse("some-cc-plan-guid", "succeeded")),
						mockcfapi.GetServicePlan("some-cc-plan-guid").RespondsWith(getServicePlanResponse(dedicatedPlanID)),
					)

					boshDirector.AppendMocks(
						mockbosh.GetDeployment(deploymentName(instanceID)).NotFound(),

						mockbosh.GetDeployment(deploymentName(secondInstanceID)).RespondsWith(manifest),
						mockbosh.Tasks(deploymentName(secondInstanceID)).
							RespondsWithATaskContainingState(
								boshclient.BoshTaskDone,
								"it's a task",
							).For("the pre-upgrade check"),
						mockbosh.Deploy().RedirectsToTask(upgradingTaskID),
						mockbosh.Task(upgradingTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskDone),
					)
				})

				It("prints the number of orhpaned instances ", func() {
					Eventually(runningTool, 10*time.Second).Should(gbytes.Say(fmt.Sprintf(OrphanNumberOfInstances, 1)))
				})

				It("deploys the existing instance", func() {
					Expect(adapter.GenerateManifest().ReceivedDeployment().DeploymentName).To(Equal(deploymentName(secondInstanceID)))
				})
			})

		})

	})

	Context("when the plan has a post-deploy errand", func() {
		errandTaskID := 12345
		errandName := "some-errand"
		postDeployErrandPlanID := "post-deploy-errand-id"
		var boshManifest = bosh.BoshManifest{}

		BeforeEach(func() {
			err := yaml.Unmarshal(manifest, &boshManifest)
			Expect(err).NotTo(HaveOccurred())

			adapter.GenerateManifest().ToReturnManifest(string(manifest))

			cfAPI.VerifyAndMock(
				mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
				mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(), // skip startup check

				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(postDeployErrandPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithServiceInstances(instanceID),

				mockcfapi.GetServiceInstance(instanceID).RespondsWith(getServiceInstanceResponse("some-cc-plan-guid", "succeeded")),
				mockcfapi.GetServicePlan("some-cc-plan-guid").RespondsWith(getServicePlanResponse(postDeployErrandPlanID)),
			)
		})

		It("runs the post deploy errand on upgrade", func() {
			inProgressPostDeploy := boshclient.BoshTasks{
				{ID: 2002, State: boshclient.BoshTaskProcessing},
				{ID: 2001, State: boshclient.BoshTaskDone},
			}
			completedPostDeploy := boshclient.BoshTasks{
				{ID: 2002, State: boshclient.BoshTaskDone},
				{ID: 2001, State: boshclient.BoshTaskDone},
			}

			boshDirector.VerifyAndMock(
				mockbosh.Info().RespondsWithSufficientVersionForLifecycleErrands(), // startup check

				mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWith(manifest),
				mockbosh.Tasks(deploymentName(instanceID)).
					RespondsWithATaskContainingState(boshclient.BoshTaskDone, "it's a task"),

				mockbosh.Deploy().WithManifest(boshManifest).WithAnyContextID().RedirectsToTask(upgradingTaskID),
				mockbosh.TasksByAnyContext(deploymentName(instanceID)).
					RespondsWithATaskContainingState(boshclient.BoshTaskProcessing, "upgrade in progress"),
				mockbosh.TasksByAnyContext(deploymentName(instanceID)).
					RespondsWithATaskContainingState(boshclient.BoshTaskDone, "upgrade done"),
				mockbosh.TaskOutput(0).RespondsWithTaskOutput([]boshclient.BoshTaskOutput{}),

				mockbosh.Errand(deploymentName(instanceID), errandName).WithAnyContextID().RedirectsToTask(errandTaskID),
				mockbosh.Task(errandTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskProcessing),
				mockbosh.TasksByAnyContext(deploymentName(instanceID)).RespondsWithJson(inProgressPostDeploy),
				mockbosh.TaskOutput(2001).RespondsWithTaskOutput([]boshclient.BoshTaskOutput{}),
				mockbosh.TasksByAnyContext(deploymentName(instanceID)).RespondsWithJson(completedPostDeploy),
				mockbosh.TaskOutput(2002).RespondsWithTaskOutput([]boshclient.BoshTaskOutput{}),
				mockbosh.TaskOutput(2001).RespondsWithTaskOutput([]boshclient.BoshTaskOutput{}),
			)

			brokerConfig := defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)

			planWithPostDeploy := config.Plan{
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
					PostDeploy: errandName,
				},
			}

			brokerConfig.ServiceCatalog.Plans = []config.Plan{planWithPostDeploy}

			runningBroker = startBroker(brokerConfig)

			params := []string{"-brokerUsername", brokerUsername, "-brokerPassword", brokerPassword, "-brokerUrl", fmt.Sprintf("http://localhost:%d", brokerPort), "-pollingInterval", "1"}
			cmd := exec.Command(upgradeToolPath, params...)
			var err error
			runningTool, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(runningTool, 10*time.Second).Should(gexec.Exit(0))
		})
	})

	Context("when the upgrade tool fails", func() {
		BeforeEach(func() {
			runningBroker = startBrokerWithPassingStartupChecks(defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL), cfAPI, boshDirector)
		})

		Context("with all flags", func() {
			var (
				currentBrokerUsername  string
				currentBrokerPassword  string
				currentBrokerUrl       string
				currentPollingInterval string
			)

			BeforeEach(func() {
				currentBrokerPassword = brokerPassword
				currentBrokerUsername = brokerUsername
				currentPollingInterval = "1"
				currentBrokerUrl = fmt.Sprintf("http://localhost:%d", brokerPort)
			})

			JustBeforeEach(func() {
				params := []string{"-brokerUsername", currentBrokerUsername, "-brokerPassword", currentBrokerPassword, "-brokerUrl", currentBrokerUrl, "-pollingInterval", currentPollingInterval}
				cmd := exec.Command(upgradeToolPath, params...)
				var err error
				runningTool, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(runningTool).Should(gexec.Exit(1))
			})

			Context("because the broker credentials are wrong", func() {
				BeforeEach(func() {
					currentBrokerPassword = "not the " + brokerPassword
				})

				It("prints an error message", func() {
					Eventually(runningTool).Should(gbytes.Say("error listing service instances: HTTP response status: 401 Unauthorized"))
				})
			})

			Context("because brokerUsername is blank", func() {
				BeforeEach(func() {
					currentBrokerUsername = ""
				})

				It("prints an error message", func() {
					Eventually(runningTool).Should(gbytes.Say(`the brokerUsername, brokerPassword and brokerUrl are required to function`))
				})
			})

			Context("because brokerPassword is blank", func() {
				BeforeEach(func() {
					currentBrokerPassword = ""
				})

				It("prints an error message", func() {
					Eventually(runningTool).Should(gbytes.Say(`the brokerUsername, brokerPassword and brokerUrl are required to function`))
				})
			})

			Context("because brokerUrl is blank", func() {
				BeforeEach(func() {
					currentBrokerUrl = ""
				})

				It("prints an error message", func() {
					Eventually(runningTool).Should(gbytes.Say(`the brokerUsername, brokerPassword and brokerUrl are required to function`))
				})
			})

			Context("because pollingInterval is zero", func() {
				BeforeEach(func() {
					currentPollingInterval = "0"
				})

				It("prints an error message", func() {
					Eventually(runningTool).Should(gbytes.Say("the pollingInterval must be greater than zero"))
				})
			})

			Context("because pollingInterval is negative", func() {
				BeforeEach(func() {
					currentPollingInterval = "-123"
				})

				It("prints an error message", func() {
					Eventually(runningTool).Should(gbytes.Say("the pollingInterval must be greater than zero"))
				})
			})
		})

		Context("when flags are not given", func() {
			var params []string

			JustBeforeEach(func() {
				cmd := exec.Command(upgradeToolPath, params...)
				var err error
				runningTool, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(runningTool).Should(gexec.Exit(1))
			})

			Context("because brokerUsername is not given", func() {
				BeforeEach(func() {
					params = []string{"-brokerPassword", "bar", "-brokerUrl", "bar", "-pollingInterval", "1"}
				})

				It("prints an error message", func() {
					Eventually(runningTool).Should(gbytes.Say("the brokerUsername, brokerPassword and brokerUrl are required to function"))
				})
			})

			Context("because brokerPassword is not given", func() {
				BeforeEach(func() {
					params = []string{"-brokerUsername", "bar", "-brokerUrl", "bar", "-pollingInterval", "1"}
				})

				It("prints an error message", func() {
					Eventually(runningTool).Should(gbytes.Say("the brokerUsername, brokerPassword and brokerUrl are required to function"))
				})
			})

			Context("because brokerUrl is not given", func() {
				BeforeEach(func() {
					params = []string{"-brokerUsername", "bar", "-brokerPassword", "bar", "-pollingInterval", "1"}
				})

				It("prints an error message", func() {
					Eventually(runningTool).Should(gbytes.Say("the brokerUsername, brokerPassword and brokerUrl are required to function"))
				})
			})

			Context("because pollingInterval is not given", func() {
				BeforeEach(func() {
					params = []string{"-brokerUsername", "bar", "-brokerPassword", "bar", "-brokerUrl", "bar"}
				})

				It("prints an error message", func() {
					Eventually(runningTool).Should(gbytes.Say("the pollingInterval must be greater than zero"))
				})
			})
		})
	})

	Context("when the upgrade fails", func() {
		const (
			firstUpgradingTaskID  = 444
			secondUpgradingTaskId = 456
		)

		BeforeEach(func() {
			params := []string{"-brokerUsername", brokerUsername, "-brokerPassword", brokerPassword, "-brokerUrl", fmt.Sprintf("http://localhost:%d", brokerPort), "-pollingInterval", "1"}
			adapter.GenerateManifest().ToReturnManifest(string(manifest))

			runningBroker = startBrokerWithPassingStartupChecks(defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL), cfAPI, boshDirector)

			By("upgrading once and failing")
			boshDirector.VerifyAndMock(
				mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWith(manifest),
				mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
				mockbosh.Deploy().RedirectsToTask(firstUpgradingTaskID),
				mockbosh.Task(firstUpgradingTaskID).RespondsWithTaskContainingState(boshclient.BoshTaskError),
			)
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithServiceInstances(instanceID),

				mockcfapi.GetServiceInstance(instanceID).RespondsWith(getServiceInstanceResponse("some-cc-plan-guid", "succeeded")),
				mockcfapi.GetServicePlan("some-cc-plan-guid").RespondsWith(getServicePlanResponse(dedicatedPlanID)),
			)

			firstCommand := exec.Command(upgradeToolPath, params...)
			firstRunningTool, err := gexec.Start(firstCommand, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(firstRunningTool, 10*time.Second).Should(gexec.Exit())
			Expect(firstRunningTool.ExitCode()).To(Equal(1))

			By("and trying to upgrade a second time")
			boshDirector.VerifyAndMock(
				mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWith(manifest),
				mockbosh.Tasks(deploymentName(instanceID)).
					RespondsWithATaskContainingState(boshclient.BoshTaskError, "it's a task"),
				mockbosh.Deploy().RedirectsToTask(secondUpgradingTaskId),
				mockbosh.Task(secondUpgradingTaskId).RespondsWithTaskContainingState(boshclient.BoshTaskDone),
			)
			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
				mockcfapi.ListServicePlans("some-cc-service-offering-guid").
					RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
				mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithServiceInstances(instanceID),

				mockcfapi.GetServiceInstance(instanceID).RespondsWith(getServiceInstanceResponse("some-cc-plan-guid", "succeeded")),
				mockcfapi.GetServicePlan("some-cc-plan-guid").RespondsWith(getServicePlanResponse(dedicatedPlanID)),
			)

			secondCommand := exec.Command(upgradeToolPath, params...)
			secondRunningTool, err := gexec.Start(secondCommand, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(secondRunningTool, 10*time.Second).Should(gexec.Exit())
			Expect(secondRunningTool.ExitCode()).To(Equal(0))
		})

		It("should redeploy the correct instance", func() {
			Expect(adapter.GenerateManifest().ReceivedDeployment().DeploymentName).To(Equal(deploymentName(instanceID)))
		})
	})
})
