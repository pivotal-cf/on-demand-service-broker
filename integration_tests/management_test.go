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
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"io"
)

var _ = Describe("Management API", func() {
	var (
		conf                   config.Config
		runningBroker          *gexec.Session
		boshDirector           *mockhttp.Server
		boshUAA                *mockuaa.ClientCredentialsServer
		cfAPI                  *mockhttp.Server
		cfUAA                  *mockuaa.ClientCredentialsServer
		instanceID             = "instance-id"
		postDeployErrandPlanID = "post-deploy-plan-errand-id"
	)

	BeforeEach(func() {
		boshDirector = mockbosh.New()
		boshUAA = mockuaa.NewClientCredentialsServer(boshClientID, boshClientSecret, "bosh uaa token")
		boshDirector.ExpectedAuthorizationHeader(boshUAA.ExpectedAuthorizationHeader())
		cfAPI = mockcfapi.New()
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")
		conf = defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)
	})

	JustBeforeEach(func() {
		runningBroker = startBrokerWithPassingStartupChecks(conf, cfAPI, boshDirector)
	})

	AfterEach(func() {
		killBrokerAndCheckForOpenConnections(runningBroker, "not used")
		boshDirector.VerifyMocks()
		boshDirector.Close()
		boshUAA.Close()
		cfAPI.VerifyMocks()
		cfAPI.Close()
		cfUAA.Close()
	})

	Describe("all instances", func() {
		Context("when there is one instance", func() {
			It("responds with the instance ID", func() {
				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
					mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
					mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithServiceInstances("instance-1"),
				)

				instancesRequest, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/mgmt/service_instances", brokerPort), nil)
				Expect(err).ToNot(HaveOccurred())
				instancesRequest = basicAuthBrokerRequest(instancesRequest)

				By("responding with 200")
				instancesResponse, err := http.DefaultClient.Do(instancesRequest)
				Expect(err).ToNot(HaveOccurred())
				Expect(instancesResponse.StatusCode).To(Equal(http.StatusOK))

				By("responding with a JSON body listing one instance")
				defer instancesResponse.Body.Close()
				Expect(ioutil.ReadAll(instancesResponse.Body)).To(MatchJSON(`[{"instance_id": "instance-1"}]`))
			})
		})

		Context("when the CF API call fails", func() {
			It("responds with an internal server error", func() {
				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsInternalServerErrorWith("error listing service offerings"),
				)

				instancesRequest, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/mgmt/service_instances", brokerPort), nil)
				Expect(err).ToNot(HaveOccurred())
				instancesRequest = basicAuthBrokerRequest(instancesRequest)

				By("responding with 500")
				instancesResponse, err := http.DefaultClient.Do(instancesRequest)
				Expect(err).ToNot(HaveOccurred())
				Expect(instancesResponse.StatusCode).To(Equal(http.StatusInternalServerError))

				By("logging the CF API call with the request ID")
				cfRegexpString := logRegexpStringWithRequestIDCapture(`GET http://127.0.0.1:\d+/v2/services\?results-per-page=100`)
				Eventually(runningBroker).Should(gbytes.Say(cfRegexpString))
				requestID := firstMatchInOutput(runningBroker, cfRegexpString)

				By("logging the error with the same request ID")
				mgmtLogRegexpString := logRegexpString(requestID, `error occurred querying instances: Unexpected reponse status 500, "error listing service offerings"`)
				Eventually(runningBroker).Should(gbytes.Say(mgmtLogRegexpString))
			})
		})
	})

	Describe("orphan deployments", func() {
		Context("when there is an orphan deployment", func() {
			It("responds with the deployment name", func() {
				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
					mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
					mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsOKWith(`{"next_url": null, "resources": []}`),
				)

				boshDirector.VerifyAndMock(
					mockbosh.Deployments().RespondsOKWith(`[{"name":"service-instance_123abc"}]`),
				)

				orphanRequest, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/mgmt/orphan_deployments", brokerPort), nil)
				Expect(err).ToNot(HaveOccurred())
				orphanRequest = basicAuthBrokerRequest(orphanRequest)

				By("responding with 200")
				orphanResponse, err := http.DefaultClient.Do(orphanRequest)
				Expect(err).ToNot(HaveOccurred())
				Expect(orphanResponse.StatusCode).To(Equal(http.StatusOK))

				By("responding with a JSON list of one orphan deployment")
				defer orphanResponse.Body.Close()
				Expect(ioutil.ReadAll(orphanResponse.Body)).To(MatchJSON(`[{"deployment_name": "service-instance_123abc"}]`))
			})

			Context("and CF instances have multiple pages", func() {
				It("responds with the deployment name", func() {
					cfAPI.VerifyAndMock(
						mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
						mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
						mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsWithPaginatedServiceInstances(
							"some-cc-plan-guid",
							1,
							100, //Match constant in implementation
							2,
							"one",
						),
						mockcfapi.ListServiceInstancesForPage("some-cc-plan-guid", 2).RespondsWithPaginatedServiceInstances(
							"some-cc-plan-guid",
							2,
							100, //Match constant in implementation
							2,
							"two",
						),
					)

					boshDirector.VerifyAndMock(
						mockbosh.Deployments().RespondsOKWith(`[{"name":"service-instance_123abc"},{"name":"service-instance_one"},{"name":"service-instance_two"}]`),
					)

					orphanRequest, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/mgmt/orphan_deployments", brokerPort), nil)
					Expect(err).ToNot(HaveOccurred())
					orphanRequest = basicAuthBrokerRequest(orphanRequest)

					By("responding with 200")
					orphanResponse, err := http.DefaultClient.Do(orphanRequest)
					Expect(err).ToNot(HaveOccurred())
					Expect(orphanResponse.StatusCode).To(Equal(http.StatusOK))

					By("responding with a JSON list of one orphan deployment")
					defer orphanResponse.Body.Close()
					Expect(ioutil.ReadAll(orphanResponse.Body)).To(MatchJSON(`[{"deployment_name": "service-instance_123abc"}]`))
				})
			})
		})

		Context("when the CF API call fails", func() {
			It("responds with an internal server error", func() {
				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsInternalServerErrorWith("error listing service offerings"),
				)

				orphanRequest, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/mgmt/orphan_deployments", brokerPort), nil)
				Expect(err).ToNot(HaveOccurred())
				orphanRequest = basicAuthBrokerRequest(orphanRequest)

				By("responding with 500")
				orphanResponse, err := http.DefaultClient.Do(orphanRequest)
				Expect(err).ToNot(HaveOccurred())
				Expect(orphanResponse.StatusCode).To(Equal(http.StatusInternalServerError))

				By("logging the CF API call with the request ID")
				cfRegexpString := logRegexpStringWithRequestIDCapture(`GET http://127.0.0.1:\d+/v2/services\?results-per-page=100`)
				Eventually(runningBroker).Should(gbytes.Say(cfRegexpString))
				requestID := firstMatchInOutput(runningBroker, cfRegexpString)

				By("logging the error with the same request ID")
				mgmtLogRegexpString := logRegexpString(requestID, `error occurred querying orphan deployments: Unexpected reponse status 500, "error listing service offerings"`)
				Eventually(runningBroker).Should(gbytes.Say(mgmtLogRegexpString))
			})
		})

		Context("when the BOSH Director call fails", func() {
			It("responds with an internal server error", func() {
				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
					mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlan(dedicatedPlanID, "some-cc-plan-guid"),
					mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsOKWith(`{"next_url": null, "resources": []}`),
				)

				boshDirector.VerifyAndMock(
					mockbosh.Deployments().RespondsInternalServerErrorWith("error listing deployments"),
				)

				orphanRequest, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/mgmt/orphan_deployments", brokerPort), nil)
				Expect(err).ToNot(HaveOccurred())
				orphanRequest = basicAuthBrokerRequest(orphanRequest)

				By("responding with 500")
				orphanResponse, err := http.DefaultClient.Do(orphanRequest)
				Expect(err).ToNot(HaveOccurred())
				Expect(orphanResponse.StatusCode).To(Equal(http.StatusInternalServerError))

				By("logging the BOSH Director call with the request ID")
				boshRegexpString := logRegexpStringWithRequestIDCapture(`getting deployments from bosh`)
				Eventually(runningBroker).Should(gbytes.Say(boshRegexpString))
				requestID := firstMatchInOutput(runningBroker, boshRegexpString)

				By("logging the error with the same request ID")
				mgmtLogRegexpString := logRegexpString(requestID, `error occurred querying orphan deployments: expected status 200, was 500. Response Body: error listing deployments`)
				Eventually(runningBroker).Should(gbytes.Say(mgmtLogRegexpString))
			})
		})
	})

	Describe("metrics", func() {
		Context("when the broker is registered with CF", func() {
			Context("when there are some instances and there is a global quota", func() {
				BeforeEach(func() {
					limit := 12
					conf.ServiceCatalog.GlobalQuotas = config.Quotas{ServiceInstanceLimit: &limit}
				})

				It("responds with metrics", func() {
					cfAPI.VerifyAndMock(
						mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
						mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlans(
							mockcfapi.Plan{ID: dedicatedPlanID, CloudControllerGUID: "some-cc-plan-guid"},
							mockcfapi.Plan{ID: highMemoryPlanID, CloudControllerGUID: "other-plan-guid"},
						),
						mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsOKWith(listCFServiceInstanceCountForPlanResponse(1)),
						mockcfapi.ListServiceInstances("other-plan-guid").RespondsOKWith(listCFServiceInstanceCountForPlanResponse(4)),
					)

					metricsReq, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/mgmt/metrics", brokerPort), nil)
					Expect(err).ToNot(HaveOccurred())
					metricsReq = basicAuthBrokerRequest(metricsReq)

					By("responding with 200")
					metricsResp, err := http.DefaultClient.Do(metricsReq)
					Expect(err).ToNot(HaveOccurred())
					Expect(metricsResp.StatusCode).To(Equal(http.StatusOK))

					By("responding with a JSON body listing metrics for both plans")
					defer metricsResp.Body.Close()
					var brokerMetrics []mgmtapi.Metric
					Expect(json.NewDecoder(metricsResp.Body).Decode(&brokerMetrics)).To(Succeed())
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
			})

			Context("when there are no instances and no global quota", func() {

				It("responds with metrics", func() {
					cfAPI.VerifyAndMock(
						mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceID, "some-cc-service-offering-guid"),
						mockcfapi.ListServicePlans("some-cc-service-offering-guid").RespondsWithServicePlans(
							mockcfapi.Plan{ID: dedicatedPlanID, CloudControllerGUID: "some-cc-plan-guid"},
							mockcfapi.Plan{ID: highMemoryPlanID, CloudControllerGUID: "other-plan-guid"},
						),
						mockcfapi.ListServiceInstances("some-cc-plan-guid").RespondsOKWith(listCFServiceInstanceCountForPlanResponse(0)),
						mockcfapi.ListServiceInstances("other-plan-guid").RespondsOKWith(listCFServiceInstanceCountForPlanResponse(0)),
					)

					metricsReq, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/mgmt/metrics", brokerPort), nil)
					Expect(err).ToNot(HaveOccurred())
					metricsReq = basicAuthBrokerRequest(metricsReq)

					By("responding with 200")
					metricsResp, err := http.DefaultClient.Do(metricsReq)
					Expect(err).ToNot(HaveOccurred())
					Expect(metricsResp.StatusCode).To(Equal(http.StatusOK))

					By("responding with a JSON body listing zero instances for plans")
					defer metricsResp.Body.Close()
					var brokerMetrics []mgmtapi.Metric
					Expect(json.NewDecoder(metricsResp.Body).Decode(&brokerMetrics)).To(Succeed())
					Expect(brokerMetrics).To(ConsistOf(
						mgmtapi.Metric{
							Key:   "/on-demand-broker/service-name/dedicated-plan-name/total_instances",
							Value: 0,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/service-name/dedicated-plan-name/quota_remaining",
							Value: 1,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/service-name/high-memory-plan-name/total_instances",
							Value: 0,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/service-name/total_instances",
							Value: 0,
							Unit:  "count",
						},
					))
				})
			})
		})

		Context("when the broker is not registered with CF", func() {
			Context("when there are no instances", func() {
				It("responds with metrics", func() {
					cfAPI.VerifyAndMock(
						mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
					)

					metricsReq, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/mgmt/metrics", brokerPort), nil)
					Expect(err).ToNot(HaveOccurred())
					metricsReq = basicAuthBrokerRequest(metricsReq)

					By("responding with 503")
					metricsResp, err := http.DefaultClient.Do(metricsReq)
					Expect(err).ToNot(HaveOccurred())
					Expect(metricsResp.StatusCode).To(Equal(http.StatusServiceUnavailable))

					By("logging the CF API call with the request ID")
					cfRegexpString := logRegexpStringWithRequestIDCapture(`GET http://127.0.0.1:\d+/v2/services\?results-per-page=100`)
					Eventually(runningBroker).Should(gbytes.Say(cfRegexpString))
					requestID := firstMatchInOutput(runningBroker, cfRegexpString)

					By("logging the error with the same request ID")
					mgmtLogRegexpString := logRegexpString(requestID, fmt.Sprintf(`service %s not registered with Cloud Foundry`, serviceName))
					Eventually(runningBroker).Should(gbytes.Say(mgmtLogRegexpString))
				})
			})
		})

		Context("when the CF API fails", func() {
			It("responds with 500", func() {
				cfAPI.VerifyAndMock(
					mockcfapi.ListServiceOfferings().RespondsInternalServerErrorWith("error listing service offerings"),
				)

				metricsReq, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/mgmt/metrics", brokerPort), nil)
				Expect(err).ToNot(HaveOccurred())
				metricsReq = basicAuthBrokerRequest(metricsReq)

				By("responding with 500")
				metricsResp, err := http.DefaultClient.Do(metricsReq)
				Expect(err).ToNot(HaveOccurred())
				Expect(metricsResp.StatusCode).To(Equal(http.StatusInternalServerError))

				By("logging the CF API call with the request ID")
				cfRegexpString := logRegexpStringWithRequestIDCapture(`GET http://127.0.0.1:\d+/v2/services\?results-per-page=100`)
				Eventually(runningBroker).Should(gbytes.Say(cfRegexpString))
				requestID := firstMatchInOutput(runningBroker, cfRegexpString)

				By("logging the error with the same request ID")
				mgmtLogRegexpString := logRegexpString(requestID, fmt.Sprintf(`error getting instance count for service offering %s: Unexpected reponse status 500, "error listing service offerings"`, serviceName))
				Eventually(runningBroker).Should(gbytes.Say(mgmtLogRegexpString))
			})
		})
	})

	Describe("upgrade instance", func() {
		Context("when the upgrade request is accepted", func() {
			BeforeEach(func() {
				adapter.GenerateManifest().ToReturnManifest(rawManifestWithDeploymentName(instanceID))

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
						PostDeploy: "health-check",
					},
				}

				conf.ServiceCatalog.Plans = []config.Plan{planWithPostDeploy}
			})

			It("responds with the correct operation data", func() {
				upgradingTaskID := 123
				cfAPI.VerifyAndMock(
					mockcfapi.GetServiceInstance(instanceID).RespondsWithSucceedWithPlanUrl(instanceID, `\/v2\/service_plans\/my-plan`),
					mockcfapi.GetServicePlan("my-plan").RespondsOKWith(getServicePlanResponse(postDeployErrandPlanID)),
				)

				boshDirector.VerifyAndMock(
					mockbosh.GetDeployment("service-instance_instance-id").RespondsWithRawManifest([]byte(rawManifestWithDeploymentName(instanceID))),
					mockbosh.Tasks("service-instance_instance-id").RespondsWithNoTasks(),
					mockbosh.Deploy().RedirectsToTask(upgradingTaskID),
				)

				upgradeReq, err := http.NewRequest("PATCH", fmt.Sprintf("http://localhost:%d/mgmt/service_instances/%s", brokerPort, instanceID), nil)
				Expect(err).ToNot(HaveOccurred())
				upgradeReq = basicAuthBrokerRequest(upgradeReq)

				By("responding with 202")
				upgradeResp, err := http.DefaultClient.Do(upgradeReq)
				Expect(err).ToNot(HaveOccurred())
				Expect(upgradeResp.StatusCode).To(Equal(http.StatusAccepted))

				operationData := decodeOperationDataFromResponseBody(upgradeResp.Body)

				Expect(operationData.BoshTaskID).To(Equal(123))
				Expect(operationData.BoshContextID).ToNot(BeEmpty())
				Expect(operationData.OperationType).To(Equal(broker.OperationTypeUpgrade))
				Expect(operationData.PostDeployErrandName).To(Equal("health-check"))
			})
		})

		Context("when the upgrade request fails", func() {
			It("responds with internal server error", func() {
				cfAPI.VerifyAndMock(
					mockcfapi.GetServiceInstance(instanceID).RespondsInternalServerErrorWith("error getting service instance"),
				)

				upgradeReq, err := http.NewRequest("PATCH", fmt.Sprintf("http://localhost:%d/mgmt/service_instances/%s", brokerPort, instanceID), nil)
				Expect(err).ToNot(HaveOccurred())
				upgradeReq = basicAuthBrokerRequest(upgradeReq)

				By("responding with 500")
				upgradeResp, err := http.DefaultClient.Do(upgradeReq)
				Expect(err).ToNot(HaveOccurred())
				Expect(upgradeResp.StatusCode).To(Equal(http.StatusInternalServerError))

				By("logging the CF API call with the request ID")
				cfRegexpString := logRegexpStringWithRequestIDCapture(fmt.Sprintf(`GET http://127.0.0.1:\d+/v2/service_instances/%s`, instanceID))
				Eventually(runningBroker).Should(gbytes.Say(cfRegexpString))
				requestID := firstMatchInOutput(runningBroker, cfRegexpString)

				By("logging the error with the same request ID")
				mgmtLogRegexpString := logRegexpString(requestID, fmt.Sprintf(`error occurred upgrading instance %s: Unexpected reponse status 500, "error getting service instance"`, instanceID))
				Eventually(runningBroker).Should(gbytes.Say(mgmtLogRegexpString))
			})
		})
	})
})

func decodeOperationDataFromResponseBody(respBody io.ReadCloser) broker.OperationData {
	body, err := ioutil.ReadAll(respBody)
	Expect(err).NotTo(HaveOccurred())

	var operationData broker.OperationData
	err = json.Unmarshal(body, &operationData)
	Expect(err).NotTo(HaveOccurred())
	return operationData
}
