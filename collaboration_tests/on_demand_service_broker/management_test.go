package on_demand_service_broker_test

import (
	"fmt"
	"net/http"

	"encoding/json"

	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/task"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"github.com/pkg/errors"
)

var _ = Describe("Management API", func() {
	var (
		globalQuota  = 12
		globalQuotas brokerConfig.Quotas
		instances    []string
	)

	BeforeEach(func() {
		instances = []string{}
	})

	JustBeforeEach(func() {
		conf := brokerConfig.Config{
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
								Instances: instances,
							}},
						},
					},
					{
						Name: highMemoryPlanName,
						ID:   highMemoryPlanID,
					},
				},
				GlobalQuotas: globalQuotas,
			},
		}

		StartServer(conf)
	})

	Describe("GET /mgmt/service_instances", func() {
		const (
			serviceInstancesPath = "service_instances"
		)

		It("returns some service instances results", func() {
			fakeCfClient.GetInstancesOfServiceOfferingReturns([]service.Instance{
				{
					GUID:         "service-instance-id",
					PlanUniqueID: "plan-id",
				},
				{
					GUID:         "another-service-instance-id",
					PlanUniqueID: "another-plan-id",
				},
			}, nil)

			response, bodyContent := doGetRequest(serviceInstancesPath)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the service instances")
			Expect(bodyContent).To(MatchJSON(
				`[
					{"service_instance_id": "service-instance-id", "plan_id":"plan-id"},
					{"service_instance_id": "another-service-instance-id", "plan_id":"another-plan-id"}
				]`,
			))
		})

		It("returns 500 when getting instances fails", func() {
			fakeCfClient.GetInstancesOfServiceOfferingReturns([]service.Instance{}, errors.New("something failed"))

			response, _ := doGetRequest(serviceInstancesPath)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("logging the failure")
			Expect(loggerBuffer).To(gbytes.Say(`error occurred querying instances: something failed`))
		})
	})

	Describe("GET /mgmt/orphan_deployments", func() {
		const (
			orphanDeploymentsPath = "orphan_deployments"
		)

		It("responds with the orphan deployments", func() {
			fakeCfClient.GetInstancesOfServiceOfferingReturns([]service.Instance{
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
			fakeCfClient.GetInstancesOfServiceOfferingReturns([]service.Instance{}, errors.New("something failed on cf"))

			response, _ := doGetRequest(orphanDeploymentsPath)

			By("returning the correct status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("logging the failure")
			Expect(loggerBuffer).To(gbytes.Say(`error occurred querying orphan deployments: something failed on cf`))

		})

		It("responds with 500 when BOSH API call fails", func() {
			fakeCfClient.GetInstancesOfServiceOfferingReturns([]service.Instance{
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
			globalQuotas = brokerConfig.Quotas{ServiceInstanceLimit: &globalQuota}
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
				globalQuotas = brokerConfig.Quotas{}
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

	Describe("PATCH /mgmt/service_instances/:id", func() {
		const (
			instanceID = "some-instance-id"
		)

		It("responds with the upgrade operation data", func() {
			taskID := 123
			fakeDeployer.UpgradeReturns(taskID, nil, nil)

			response, bodyContent := doUpgradeRequest(instanceID, fmt.Sprintf(`{"plan_id": "%s"}`, dedicatedPlanID))

			Expect(response.StatusCode).To(Equal(http.StatusAccepted))

			By("upgrades the correct instance")
			deploymentName, planID, _, contextID, _ := fakeDeployer.UpgradeArgsForCall(0)
			Expect(deploymentName).To(Equal("service-instance_some-instance-id"))
			Expect(planID).To(Equal(dedicatedPlanID))
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
					Instances: []string{},
				}},
			}))
		})

		Context("when post-deploy errand instances are provided", func() {
			BeforeEach(func() {
				instances = []string{"instance-group-name/0"}
			})

			It("responds with the upgrade operation data", func() {
				taskID := 123
				fakeDeployer.UpgradeReturns(taskID, nil, nil)

				response, bodyContent := doUpgradeRequest(instanceID, fmt.Sprintf(`{"plan_id": "%s"}`, dedicatedPlanID))

				Expect(response.StatusCode).To(Equal(http.StatusAccepted))

				By("upgrades the correct instance")
				deploymentName, planID, _, contextID, _ := fakeDeployer.UpgradeArgsForCall(0)
				Expect(deploymentName).To(Equal("service-instance_some-instance-id"))
				Expect(planID).To(Equal(dedicatedPlanID))
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

		It("responds with 410 when instance's deployment cannot be found in BOSH", func() {
			fakeDeployer.UpgradeReturns(0, nil, task.DeploymentNotFoundError{})

			response, _ := doUpgradeRequest(instanceID, fmt.Sprintf(`{"plan_id": "%s"}`, dedicatedPlanID))

			Expect(response.StatusCode).To(Equal(http.StatusGone))
		})

		It("responds with 409 when there are incomplete tasks for the instance's deployment", func() {
			fakeDeployer.UpgradeReturns(0, nil, task.TaskInProgressError{})

			response, _ := doUpgradeRequest(instanceID, fmt.Sprintf(`{"plan_id": "%s"}`, dedicatedPlanID))

			Expect(response.StatusCode).To(Equal(http.StatusConflict))
		})

		It("responds with 422 when the request body is empty", func() {
			response, _ := doUpgradeRequest(instanceID, "")

			Expect(response.StatusCode).To(Equal(http.StatusUnprocessableEntity))
		})
	})
})

func doGetRequest(path string) (*http.Response, []byte) {
	return doRequest(http.MethodGet, fmt.Sprintf("http://%s/mgmt/%s", serverURL, path), nil)
}

func doUpgradeRequest(serviceInstanceID, body string) (*http.Response, []byte) {
	return doRequest(http.MethodPatch, fmt.Sprintf("http://%s/mgmt/service_instances/%s", serverURL, serviceInstanceID), strings.NewReader(body))
}
