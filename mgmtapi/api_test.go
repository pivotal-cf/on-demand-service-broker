// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mgmtapi_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"

	"strings"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi/fake_manageable_broker"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

var _ = Describe("Management API", func() {
	var (
		server           *httptest.Server
		manageableBroker *fake_manageable_broker.FakeManageableBroker
		logs             *gbytes.Buffer
		loggerFactory    *loggerfactory.LoggerFactory
		serviceOffering  config.ServiceOffering
	)

	BeforeEach(func() {
		serviceOffering = config.ServiceOffering{
			ID:    "some_service_offering-id",
			Name:  "some_service_offering",
			Plans: []config.Plan{{ID: "foo_id", Name: "foo_plan"}, {ID: "bar_id", Name: "bar_plan"}},
		}
		logs = gbytes.NewBuffer()
		loggerFactory = loggerfactory.New(io.MultiWriter(GinkgoWriter, logs), "mgmtapi-unit-tests", log.LstdFlags)
		manageableBroker = new(fake_manageable_broker.FakeManageableBroker)
	})

	JustBeforeEach(func() {
		router := mux.NewRouter()
		mgmtapi.AttachRoutes(router, manageableBroker, serviceOffering, loggerFactory)
		server = httptest.NewServer(router)
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("listing all instances", func() {
		When("there are no query params", func() {
			var (
				instance1 = service.Instance{
					GUID:         "instance-guid-1",
					PlanUniqueID: "this-is-plan-1",
				}
				instance2 = service.Instance{
					GUID:         "instance-guid-2",
					PlanUniqueID: "this-is-plan-1",
				}
				instance3 = service.Instance{
					GUID:         "instance-guid-3",
					PlanUniqueID: "this-is-plan-2",
				}
			)

			It("returns a list of all instances", func() {
				instances := []service.Instance{instance1, instance2, instance3}
				manageableBroker.InstancesReturns(instances, nil)

				listResp, err := http.Get(fmt.Sprintf("%s/mgmt/service_instances", server.URL))
				Expect(err).NotTo(HaveOccurred())

				Expect(listResp.StatusCode).To(Equal(http.StatusOK))
				var instancesResp []service.Instance
				Expect(json.NewDecoder(listResp.Body).Decode(&instancesResp)).To(Succeed())
				Expect(instancesResp).To(ConsistOf(instance1, instance2, instance3))
			})

			It("returns HTTP 500 and logs the error", func() {
				manageableBroker.InstancesReturns(nil, errors.New("error getting instances"))

				listResp, err := http.Get(fmt.Sprintf("%s/mgmt/service_instances", server.URL))
				Expect(err).NotTo(HaveOccurred())

				Expect(listResp.StatusCode).To(Equal(http.StatusInternalServerError))
				Eventually(logs).Should(gbytes.Say("error occurred querying instances: error getting instances"))
			})
		})

		When("there are query params", func() {
			var (
				instance1 = service.Instance{
					GUID:         "instance-guid-1",
					PlanUniqueID: "this-is-plan-1",
				}
				instance2 = service.Instance{
					GUID:         "instance-guid-2",
					PlanUniqueID: "this-is-plan-1",
				}
				instance3 = service.Instance{
					GUID:         "instance-guid-3",
					PlanUniqueID: "this-is-plan-2",
				}
			)

			It("returns a list of all instances", func() {
				instances := []service.Instance{instance1, instance2, instance3}
				manageableBroker.FilteredInstancesReturns(instances, nil)

				listResp, err := http.Get(fmt.Sprintf("%s/mgmt/service_instances?cf_org=banana&cf_space=latundan", server.URL))
				Expect(err).NotTo(HaveOccurred())

				Expect(listResp.StatusCode).To(Equal(http.StatusOK))
				var instancesResp []service.Instance
				Expect(json.NewDecoder(listResp.Body).Decode(&instancesResp)).To(Succeed())
				Expect(instancesResp).To(ConsistOf(instance1, instance2, instance3))
			})

			It("returns HTTP 500 and logs the error", func() {
				manageableBroker.FilteredInstancesReturns(nil, errors.New("error getting instances"))

				listResp, err := http.Get(fmt.Sprintf("%s/mgmt/service_instances?cf_org=banana&cf_space=latundan", server.URL))
				Expect(err).NotTo(HaveOccurred())

				Expect(listResp.StatusCode).To(Equal(http.StatusInternalServerError))
				Eventually(logs).Should(gbytes.Say("error occurred querying instances: error getting instances"))
			})
		})
	})

	Describe("process an instance", func() {
		var (
			instanceID  = "283974"
			taskID      = 54321
			planID      = "some-plan-id"
			requestBody string

			upgradeResp *http.Response
		)

		Context("when no operation type is provided", func() {
			It("responds with a 400 Bad Request", func() {
				var err error
				resp, err := Patch(fmt.Sprintf("%s/mgmt/service_instances/%s", server.URL, instanceID), requestBody)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Context("when the operation type is unknown", func() {
			It("responds with a 400 Bad Request", func() {
				var err error
				operationType := "not_a_real_operation"
				resp, err := Patch(fmt.Sprintf("%s/mgmt/service_instances/%s?operation_type=%s", server.URL, instanceID, operationType), requestBody)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Context("when the process is an upgrade", func() {
			const operationType = "upgrade"
			BeforeEach(func() {
				requestBody = fmt.Sprintf(`{"plan_id":"%s"}`, planID)
			})
			JustBeforeEach(func() {
				var err error
				upgradeResp, err = Patch(fmt.Sprintf("%s/mgmt/service_instances/%s?operation_type=%s", server.URL, instanceID, operationType), requestBody)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when it succeeds", func() {
				contextID := "some-context-id"

				BeforeEach(func() {
					manageableBroker.UpgradeReturns(broker.OperationData{
						BoshTaskID:    taskID,
						BoshContextID: contextID,
						PlanID:        planID,
						OperationType: broker.OperationTypeUpgrade,
					}, nil)
				})

				It("upgrades the instance using the broker", func() {
					Expect(manageableBroker.UpgradeCallCount()).To(Equal(1))
					_, actualInstanceID, actualUpdateDetails, _ := manageableBroker.UpgradeArgsForCall(0)
					Expect(actualInstanceID).To(Equal(instanceID))
					Expect(actualUpdateDetails).To(Equal(
						brokerapi.UpdateDetails{
							PlanID: planID,
						},
					))
				})

				It("responds with HTTP 202", func() {
					Expect(upgradeResp.StatusCode).To(Equal(http.StatusAccepted))
				})

				It("responds with operation data", func() {
					var upgradeRespBody broker.OperationData
					Expect(json.NewDecoder(upgradeResp.Body).Decode(&upgradeRespBody)).To(Succeed())
					Expect(upgradeRespBody.BoshTaskID).To(Equal(taskID))
					Expect(upgradeRespBody.BoshContextID).To(Equal(contextID))
					Expect(upgradeRespBody.PlanID).To(Equal(planID))
					Expect(upgradeRespBody.OperationType).To(Equal(broker.OperationTypeUpgrade))
				})
			})

			Context("when the CF service instance is not found", func() {
				BeforeEach(func() {
					manageableBroker.UpgradeReturns(broker.OperationData{}, cf.ResourceNotFoundError{})
				})

				It("responds with HTTP 404 Not Found", func() {
					Expect(upgradeResp.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when the bosh deployment is not found", func() {
				BeforeEach(func() {
					manageableBroker.UpgradeReturns(broker.OperationData{}, broker.NewDeploymentNotFoundError(errors.New("error finding deployment")))
				})

				It("responds with HTTP 410 Gone", func() {
					Expect(upgradeResp.StatusCode).To(Equal(http.StatusGone))
				})
			})

			Context("when there is an operation in progress", func() {
				BeforeEach(func() {
					manageableBroker.UpgradeReturns(broker.OperationData{}, broker.NewOperationInProgressError(errors.New("operation in progress error")))
				})

				It("responds with HTTP 409 Conflict", func() {
					Expect(upgradeResp.StatusCode).To(Equal(http.StatusConflict))
				})
			})

			Context("when it fails", func() {
				BeforeEach(func() {
					manageableBroker.UpgradeReturns(broker.OperationData{}, errors.New("upgrade error"))
				})

				It("responds with HTTP 500", func() {
					Expect(upgradeResp.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("includes the upgrade error in the response", func() {
					Expect(ioutil.ReadAll(upgradeResp.Body)).To(MatchJSON(`{"description": "upgrade error"}`))
				})

				It("logs the error", func() {
					Eventually(logs).Should(gbytes.Say(fmt.Sprintf("error occurred upgrading instance %s: upgrade error", instanceID)))
				})
			})

			Context("when no request body is provided", func() {
				BeforeEach(func() {
					requestBody = ""
				})

				It("fails with an appropriate error", func() {
					Expect(upgradeResp.StatusCode).To(Equal(http.StatusUnprocessableEntity))
					Expect(ioutil.ReadAll(upgradeResp.Body)).To(MatchJSON(`{"description": "Error in request body. Invalid JSON"}`))
					Eventually(logs).Should(gbytes.Say("error occurred parsing requests body: "))
				})
			})

		})
	})

	Describe("producing service metrics", func() {
		var instancesForPlanResponse *http.Response

		JustBeforeEach(func() {
			var err error
			instancesForPlanResponse, err = http.Get(fmt.Sprintf("%s/mgmt/metrics", server.URL))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when no quota is set", func() {
			Context("when there is one plan with instance count", func() {
				BeforeEach(func() {
					manageableBroker.CountInstancesOfPlansReturns(map[cf.ServicePlan]int{
						cfServicePlan("1234", "foo_id", "url", "name"): 2,
					}, nil)
				})

				It("returns HTTP 200", func() {
					Expect(instancesForPlanResponse.StatusCode).To(Equal(http.StatusOK))
				})

				It("counts instances for the plan", func() {
					Expect(manageableBroker.CountInstancesOfPlansCallCount()).To(Equal(1))
				})

				It("returns the correct number of instances", func() {
					defer instancesForPlanResponse.Body.Close()
					var brokerMetrics []mgmtapi.Metric

					Expect(json.NewDecoder(instancesForPlanResponse.Body).Decode(&brokerMetrics)).To(Succeed())
					Expect(brokerMetrics).To(ConsistOf(
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/foo_plan/total_instances",
							Value: 2,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/total_instances",
							Value: 2,
							Unit:  "count",
						},
					))
				})
			})

			Context("when there are multiple plans with instance counts", func() {
				BeforeEach(func() {
					manageableBroker.CountInstancesOfPlansReturns(map[cf.ServicePlan]int{
						cfServicePlan("1234", "foo_id", "url", "name"): 2,
						cfServicePlan("1234", "bar_id", "url", "name"): 3,
					}, nil)
				})

				It("returns HTTP 200", func() {
					Expect(instancesForPlanResponse.StatusCode).To(Equal(http.StatusOK))
				})

				It("counts instances for the plan", func() {
					Expect(manageableBroker.CountInstancesOfPlansCallCount()).To(Equal(1))
				})

				It("returns the correct number of instances", func() {
					defer instancesForPlanResponse.Body.Close()
					var brokerMetrics []mgmtapi.Metric

					Expect(json.NewDecoder(instancesForPlanResponse.Body).Decode(&brokerMetrics)).To(Succeed())
					Expect(brokerMetrics).To(ConsistOf(
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/foo_plan/total_instances",
							Value: 2,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/bar_plan/total_instances",
							Value: 3,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/total_instances",
							Value: 5,
							Unit:  "count",
						},
					))
				})
			})

			Context("when the instance count cannot be retrieved", func() {
				BeforeEach(func() {
					manageableBroker.CountInstancesOfPlansReturns(nil, errors.New("error counting instances"))
				})

				It("returns HTTP 500", func() {
					Expect(instancesForPlanResponse.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the broker is not registered with CF", func() {
				BeforeEach(func() {
					manageableBroker.CountInstancesOfPlansReturns(map[cf.ServicePlan]int{}, nil)
				})

				It("returns HTTP 503 and logs why", func() {
					Expect(instancesForPlanResponse.StatusCode).To(Equal(http.StatusServiceUnavailable))
					Expect(logs).To(gbytes.Say("The some_service_offering service broker must be registered with Cloud Foundry before metrics can be collected"))
				})
			})
		})

		Context("when a plan quota is set", func() {
			BeforeEach(func() {
				limit := 7
				serviceOffering.Plans[0].Quotas = config.Quotas{ServiceInstanceLimit: &limit}
			})

			Context("when the instance count can be retrieved", func() {
				BeforeEach(func() {
					manageableBroker.CountInstancesOfPlansReturns(map[cf.ServicePlan]int{
						cfServicePlan("1234", "foo_id", "url", "name"): 2,
					}, nil)
				})

				It("returns HTTP 200", func() {
					Expect(instancesForPlanResponse.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns the correct number of instances and quota", func() {
					defer instancesForPlanResponse.Body.Close()
					var brokerMetrics []mgmtapi.Metric

					Expect(json.NewDecoder(instancesForPlanResponse.Body).Decode(&brokerMetrics)).To(Succeed())
					Expect(brokerMetrics).To(ConsistOf(
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/foo_plan/total_instances",
							Value: 2,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/foo_plan/quota_remaining",
							Value: 5,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/total_instances",
							Value: 2,
							Unit:  "count",
						},
					))
				})

				It("counts instances for the plan", func() {
					Expect(manageableBroker.CountInstancesOfPlansCallCount()).To(Equal(1))
				})
			})
		})

		Context("when a global quota is set", func() {
			BeforeEach(func() {
				limit := 12
				serviceOffering.GlobalQuotas = config.Quotas{ServiceInstanceLimit: &limit}
			})

			Context("when the instance count can be retrieved", func() {
				BeforeEach(func() {
					manageableBroker.CountInstancesOfPlansReturns(map[cf.ServicePlan]int{
						cfServicePlan("1234", "foo_id", "url", "name"): 2,
						cfServicePlan("1234", "bar_id", "url", "name"): 3,
					}, nil)
				})

				It("returns HTTP 200", func() {
					Expect(instancesForPlanResponse.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns the correct number of instances", func() {
					defer instancesForPlanResponse.Body.Close()
					var brokerMetrics []mgmtapi.Metric

					Expect(json.NewDecoder(instancesForPlanResponse.Body).Decode(&brokerMetrics)).To(Succeed())
					Expect(brokerMetrics).To(ConsistOf(
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/foo_plan/total_instances",
							Value: 2,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/bar_plan/total_instances",
							Value: 3,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/total_instances",
							Value: 5,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/quota_remaining",
							Value: 7,
							Unit:  "count",
						},
					))
				})

				It("counts instances for the plan", func() {
					Expect(manageableBroker.CountInstancesOfPlansCallCount()).To(Equal(1))
				})
			})
		})

		Context("when there are no service instances", func() {
			BeforeEach(func() {
				manageableBroker.CountInstancesOfPlansReturns(map[cf.ServicePlan]int{
					cfServicePlan("1234", "foo_id", "url", "name"): 0,
					cfServicePlan("1234", "bar_id", "url", "name"): 0,
				}, nil)
			})

			It("returns HTTP 200", func() {
				Expect(instancesForPlanResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns the correct number of instances", func() {
				defer instancesForPlanResponse.Body.Close()
				var brokerMetrics []mgmtapi.Metric

				Expect(json.NewDecoder(instancesForPlanResponse.Body).Decode(&brokerMetrics)).To(Succeed())
				Expect(brokerMetrics).To(ConsistOf(
					mgmtapi.Metric{
						Key:   "/on-demand-broker/some_service_offering/foo_plan/total_instances",
						Value: 0,
						Unit:  "count",
					},
					mgmtapi.Metric{
						Key:   "/on-demand-broker/some_service_offering/bar_plan/total_instances",
						Value: 0,
						Unit:  "count",
					},
					mgmtapi.Metric{
						Key:   "/on-demand-broker/some_service_offering/total_instances",
						Value: 0,
						Unit:  "count",
					},
				))
			})

			It("counts instances for the plan", func() {
				Expect(manageableBroker.CountInstancesOfPlansCallCount()).To(Equal(1))
			})
		})
	})

	Describe("listing orphan service deployments", func() {
		var listResp *http.Response

		JustBeforeEach(func() {
			var err error
			listResp, err = http.Get(fmt.Sprintf("%s/mgmt/orphan_deployments", server.URL))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there are no orphans", func() {
			It("returns HTTP 200", func() {
				Expect(listResp.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns no deployments", func() {
				defer listResp.Body.Close()
				body, err := ioutil.ReadAll(listResp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body).To(MatchJSON(`[]`))
			})
		})

		Context("when there are some orphans", func() {
			var (
				orphan1 = mgmtapi.Deployment{
					Name: "orphan1",
				}
				orphan2 = mgmtapi.Deployment{
					Name: "orphan2",
				}
			)

			BeforeEach(func() {
				manageableBroker.OrphanDeploymentsReturns([]string{orphan1.Name, orphan2.Name}, nil)
			})

			It("returns HTTP 200", func() {
				Expect(listResp.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns some deployments", func() {
				var orphans []mgmtapi.Deployment
				Expect(json.NewDecoder(listResp.Body).Decode(&orphans)).To(Succeed())
				Expect(orphans).To(ConsistOf(orphan1, orphan2))
			})
		})

		Context("when broker returns an error", func() {
			BeforeEach(func() {
				manageableBroker.OrphanDeploymentsReturns([]string{}, errors.New("Broker errored."))
			})

			It("returns HTTP 500", func() {
				Expect(listResp.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("returns an empty body", func() {
				defer listResp.Body.Close()
				body, err := ioutil.ReadAll(listResp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body).To(BeEmpty())
			})

			It("logs an error", func() {
				Eventually(logs).Should(gbytes.Say("error occurred querying orphan deployments: Broker errored."))
			})
		})
	})
})

func Patch(url, body string) (resp *http.Response, err error) {
	bodyReader := strings.NewReader(body)
	req, err := http.NewRequest("PATCH", url, bodyReader)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

func cfServicePlan(guid, uniqueID, servicePlanUrl, name string) cf.ServicePlan {
	return cf.ServicePlan{
		Metadata: cf.Metadata{
			GUID: guid,
		},
		ServicePlanEntity: cf.ServicePlanEntity{
			UniqueID:            uniqueID,
			ServiceInstancesUrl: servicePlanUrl,
			Name:                name,
		},
	}
}
