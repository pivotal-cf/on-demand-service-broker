// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package on_demand_service_broker_test

import (
	"fmt"
	"net/http"

	"github.com/pivotal-cf/on-demand-service-broker/integration_tests/helpers"

	"github.com/onsi/gomega/ghttp"

	"encoding/json"

	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"github.com/pkg/errors"
)

var _ = Describe("Management API", func() {
	var (
		conf brokerConfig.Config
	)

	BeforeEach(func() {
		conf = brokerConfig.Config{
			Broker: brokerConfig.Broker{
				Port: serverPort, Username: brokerUsername, Password: brokerPassword,
			},
			ServiceCatalog: brokerConfig.ServiceOffering{
				Name: serviceName,
				Plans: brokerConfig.Plans{
					{
						Name:   dedicatedPlanName,
						ID:     dedicatedPlanID,
						Quotas: brokerConfig.Quotas{ServiceInstanceLimit: &dedicatedPlanQuota},
						LifecycleErrands: &sdk.LifecycleErrands{
							PostDeploy: []sdk.Errand{{
								Name:      "post-deploy-errand",
								Instances: []string{},
							}},
						},
					},
					{
						Name: highMemoryPlanName,
						ID:   highMemoryPlanID,
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		StartServer(conf)
	})

	Describe("GET /mgmt/service_instances", func() {
		const (
			serviceInstancesPath = "service_instances"
		)

		When("CF is set", func() {
			It("returns all service instances results", func() {
				fakeCfClient.GetServiceInstancesReturns([]cf.Instance{
					{
						GUID:         "service-instance-id",
						PlanUniqueID: "plan-id",
						SpaceGUID:    "space-id",
					},
					{
						GUID:         "another-service-instance-id",
						PlanUniqueID: "another-plan-id",
						SpaceGUID:    "space-id",
					},
				}, nil)

				response, bodyContent := doGetRequest(serviceInstancesPath)

				By("returning the correct status code")
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				By("returning the service instances")
				Expect(bodyContent).To(MatchJSON(
					`[
					{"service_instance_id": "service-instance-id", "plan_id":"plan-id", "space_guid":"space-id"},
					{"service_instance_id": "another-service-instance-id", "plan_id":"another-plan-id", "space_guid":"space-id"}
				]`,
				))
			})

			It("returns filtered service instances results", func() {
				fakeCfClient.GetServiceInstancesReturns([]cf.Instance{
					{
						GUID:         "service-instance-id",
						PlanUniqueID: "plan-id",
						SpaceGUID:    "space-id",
					},
				}, nil)

				response, bodyContent := doGetRequest(serviceInstancesPath + "?cf_org=banana&cf_space=banane")

				By("returning the correct status code")
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				By("returning the service instances")
				Expect(bodyContent).To(MatchJSON(
					`[{"service_instance_id": "service-instance-id", "plan_id":"plan-id", "space_guid":"space-id"}]`,
				))
				filter, _ := fakeCfClient.GetServiceInstancesArgsForCall(0)
				Expect(filter.OrgName).To(Equal("banana"))
				Expect(filter.SpaceName).To(Equal("banane"))
			})

			It("returns 500 when getting instances fails", func() {
				fakeCfClient.GetServiceInstancesReturns([]cf.Instance{}, errors.New("something failed"))

				response, _ := doGetRequest(serviceInstancesPath)

				By("returning the correct status code")
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

				By("logging the failure")
				Expect(loggerBuffer).To(gbytes.Say(`something failed`))
			})

			It("return 401 when not authorised", func() {

				response, _ := doGetRequestWithoutAuth(serviceInstancesPath)

				By("returning the correct status code")
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when SIAPI is set", func() {

			var (
				siApiServer             *ghttp.Server
				expectedResponse        string
				serviceInstancesHandler *helpers.FakeHandler
			)

			BeforeEach(func() {
				siApiServer = ghttp.NewServer()
				serviceInstancesHandler = new(helpers.FakeHandler)

				siApiServer.RouteToHandler(http.MethodGet, "/", ghttp.CombineHandlers(
					ghttp.VerifyBasicAuth("test-user", "test-password"),
					serviceInstancesHandler.Handle,
				))

				expectedResponse = `[{"plan_id": "service-plan-id", "service_instance_id": "my-instance-id-from-siApi"}]`

				serviceInstancesHandler.WithQueryParams().RespondsWith(http.StatusOK, expectedResponse)

				conf.ServiceInstancesAPI = brokerConfig.ServiceInstancesAPI{
					URL:        siApiServer.URL(),
					RootCACert: "",
					Authentication: brokerConfig.Authentication{
						Basic: brokerConfig.UserCredentials{
							Username: "test-user",
							Password: "test-password",
						},
					},
					DisableSSLCertVerification: true,
				}
			})

			It("returns all service instances results", func() {
				response, bodyContent := doGetRequest(serviceInstancesPath)

				By("returning the correct status code")
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				By("returning the service instances")
				Expect(bodyContent).To(MatchJSON(expectedResponse))
				Expect(fakeCfClient.GetServiceInstancesCallCount()).To(Equal(0))
			})

			It("returns filtered service instances results", func() {
				expectedResponse := `[{"plan_id": "other-plan", "service_instance_id": "other-id"}]`

				serviceInstancesHandler.WithQueryParams("foo=bar").RespondsWith(http.StatusOK, expectedResponse)

				response, bodyContent := doGetRequest(serviceInstancesPath + "?foo=bar")

				By("returning the correct status code")
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				By("returning the service instances")
				Expect(bodyContent).To(MatchJSON(expectedResponse))
				Expect(fakeCfClient.GetServiceInstancesCallCount()).To(Equal(0))
			})

			AfterEach(func() {
				siApiServer.Close()
			})
		})
	})

	Describe("GET /mgmt/orphan_deployments", func() {
		const (
			orphanDeploymentsPath = "orphan_deployments"
		)

		It("responds with the orphan deployments", func() {
			fakeCfClient.GetServiceInstancesReturns([]cf.Instance{
				{
					GUID:         "not-orphan",
					PlanUniqueID: "plan-id",
				},
			}, nil)
			fakeBoshClient.GetDeploymentsReturns([]boshdirector.Deployment{
				{Name: "service-instance_not-orphan"},
				{Name: "service-instance_orphan"},
			}, nil)

			response, bodyContent := doGetRequest(orphanDeploymentsPath)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the service instances")
			Expect(bodyContent).To(MatchJSON(
				`[	{"deployment_name": "service-instance_orphan"}]`,
			))
		})

		It("responds with 500 when CF API call fails", func() {
			fakeCfClient.GetServiceInstancesReturns([]cf.Instance{}, errors.New("something failed on cf"))

			response, _ := doGetRequest(orphanDeploymentsPath)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("logging the failure")
			Expect(loggerBuffer).To(gbytes.Say(`something failed on cf`))
		})

		It("responds with 500 when BOSH API call fails", func() {
			fakeCfClient.GetServiceInstancesReturns([]cf.Instance{
				{
					GUID:         "not-orphan",
					PlanUniqueID: "plan-id",
				},
			}, nil)
			fakeBoshClient.GetDeploymentsReturns([]boshdirector.Deployment{}, errors.New("some bosh error"))

			response, _ := doGetRequest(orphanDeploymentsPath)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("logging the failure")
			Expect(loggerBuffer).To(gbytes.Say(`error occurred querying orphan deployments: some bosh error`))

		})
	})

	Describe("GET /mgmt/metrics", func() {
		const (
			metricsPath = "metrics"
		)

		BeforeEach(func() {
			quota := 12
			conf.ServiceCatalog.GlobalQuotas = brokerConfig.Quotas{ServiceInstanceLimit: &quota}
			servicePlan := cf.ServicePlan{
				ServicePlanEntity: cf.ServicePlanEntity{
					UniqueID: dedicatedPlanID,
				},
			}

			anotherServicePlan := cf.ServicePlan{
				ServicePlanEntity: cf.ServicePlanEntity{
					UniqueID: highMemoryPlanID,
				},
			}

			fakeCfClient.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{servicePlan: 1, anotherServicePlan: 4}, nil)
		})

		It("responds with some metrics", func() {
			metricsResp, bodyContent := doGetRequest(metricsPath)
			Expect(metricsResp.StatusCode).To(Equal(http.StatusOK))

			var brokerMetrics []mgmtapi.Metric
			Expect(json.Unmarshal(bodyContent, &brokerMetrics)).To(Succeed())
			Expect(brokerMetrics).To(ConsistOf(
				mgmtapi.Metric{
					Key:   "/on-demand-broker/service-name/dedicated-plan-name/total_instances",
					Value: 1,
					Unit:  "count",
				},
				mgmtapi.Metric{
					Key:   "/on-demand-broker/service-name/dedicated-plan-name/quota_remaining",
					Value: 0,
					Unit:  "count",
				},
				mgmtapi.Metric{
					Key:   "/on-demand-broker/service-name/high-memory-plan-name/total_instances",
					Value: 4,
					Unit:  "count",
				},
				mgmtapi.Metric{
					Key:   "/on-demand-broker/service-name/total_instances",
					Value: 5,
					Unit:  "count",
				},
				mgmtapi.Metric{
					Key:   "/on-demand-broker/service-name/quota_remaining",
					Value: 7,
					Unit:  "count",
				},
			))

		})

		Context("when no global quota is configured", func() {
			BeforeEach(func() {
				conf.ServiceCatalog.GlobalQuotas = brokerConfig.Quotas{}
			})

			It("does not include global quota metric", func() {
				metricsResp, bodyContent := doGetRequest(metricsPath)
				Expect(metricsResp.StatusCode).To(Equal(http.StatusOK))

				var brokerMetrics []mgmtapi.Metric
				Expect(json.Unmarshal(bodyContent, &brokerMetrics)).To(Succeed())
				Expect(brokerMetrics).To(ConsistOf(
					mgmtapi.Metric{
						Key:   "/on-demand-broker/service-name/dedicated-plan-name/total_instances",
						Value: 1,
						Unit:  "count",
					},
					mgmtapi.Metric{
						Key:   "/on-demand-broker/service-name/dedicated-plan-name/quota_remaining",
						Value: 0,
						Unit:  "count",
					},
					mgmtapi.Metric{
						Key:   "/on-demand-broker/service-name/high-memory-plan-name/total_instances",
						Value: 4,
						Unit:  "count",
					},
					mgmtapi.Metric{
						Key:   "/on-demand-broker/service-name/total_instances",
						Value: 5,
						Unit:  "count",
					},
				))
			})
		})

		It("fails when the broker is not registered with CF", func() {
			fakeCfClient.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{}, nil)

			response, _ := doGetRequest(metricsPath)
			Expect(response.StatusCode).To(Equal(http.StatusServiceUnavailable))

			By("logging the error with the same request ID")
			Eventually(loggerBuffer).Should(gbytes.Say(fmt.Sprintf(`The %s service broker must be registered with Cloud Foundry before metrics can be collected`, serviceName)))
		})

		It("fails when the CF API fails", func() {
			fakeCfClient.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{}, errors.New("CF API error"))

			response, _ := doGetRequest(metricsPath)
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("logging the error with the same request ID")
			Eventually(loggerBuffer).Should(gbytes.Say(fmt.Sprintf(`error getting instance count for service offering %s: CF API error`, serviceName)))
		})
	})

	Describe("PATCH /mgmt/service_instances/:id?operation_type=", func() {
		const (
			instanceID = "some-instance-id"
		)

		Context("when performing an upgrade", func() {
			const (
				operationType = "upgrade"
			)

			It("responds with the upgrade operation data", func() {
				taskID := 123
				fakeTaskBoshClient.GetDeploymentReturns(nil, true, nil)
				fakeTaskBoshClient.DeployReturns(taskID, nil)
				setupFakeGenerateManifestOutput()

				response, bodyContent := doProcessRequest(instanceID, fmt.Sprintf(`{"plan_id": "%s"}`, dedicatedPlanID), operationType)

				Expect(response.StatusCode).To(Equal(http.StatusAccepted))

				By("upgrades the correct instance")
				input, actualOthers := fakeCommandRunner.RunWithInputParamsArgsForCall(0)
				actualInput, ok := input.(sdk.InputParams)
				Expect(ok).To(BeTrue(), "command runner takes a sdk.inputparams obj")
				Expect(actualOthers[1]).To(Equal("generate-manifest"))
				Expect(actualInput.GenerateManifest.ServiceDeployment).To(ContainSubstring(`"deployment_name":"service-instance_some-instance-id"`))

				_, contextID, _, _ := fakeTaskBoshClient.DeployArgsForCall(0)
				Expect(contextID).NotTo(BeEmpty())

				By("updating the bosh configs")
				Expect(fakeTaskBoshClient.UpdateConfigCallCount()).To(Equal(1), "UpdateConfig should have been called")

				By("returning the correct operation data")
				var operationData broker.OperationData
				Expect(json.Unmarshal(bodyContent, &operationData)).To(Succeed())

				Expect(operationData).To(Equal(broker.OperationData{
					OperationType: broker.OperationTypeUpgrade,
					BoshTaskID:    123,
					BoshContextID: operationData.BoshContextID,
					Errands: []brokerConfig.Errand{{
						Name:      "post-deploy-errand",
						Instances: []string{},
					}},
				}))
			})

			Context("when post-deploy errand instances are provided", func() {
				BeforeEach(func() {
					conf.ServiceCatalog.Plans[0].LifecycleErrands.PostDeploy[0].Instances = []string{"instance-group-name/0"}
				})

				It("responds with the upgrade operation data", func() {
					taskID := 123
					fakeTaskBoshClient.GetDeploymentReturns(nil, true, nil)
					fakeTaskBoshClient.DeployReturns(taskID, nil)
					setupFakeGenerateManifestOutput()

					response, bodyContent := doProcessRequest(instanceID, fmt.Sprintf(`{"plan_id": "%s"}`, dedicatedPlanID), operationType)

					Expect(response.StatusCode).To(Equal(http.StatusAccepted))

					By("upgrades the correct instance")
					input, actualOthers := fakeCommandRunner.RunWithInputParamsArgsForCall(0)
					actualInput, ok := input.(sdk.InputParams)
					Expect(ok).To(BeTrue(), "command runner takes a sdk.inputparams obj")
					Expect(actualOthers[1]).To(Equal("generate-manifest"))
					Expect(actualInput.GenerateManifest.ServiceDeployment).To(ContainSubstring(`"deployment_name":"service-instance_some-instance-id"`))

					_, contextID, _, _ := fakeTaskBoshClient.DeployArgsForCall(0)
					Expect(contextID).NotTo(BeEmpty())

					By("returning the correct operation data")
					var operationData broker.OperationData
					Expect(json.Unmarshal(bodyContent, &operationData)).To(Succeed())

					Expect(operationData).To(Equal(broker.OperationData{
						OperationType: broker.OperationTypeUpgrade,
						BoshTaskID:    123,
						BoshContextID: operationData.BoshContextID,
						Errands: []brokerConfig.Errand{{
							Name:      "post-deploy-errand",
							Instances: []string{"instance-group-name/0"},
						}},
					}))
				})
			})

			It("responds with 422 when the request body is empty", func() {
				response, _ := doProcessRequest(instanceID, "", operationType)

				Expect(response.StatusCode).To(Equal(http.StatusUnprocessableEntity))
			})

			When("Bosh configs are disabled", func() {
				BeforeEach(func() {
					conf.Broker.DisableBoshConfigs = true

					taskID := 123
					fakeTaskBoshClient.GetDeploymentReturns(nil, true, nil)
					fakeTaskBoshClient.DeployReturns(taskID, nil)
				})

				It("succeeds when generate manifest output doesn't include bosh configs", func() {
					generateManifestOutput := sdk.MarshalledGenerateManifest{
						Manifest: `name: service-instance_some-instance-id`,
						ODBManagedSecrets: map[string]interface{}{
							"": nil,
						},
					}
					generateManifestOutputBytes, err := json.Marshal(generateManifestOutput)
					Expect(err).NotTo(HaveOccurred())
					zero := 0
					fakeCommandRunner.RunWithInputParamsReturns(generateManifestOutputBytes, []byte{}, &zero, nil)

					response, _ := doProcessRequest(instanceID, fmt.Sprintf(`{"plan_id": "%s"}`, dedicatedPlanID), operationType)

					Expect(response.StatusCode).To(Equal(http.StatusAccepted))
					Expect(fakeTaskBoshClient.GetConfigsCallCount()).To(Equal(0), "GetConfigs shouldn't be called")
					Expect(fakeTaskBoshClient.UpdateConfigCallCount()).To(Equal(0), "UpdateConfig shouldn't be called")
				})

				It("fails when the adapter generate manifest output includes bosh configs", func() {
					response, _ := doProcessRequest(instanceID, fmt.Sprintf(`{"plan_id": "%s"}`, dedicatedPlanID), operationType)
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					Expect(fakeTaskBoshClient.GetConfigsCallCount()).To(Equal(0), "GetConfigs shouldn't be called")
					Expect(fakeTaskBoshClient.UpdateConfigCallCount()).To(Equal(0), "UpdateConfig shouldn't be called")
				})
			})
		})

		Context("when performing a recreate", func() {
			const (
				operationType = "recreate"
			)

			It("responds with the recreate operation data", func() {
				taskID := 123
				fakeTaskBoshClient.GetDeploymentReturns(nil, true, nil)
				fakeTaskBoshClient.RecreateReturns(taskID, nil)

				response, bodyContent := doProcessRequest(instanceID, fmt.Sprintf(`{"plan_id": "%s"}`, dedicatedPlanID), operationType)

				Expect(response.StatusCode).To(Equal(http.StatusAccepted))

				By("recreates the correct instance")
				deploymentName, _, _, _ := fakeTaskBoshClient.RecreateArgsForCall(0)
				Expect(deploymentName).To(Equal(fmt.Sprintf("service-instance_%s", instanceID)))
				Expect(fakeCommandRunner.RunWithInputParamsCallCount()).To(BeZero())

				By("returning the correct operation data")
				var operationData broker.OperationData
				Expect(json.Unmarshal(bodyContent, &operationData)).To(Succeed())

				Expect(operationData).To(Equal(broker.OperationData{
					OperationType: broker.OperationTypeRecreate,
					BoshTaskID:    123,
					BoshContextID: operationData.BoshContextID,
					Errands: []brokerConfig.Errand{{
						Name:      "post-deploy-errand",
						Instances: []string{},
					}},
				}))
			})

			It("responds with 422 when the request body is empty", func() {
				response, _ := doProcessRequest(instanceID, "", operationType)

				Expect(response.StatusCode).To(Equal(http.StatusUnprocessableEntity))
			})
		})

		Context("With a valid operation type", func() {

			const (
				operationType = "upgrade"
			)

			It("responds with 410 when instance's deployment cannot be found in BOSH", func() {
				// This is the default for the fake, but just to be explicit
				fakeTaskBoshClient.GetDeploymentReturns(nil, false, nil)

				response, _ := doProcessRequest(instanceID, fmt.Sprintf(`{"plan_id": "%s"}`, dedicatedPlanID), operationType)

				Expect(response.StatusCode).To(Equal(http.StatusGone))
			})

			It("responds with 409 when there are incomplete tasks for the instance's deployment", func() {
				fakeTaskBoshClient.GetTasksInProgressReturns(boshdirector.BoshTasks{
					{State: boshdirector.TaskProcessing},
				}, nil)

				response, _ := doProcessRequest(instanceID, fmt.Sprintf(`{"plan_id": "%s"}`, dedicatedPlanID), operationType)

				Expect(response.StatusCode).To(Equal(http.StatusConflict))
			})
		})

		It("responds with 400 'Bad Request' when operation_type is unknown", func() {
			response, _ := doProcessRequest(instanceID, "", "unknown_operation_type")

			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})

	})
})

func doProcessRequest(serviceInstanceID, body, operationType string) (*http.Response, []byte) {
	return doRequestWithAuth(
		http.MethodPatch,
		fmt.Sprintf("http://%s/mgmt/service_instances/%s?operation_type=%s", serverURL, serviceInstanceID, operationType),
		strings.NewReader(body))
}

func doGetRequest(path string) (*http.Response, []byte) {
	return doRequestWithAuth(
		http.MethodGet,
		fmt.Sprintf("http://%s/mgmt/%s", serverURL, path),
		nil)
}

func doGetRequestWithoutAuth(path string) (*http.Response, []byte) {
	return doRequestWithoutAuth(
		http.MethodGet,
		fmt.Sprintf("http://%s/mgmt/%s", serverURL, path),
		nil)
}
