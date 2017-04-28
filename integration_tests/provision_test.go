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

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"gopkg.in/yaml.v2"
)

func toYaml(obj interface{}) []byte {
	data, err := yaml.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	return data
}

var _ = Describe("provision service instance", func() {
	const (
		taskID     = 4
		instanceID = "first-deployment-instance-id"
	)

	var manifestForFirstDeployment = bosh.BoshManifest{
		Name:           deploymentName(instanceID),
		Releases:       []bosh.Release{},
		Stemcells:      []bosh.Stemcell{},
		InstanceGroups: []bosh.InstanceGroup{},
	}

	var (
		conf              config.Config
		runningBroker     *gexec.Session
		boshDirector      *mockhttp.Server
		cfAPI             *mockhttp.Server
		boshUAA           *mockuaa.ClientCredentialsServer
		cfUAA             *mockuaa.ClientCredentialsServer
		planID            string
		provisionResponse *http.Response
		arbitraryParams   map[string]interface{}
	)

	BeforeEach(func() {
		boshUAA = mockuaa.NewClientCredentialsServer(boshClientID, boshClientSecret, "bosh uaa token")
		boshDirector = mockbosh.New()
		boshDirector.ExpectedAuthorizationHeader(boshUAA.ExpectedAuthorizationHeader())
		cfAPI = mockcfapi.New()
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")
		conf = defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)
		planID = dedicatedPlanID
		adapter.DashboardUrlGenerator().NotImplemented()
		adapter.GenerateManifest().ToReturnManifest(string(toYaml(manifestForFirstDeployment)))
	})

	AfterEach(func() {
		if runningBroker != nil {
			killBrokerAndCheckForOpenConnections(runningBroker, boshDirector.URL)
		}
		boshDirector.VerifyMocks()
		boshDirector.Close()
		boshUAA.Close()

		cfAPI.VerifyMocks()
		cfAPI.Close()
		cfUAA.Close()
	})

	Context("when the plan has a quota", func() {
		JustBeforeEach(func() {
			runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)

			cfAPI.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsOKWith(listCFServiceOfferingsResponse(serviceID, "21f13659-278c-4fa9-a3d7-7fe737e52895")),
				mockcfapi.ListServicePlans("21f13659-278c-4fa9-a3d7-7fe737e52895").RespondsWithServicePlan(planID, "df717e7c-afd5-4d0a-bafe-16c7eff546ec"),
				mockcfapi.ListServiceInstances("df717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsOKWith(listCFServiceInstanceCountForPlanResponse(0)),
			)
			boshDirector.VerifyAndMock(
				mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""),
				mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
				mockbosh.Deploy().WithManifest(manifestForFirstDeployment).WithoutContextID().RedirectsToTask(taskID),
			)
			arbitraryParams = map[string]interface{}{"foo": "bar"}
			provisionResponse = provisionInstance(instanceID, planID, arbitraryParams)
		})

		It("responds with 202", func() {
			Expect(provisionResponse.StatusCode).To(Equal(http.StatusAccepted))
		})

		It("includes the operation data in the response", func() {
			body, err := ioutil.ReadAll(provisionResponse.Body)
			Expect(err).NotTo(HaveOccurred())

			var provisioningResponse brokerapi.ProvisioningResponse
			err = json.Unmarshal(body, &provisioningResponse)
			Expect(err).NotTo(HaveOccurred())

			var operationData broker.OperationData
			err = json.Unmarshal([]byte(provisioningResponse.OperationData), &operationData)
			Expect(err).NotTo(HaveOccurred())

			By("including the operation type")
			Expect(operationData.OperationType).To(Equal(broker.OperationTypeCreate))
			By("including the bosh task ID")
			Expect(operationData.BoshTaskID).To(Equal(taskID))
			By("not including a context ID")
			Expect(operationData.BoshContextID).To(BeEmpty())
			By("not including the plan ID")
			Expect(operationData.PlanID).To(BeEmpty())
		})

		It("calls the adapter with deployment info", func() {
			Expect(adapter.GenerateManifest().ReceivedDeployment()).To(Equal(
				serviceadapter.ServiceDeployment{
					DeploymentName: deploymentName(instanceID),
					Releases: serviceadapter.ServiceReleases{{
						Name:    serviceReleaseName,
						Version: serviceReleaseVersion,
						Jobs:    []string{"job-name"},
					}},
					Stemcell: serviceadapter.Stemcell{
						OS:      stemcellOS,
						Version: stemcellVersion,
					},
				}),
			)
		})

		It("calls the adapter with the correct plan", func() {
			Expect(adapter.GenerateManifest().ReceivedPlan()).To(Equal(serviceadapter.Plan{
				Properties: serviceadapter.Properties{
					"type":            "dedicated-plan-property",
					"global_property": "global_value",
				},
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
				Update: dedicatedPlanUpdateBlock,
			}))
		})

		It("calls the adapter with the correct request params", func() {
			Expect(adapter.GenerateManifest().ReceivedRequestParams()).To(Equal(map[string]interface{}{
				"plan_id":           dedicatedPlanID,
				"service_id":        serviceID,
				"space_guid":        spaceGUID,
				"organization_guid": organizationGUID,
				"parameters":        arbitraryParams,
			}))
		})

		It("calls the adapter with the correct previous manifest", func() {
			Expect(adapter.GenerateManifest().ReceivedPreviousManifest()).To(BeNil())
		})

		It("logs the incoming request without a request ID", func() {
			Eventually(runningBroker).Should(gbytes.Say(`\[on-demand-service-broker\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6} Started PUT /v2/service_instances/first-deployment-instance-id`))
		})

		It("logs the requests made to CF API with a request ID", func() {
			cfRegexpString := logRegexpStringWithRequestIDCapture(`GET http://127.0.0.1:\d+/v2/services\?results-per-page=100`)
			Eventually(runningBroker).Should(gbytes.Say(cfRegexpString))
			requestID := firstMatchInOutput(runningBroker, cfRegexpString)
			Eventually(runningBroker).Should(gbytes.Say(requestID)) // It should use the same request ID again
		})

		It("logs the requests made to bosh with a request ID", func() {
			boshRegexpString := logRegexpStringWithRequestIDCapture(`getting manifest from bosh for deployment service-instance_first-deployment-instance-id`)
			Eventually(runningBroker).Should(gbytes.Say(boshRegexpString))
			requestID := firstMatchInOutput(runningBroker, boshRegexpString)
			Eventually(runningBroker).Should(gbytes.Say(requestID)) // It should use the same request ID again
		})

		It("logs the requests made to external service adapter with a request ID", func() {
			serviceAdapterRegexpString := logRegexpStringWithRequestIDCapture(`service adapter ran generate-manifest successfully`)
			Eventually(runningBroker).Should(gbytes.Say(serviceAdapterRegexpString))
			requestID := firstMatchInOutput(runningBroker, serviceAdapterRegexpString)
			Eventually(runningBroker).Should(gbytes.Say(requestID)) // It should use the same request ID again
		})

		It("logs requests with a the same requestID", func() {
			boshRegexpString := logRegexpStringWithRequestIDCapture(`getting manifest from bosh for deployment service-instance_first-deployment-instance-id`)
			Eventually(runningBroker).Should(gbytes.Say(boshRegexpString))
			requestID := firstMatchInOutput(runningBroker, boshRegexpString)
			cfRegexpString := logRegexpString(requestID, `GET http://127.0.0.1:\d+/v2/services\?results-per-page=100`)
			Eventually(runningBroker).Should(gbytes.Say(cfRegexpString))
			serviceAdapterRegexpString := logRegexpString(requestID, `service adapter ran generate-manifest successfully`)
			Eventually(runningBroker).Should(gbytes.Say(serviceAdapterRegexpString))
		})

		Context("when the adapter returns a dashboardUrl", func() {
			BeforeEach(func() {
				adapter.DashboardUrl().Returns(`{"dashboard_url": "http://google.com"}`)
			})

			It("includes the dashboard url in the response", func() {
				var provisionResponseBody brokerapi.ProvisioningResponse
				Expect(json.NewDecoder(provisionResponse.Body).Decode(&provisionResponseBody)).To(Succeed())
				Expect(provisionResponseBody.DashboardURL).To(Equal("http://google.com"))
			})

			It("calls the adapter with the instanceID", func() {
				Expect(adapter.DashboardUrl().ReceivedInstanceID()).To(Equal(instanceID))
			})

			It("calls the adapter with the plan", func() {
				Expect(adapter.DashboardUrl().ReceivedPlan()).To(Equal(serviceadapter.Plan{
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

			It("calls the adapter with the manifest", func() {
				Expect(adapter.DashboardUrl().ReceivedManifest()).To(Equal(manifestForFirstDeployment))
			})
		})

		Context("when the adapter fails to generate a dashboardURL", func() {
			Context("fails with operator error only", func() {
				BeforeEach(func() {
					adapter.DashboardUrl().FailsWithOperatorError("adapter completely failed")
				})

				It("responds with HTTP 500", func() {
					Expect(provisionResponse.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				Describe("error message", func() {
					var errorResponse brokerapi.ErrorResponse

					JustBeforeEach(func() {
						Expect(json.NewDecoder(provisionResponse.Body).Decode(&errorResponse)).To(Succeed())
					})

					AfterEach(func() {
						defer provisionResponse.Body.Close()
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
							"operation: create",
						))
					})

					It("includes the bosh task ID", func() {
						Expect(errorResponse.Description).To(ContainSubstring(
							fmt.Sprintf("task-id: %d", taskID),
						))
					})
				})
			})

			Context("fails with cf user and operator error", func() {
				BeforeEach(func() {
					adapter.DashboardUrl().FailsWithCFUserAndOperatorError("error message for user", "error message for operator")
				})

				It("responds with HTTP 500", func() {
					Expect(provisionResponse.StatusCode).To(Equal(500))
				})

				It("includes the user error message in the response", func() {
					defer provisionResponse.Body.Close()
					var errorResponse brokerapi.ErrorResponse
					Expect(json.NewDecoder(provisionResponse.Body).Decode(&errorResponse)).To(Succeed())
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

	Context("when the plan has no quota", func() {
		BeforeEach(func() {
			planID = highMemoryPlanID
			runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)

			boshDirector.VerifyAndMock(
				mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""),
				mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
				mockbosh.Deploy().WithManifest(manifestForFirstDeployment).WithoutContextID().RedirectsToTask(taskID),
			)

			provisionResponse = provisionInstance(instanceID, planID, arbitraryParams)
		})

		It("responds with 202", func() {
			Expect(provisionResponse.StatusCode).To(Equal(http.StatusAccepted))
		})
	})

	Context("when the plan has a post-deploy errand", func() {
		BeforeEach(func() {
			planID = "post-deploy-errand-id"

			postDeployErrandPlan := config.Plan{
				Name: "post-deploy-errand-plan",
				ID:   planID,
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

			runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)

			boshDirector.VerifyAndMock(
				mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""),
				mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
				mockbosh.Deploy().WithManifest(manifestForFirstDeployment).WithAnyContextID().RedirectsToTask(taskID),
			)

			provisionResponse = provisionInstance(instanceID, planID, arbitraryParams)
		})

		It("responds with 202", func() {
			Expect(provisionResponse.StatusCode).To(Equal(http.StatusAccepted))
		})

		It("includes the operation data in the response", func() {
			body, err := ioutil.ReadAll(provisionResponse.Body)
			Expect(err).NotTo(HaveOccurred())

			var provisioningResponse brokerapi.ProvisioningResponse
			err = json.Unmarshal(body, &provisioningResponse)
			Expect(err).NotTo(HaveOccurred())

			var operationData broker.OperationData
			err = json.Unmarshal([]byte(provisioningResponse.OperationData), &operationData)
			Expect(err).NotTo(HaveOccurred())

			By("including the operation type")
			Expect(operationData.OperationType).To(Equal(broker.OperationTypeCreate))
			By("including the bosh task ID")
			Expect(operationData.BoshTaskID).To(Equal(taskID))
			By("including a context ID")
			Expect(operationData.BoshContextID).NotTo(BeEmpty())
			By("including the post deploy errand name")
			Expect(operationData.PostDeployErrandName).To(Equal("health-check"))
		})
	})

	Describe("fails to provision", func() {
		Context("when another instance with the same id is provisioned", func() {
			BeforeEach(func() {
				runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)

				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsWithRawManifest([]byte(`a: b`)),
				)

				provisionResponse = provisionInstance(instanceID, planID, arbitraryParams)
			})

			It("responds with 409", func() {
				Expect(provisionResponse.StatusCode).To(Equal(http.StatusConflict))
			})

			It("logs that the instance already exists", func() {
				Eventually(runningBroker).Should(gbytes.Say("already exists"))
			})
		})

		Context("when the service adapter fails to generate a manifest", func() {
			JustBeforeEach(func() {
				runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)

				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsOKWith(listCFServiceOfferingsResponse(serviceID, "21f13659-278c-4fa9-a3d7-7fe737e52895")),
					mockcfapi.ListServicePlans("21f13659-278c-4fa9-a3d7-7fe737e52895").RespondsWithServicePlan(planID, "df717e7c-afd5-4d0a-bafe-16c7eff546ec"),
					mockcfapi.ListServiceInstances("df717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsOKWith(listCFServiceInstanceCountForPlanResponse(0)),
				)
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""),
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
				)

				provisionResponse = provisionInstance(instanceID, planID, arbitraryParams)
			})

			Context("when the adapter exits with 1", func() {
				Context("and the adapter does not output a message for the cf user", func() {
					BeforeEach(func() {
						adapter.GenerateManifest().ToFailWithOperatorError("something has gone wrong in adapter")
					})

					It("responds with HTTP 500", func() {
						Expect(provisionResponse.StatusCode).To(Equal(http.StatusInternalServerError))
					})

					Describe("error message", func() {
						var errorResponse brokerapi.ErrorResponse

						JustBeforeEach(func() {
							Expect(json.NewDecoder(provisionResponse.Body).Decode(&errorResponse)).To(Succeed())
						})

						AfterEach(func() {
							defer provisionResponse.Body.Close()
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
								"operation: create",
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
						Expect(provisionResponse.StatusCode).To(Equal(http.StatusInternalServerError))
					})

					It("includes the error message for the cf user in the response", func() {
						var errorResponse brokerapi.ErrorResponse
						Expect(json.NewDecoder(provisionResponse.Body).Decode(&errorResponse)).To(Succeed())
						Expect(errorResponse.Description).To(ContainSubstring("error message for cf user"))
					})

					It("logs both error messages", func() {
						Eventually(runningBroker.Out).Should(gbytes.Say("error message for cf user"))
						Eventually(runningBroker.Out).Should(gbytes.Say("error message for operator"))
					})
				})
			})

			Context("when the adapter exits with 10", func() {
				BeforeEach(func() {
					adapter.ManifestGenerator().NotImplemented()
				})

				It("responds with HTTP 500", func() {
					Expect(provisionResponse.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("logs that the adapter does not implement generate-manifest", func() {
					Eventually(runningBroker).Should(gbytes.Say("generate manifest: command not implemented by service adapter"))
				})
			})
		})

		Context("when the service adapter generates a manifest with the wrong deployment name", func() {
			BeforeEach(func() {
				adapter.GenerateManifest().ToReturnManifest(rawManifestWithDeploymentName("not-the-deployment-name-given-to-the-adapter"))
				runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)

				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsOKWith(listCFServiceOfferingsResponse(serviceID, "21f13659-278c-4fa9-a3d7-7fe737e52895")),
					mockcfapi.ListServicePlans("21f13659-278c-4fa9-a3d7-7fe737e52895").RespondsWithServicePlan(planID, "df717e7c-afd5-4d0a-bafe-16c7eff546ec"),
					mockcfapi.ListServiceInstances("df717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsOKWith(listCFServiceInstanceCountForPlanResponse(0)),
				)
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""),
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
				)
				provisionResponse = provisionInstance(instanceID, planID, arbitraryParams)
			})

			It("responds with HTTP 500", func() {
				Expect(provisionResponse.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("logs that the adapter has not returned the correct deployment name", func() {
				Eventually(runningBroker).Should(gbytes.Say("external service adapter generated manifest with an incorrect deployment name"))
			})
		})

		Context("when the service adapter generates a manifest an invalid release version", func() {
			BeforeEach(func() {
				adapter.GenerateManifest().ToReturnManifest(rawManifestInvalidReleaseVersion(instanceID))
				runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)

				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsOKWith(listCFServiceOfferingsResponse(serviceID, "21f13659-278c-4fa9-a3d7-7fe737e52895")),
					mockcfapi.ListServicePlans("21f13659-278c-4fa9-a3d7-7fe737e52895").RespondsWithServicePlan(planID, "df717e7c-afd5-4d0a-bafe-16c7eff546ec"),
					mockcfapi.ListServiceInstances("df717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsOKWith(listCFServiceInstanceCountForPlanResponse(0)),
				)
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""),
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
				)
				provisionResponse = provisionInstance(instanceID, planID, arbitraryParams)
			})

			It("responds with HTTP 500", func() {
				Expect(provisionResponse.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("logs that the adapter has not returned an exact version", func() {
				Eventually(runningBroker).Should(gbytes.Say("error: external service adapter generated manifest with an incorrect version"))
			})
		})

		Context("when the service adapter generates a manifest an invalid stemcell version", func() {
			BeforeEach(func() {
				adapter.GenerateManifest().ToReturnManifest(rawManifestInvalidStemcellVersion(instanceID))
				runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)

				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsOKWith(listCFServiceOfferingsResponse(serviceID, "21f13659-278c-4fa9-a3d7-7fe737e52895")),
					mockcfapi.ListServicePlans("21f13659-278c-4fa9-a3d7-7fe737e52895").RespondsWithServicePlan(planID, "df717e7c-afd5-4d0a-bafe-16c7eff546ec"),
					mockcfapi.ListServiceInstances("df717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsOKWith(listCFServiceInstanceCountForPlanResponse(0)),
				)
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""),
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
				)
				provisionResponse = provisionInstance(instanceID, planID, arbitraryParams)
			})

			It("responds with HTTP 500", func() {
				Expect(provisionResponse.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("logs that the adapter has not returned an exact version", func() {
				Eventually(runningBroker).Should(gbytes.Say("error: external service adapter generated manifest with an incorrect version"))
			})
		})

		Context("when the bosh deploy fails", func() {
			BeforeEach(func() {
				adapter.GenerateManifest().ToReturnManifest(string(toYaml(manifestForFirstDeployment)))
				runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)

				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsOKWith(listCFServiceOfferingsResponse(serviceID, "21f13659-278c-4fa9-a3d7-7fe737e52895")),
					mockcfapi.ListServicePlans("21f13659-278c-4fa9-a3d7-7fe737e52895").RespondsWithServicePlan(planID, "df717e7c-afd5-4d0a-bafe-16c7eff546ec"),
					mockcfapi.ListServiceInstances("df717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsOKWith(listCFServiceInstanceCountForPlanResponse(0)),
				)
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""),
					mockbosh.Tasks(deploymentName(instanceID)).RespondsWithNoTasks(),
					mockbosh.Deploy().WithoutContextID().RespondsInternalServerErrorWith("cannot deploy"),
				)
				provisionResponse = provisionInstance(instanceID, planID, arbitraryParams)
			})

			It("responds with HTTP 500", func() {
				Expect(provisionResponse.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			Describe("error message", func() {
				var errorResponse brokerapi.ErrorResponse

				JustBeforeEach(func() {
					Expect(json.NewDecoder(provisionResponse.Body).Decode(&errorResponse)).To(Succeed())
				})

				AfterEach(func() {
					defer provisionResponse.Body.Close()
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
						"operation: create",
					))
				})

				It("does NOT include the bosh task ID", func() {
					Expect(errorResponse.Description).NotTo(ContainSubstring(
						"task-id:",
					))
				})
			})
		})

		Context("when the plan quota is reached", func() {
			BeforeEach(func() {
				runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)

				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsOKWith(listCFServiceOfferingsResponse(serviceID, "21f13659-278c-4fa9-a3d7-7fe737e52895")),
					mockcfapi.ListServicePlans("21f13659-278c-4fa9-a3d7-7fe737e52895").RespondsWithServicePlan(planID, "df717e7c-afd5-4d0a-bafe-16c7eff546ec"),
					mockcfapi.ListServiceInstances("df717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsOKWith(listCFServiceInstanceCountForPlanResponse(dedicatedPlanQuota)),
				)
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""),
				)
				provisionResponse = provisionInstance(instanceID, planID, arbitraryParams)
			})

			It("responds with 500", func() {
				Expect(provisionResponse.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("includes a plan quota reached message in the response", func() {
				Expect(readJSONResponse(provisionResponse.Body)).To(Equal(map[string]string{"description": "The quota for this service plan has been exceeded. Please contact your Operator for help."}))
			})
		})

		Context("when the global quota is reached", func() {
			BeforeEach(func() {
				globalQuota := 1
				conf.ServiceCatalog.GlobalQuotas = config.Quotas{
					ServiceInstanceLimit: &globalQuota,
				}
				runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)

				listCFServiceOfferingsResponse := listCFServiceOfferingsResponse(
					serviceID,
					"21f13659-278c-4fa9-a3d7-7fe737e52895",
				)

				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsOKWith(listCFServiceOfferingsResponse),
					mockcfapi.ListServicePlans("21f13659-278c-4fa9-a3d7-7fe737e52895").RespondsWithServicePlans(
						mockcfapi.Plan{ID: planID, CloudControllerGUID: "df717e7c-afd5-4d0a-bafe-16c7eff546ec"},
						mockcfapi.Plan{ID: highMemoryPlanID, CloudControllerGUID: "297a5e27-480b-4be9-96e1-c1ae4e389539"},
					),
					mockcfapi.ListServiceInstances("df717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsOKWith(
						listCFServiceInstanceCountForPlanResponse(0)),
					mockcfapi.ListServiceInstances("297a5e27-480b-4be9-96e1-c1ae4e389539").RespondsOKWith(
						listCFServiceInstanceCountForPlanResponse(1)),
				)
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""),
				)

				provisionResponse = provisionInstance(instanceID, planID, arbitraryParams)
			})

			It("responds with 500", func() {
				Expect(provisionResponse.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("includes a quota reached message in the response", func() {
				Expect(readJSONResponse(provisionResponse.Body)).To(Equal(
					map[string]string{
						"description": "The quota for this service has been exceeded. Please contact your Operator for help.",
					},
				))
			})
		})

		Context("when the global quota is set to 0", func() {
			BeforeEach(func() {
				globalQuota := 0
				conf.ServiceCatalog.GlobalQuotas = config.Quotas{
					ServiceInstanceLimit: &globalQuota,
				}
				runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)

				listCFServiceOfferingsResponse := listCFServiceOfferingsResponse(
					serviceID,
					"21f13659-278c-4fa9-a3d7-7fe737e52895",
				)

				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsOKWith(listCFServiceOfferingsResponse),
					mockcfapi.ListServicePlans("21f13659-278c-4fa9-a3d7-7fe737e52895").
						RespondsWithNoServicePlans(),
				)
				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName(instanceID)).RespondsNotFoundWith(""),
				)

				provisionResponse = provisionInstance(instanceID, planID, arbitraryParams)
			})

			It("responds with 500", func() {
				Expect(provisionResponse.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("includes a quota reached message in the response", func() {
				Expect(readJSONResponse(provisionResponse.Body)).To(Equal(
					map[string]string{
						"description": "The quota for this service has been exceeded. Please contact your Operator for help.",
					},
				))
			})
		})

		Context("when the provision request does not have the async parameter", func() {
			BeforeEach(func() {
				runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)

				planID = dedicatedPlanID
				arbitraryParams = map[string]interface{}{}
				instanceID := "instanceID"

				provisionResponse = provisionInstanceSynchronously(instanceID, planID, arbitraryParams)
			})

			It("responds with 422, async required", func() {
				Expect(provisionResponse.StatusCode).To(Equal(422))
				Expect(ioutil.ReadAll(provisionResponse.Body)).To(MatchJSON(`{"error":"AsyncRequired","description":"This service plan requires client support for asynchronous service operations."}`))
			})
		})
	})

	Context("when the bosh director is unavailable", func() {
		BeforeEach(func() {
			runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)

			boshDirector.Close()
			adapter.GenerateManifest().ToReturnManifest(string(toYaml(manifestForFirstDeployment)))

			provisionResponse = provisionInstance(instanceID, planID, arbitraryParams)
		})

		It("responds with HTTP 500", func() {
			Expect(provisionResponse.StatusCode).To(Equal(http.StatusInternalServerError))
		})

		It("includes a try again later message in the response", func() {
			var errorResponse brokerapi.ErrorResponse
			Expect(json.NewDecoder(provisionResponse.Body).Decode(&errorResponse)).To(Succeed())
			Expect(errorResponse.Description).To(ContainSubstring("Currently unable to create service instance, please try again later"))
		})
	})
})
