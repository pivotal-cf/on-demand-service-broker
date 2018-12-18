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

	"bytes"
	"encoding/json"

	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	taskfakes "github.com/pivotal-cf/on-demand-service-broker/task/fakes"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"github.com/pkg/errors"
)

var _ = Describe("Provision service instance", func() {
	const (
		taskID             = 2312
		planWithoutQuotaID = "plan-without-quota"
		planWithQuotaID    = "plan-with-quota"
		planWithErrandID   = "plan-with-errand"
		instanceID         = "some-instance-id"
		serviceID          = "service-id"
	)

	var (
		planQuota       = 5
		globalQuota     = 12
		arbitraryParams map[string]interface{}
		conf            brokerConfig.Config
	)

	BeforeEach(func() {
		arbitraryParams = map[string]interface{}{"some": "prop"}

		planCounts := map[cf.ServicePlan]int{
			cf.ServicePlan{
				ServicePlanEntity: cf.ServicePlanEntity{
					UniqueID: planWithQuotaID,
				},
			}: 0,
		}
		fakeCfClient.CountInstancesOfServiceOfferingReturns(planCounts, nil)

		conf = brokerConfig.Config{
			Broker: brokerConfig.Broker{
				Port: serverPort, Username: brokerUsername, Password: brokerPassword,
			},
			ServiceCatalog: brokerConfig.ServiceOffering{
				GlobalQuotas: brokerConfig.Quotas{ServiceInstanceLimit: &globalQuota},
				Name:         serviceName,
				ID:           serviceID,
				Plans: brokerConfig.Plans{
					{
						Name:       "some-other-plan",
						ID:         planWithQuotaID,
						Quotas:     brokerConfig.Quotas{ServiceInstanceLimit: &planQuota},
						Properties: sdk.Properties{"type": "plan-with-quota", "global_property": "global_value"},
						Update:     dedicatedPlanUpdateBlock,
						InstanceGroups: []sdk.InstanceGroup{
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
					},
					{Name: "some-plan", ID: planWithoutQuotaID},
					{
						Name: "a-plan-with-errand",
						ID:   planWithErrandID,
						InstanceGroups: []sdk.InstanceGroup{
							{
								Name:      "instance-group-name",
								VMType:    "post-deploy-errand-vm-type",
								Instances: 1,
								Networks:  []string{"net1"},
								AZs:       []string{"az1"},
							},
						},
						LifecycleErrands: &sdk.LifecycleErrands{
							PostDeploy: []sdk.Errand{{
								Name:      "health-check",
								Instances: []string{"health-check-instance/0", "health-check-instance/1"},
							}},
						},
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		StartServer(conf)
	})

	Describe("ODB Secret handling", func() {
		BeforeEach(func() {
			manifest := sdk.MarshalledGenerateManifest{
				Manifest: `name: service-instance_some-instance-id
password: ((odb_secret:foo))`,
				ODBManagedSecrets: map[string]interface{}{
					"foo": "bob",
				},
			}
			manifestBytes, err := json.Marshal(manifest)
			Expect(err).NotTo(HaveOccurred())
			zero := 0
			fakeCommandRunner.RunWithInputParamsReturns(manifestBytes, []byte{}, &zero, nil)
		})

		When("secure manifests are not enabled", func() {
			BeforeEach(func() {
				var nilStore *taskfakes.FakeBulkSetter
				fakeTaskBulkSetter = nilStore
			})

			It("does not substitute odb_secret references", func() {
				resp, _ := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)
				Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

				manifest, _, _, _ := fakeTaskBoshClient.DeployArgsForCall(0)
				Expect(manifest).To(ContainSubstring("((odb_secret:foo))"))
			})
		})

		When("secure manifests are enabled", func() {
			It("stores odb_secret secrets in credhub and replaces manifest placeholders", func() {
				resp, _ := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)
				Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

				manifest, _, _, _ := fakeTaskBoshClient.DeployArgsForCall(0)
				Expect(manifest).To(ContainSubstring(fmt.Sprintf("/odb/%s/service-instance_%s/foo", conf.ServiceCatalog.ID, instanceID)))
				Expect(manifest).ToNot(ContainSubstring("((odb_secret:"))

				Expect(fakeTaskBulkSetter.BulkSetCallCount()).To(Equal(1))
			})
		})
	})

	It("handles the request correctly when CF is disabled", func() {
		fakeCfClient.CountInstancesOfServiceOfferingReturns(make(map[cf.ServicePlan]int), nil)

		By("fulfilling the request when the plan has no quota")
		resp, _ := doProvisionRequest(instanceID, planWithoutQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)
		Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
	})

	It("responds with 202 when the plan has no quota", func() {
		resp, _ := doProvisionRequest(instanceID, planWithoutQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)
		Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
	})

	Context("when the plan has a quota", func() {
		It("successfully provision the service instance", func() {
			fakeTaskBoshClient.DeployReturns(taskID, nil)

			resp, bodyContent := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)

			By("returning http status code 202")
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			By("including the operation data in the response")
			var provisioningResponse brokerapi.ProvisioningResponse
			Expect(json.Unmarshal(bodyContent, &provisioningResponse)).To(Succeed())
			Expect(provisioningResponse.DashboardURL).To(BeEmpty())

			var operationData broker.OperationData
			Expect(json.NewDecoder(strings.NewReader(provisioningResponse.OperationData)).Decode(&operationData)).To(Succeed())

			Expect(operationData.OperationType).To(Equal(broker.OperationTypeCreate), "operation type")
			Expect(operationData.BoshTaskID).To(Equal(taskID), "task id")
			Expect(operationData.BoshContextID).To(BeEmpty(), "context id")
			Expect(operationData.PlanID).To(BeEmpty(), "plan id")

			By("calling the deployer with the correct parameters")

			input, actualOthers := fakeCommandRunner.RunWithInputParamsArgsForCall(0)
			actualInput, ok := input.(sdk.InputParams)
			Expect(ok).To(BeTrue(), "command runner takes a sdk.inputparams obj")
			Expect(actualOthers[1]).To(Equal("generate-manifest"))
			Expect(actualInput.GenerateManifest.ServiceDeployment).To(ContainSubstring(`"deployment_name":"service-instance_` + instanceID + `"`))

			var plan sdk.Plan
			err := json.Unmarshal([]byte(actualInput.GenerateManifest.Plan), &plan)
			Expect(err).NotTo(HaveOccurred())

			Expect(plan.Properties["type"]).To(Equal(planWithQuotaID))

			var reqParams map[string]interface{}
			err = json.Unmarshal([]byte(actualInput.GenerateManifest.RequestParameters), &reqParams)
			Expect(err).NotTo(HaveOccurred())

			Expect(reqParams).To(Equal(map[string]interface{}{
				"plan_id":           planWithQuotaID,
				"service_id":        serviceID,
				"space_guid":        spaceGUID,
				"organization_guid": organizationGUID,
				"parameters":        arbitraryParams,
				"maintenance_info": map[string]interface{}{},
			}))

			_, boshContextID, _, _ := fakeTaskBoshClient.DeployArgsForCall(0)
			Expect(boshContextID).To(BeEmpty())

			By("logging the incoming request")
			Eventually(loggerBuffer).Should(gbytes.Say(`\[.*\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6} Request PUT /v2/service_instances/some-instance-id Completed 202 in [0-9\.]+.* | Start Time: \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6}`))
		})

		It("includes the dashboard url when the adapter returns one", func() {
			fakeTaskBoshClient.DeployReturns(taskID, nil)
			boshManifest := []byte(`name: service-instance_` + instanceID)
			taskManifest := sdk.MarshalledGenerateManifest{
				Manifest: string(boshManifest),
			}
			manifestBytes, err := json.Marshal(taskManifest)
			Expect(err).NotTo(HaveOccurred())
			zero := 0
			fakeCommandRunner.RunWithInputParamsReturns(manifestBytes, []byte{}, &zero, nil)

			fakeServiceAdapter.GenerateDashboardUrlReturns("http://dashboard.example.com", nil)

			resp, bodyContent := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			var provisionResponseBody brokerapi.ProvisioningResponse
			Expect(json.Unmarshal(bodyContent, &provisionResponseBody)).To(Succeed())

			By("calling the adapter with the correct arguments")
			id, plan, manifest, _ := fakeServiceAdapter.GenerateDashboardUrlArgsForCall(0)
			Expect(id).To(Equal(instanceID))
			Expect(err).NotTo(HaveOccurred())
			Expect(plan).To(Equal(sdk.Plan{
				Properties: sdk.Properties{
					"type":            "plan-with-quota",
					"global_property": "global_value",
				},
				Update: dedicatedPlanUpdateBlock,
				InstanceGroups: []sdk.InstanceGroup{
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
			Expect(manifest).To(Equal(boshManifest))

			By("including the dashboard url in the response")
			Expect(provisionResponseBody.DashboardURL).To(Equal("http://dashboard.example.com"))
		})

		It("responds with 500 when generating the dashboard url fails", func() {
			fakeTaskBoshClient.DeployReturns(taskID, nil)
			fakeServiceAdapter.GenerateDashboardUrlReturns("", errors.New("something went wrong"))

			resp, bodyContent := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			var errorResponse brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())
			Expect(errorResponse.Description).To(SatisfyAll(
				ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information: ",
				),
				MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				ContainSubstring(fmt.Sprintf("task-id: %d", taskID)),
				ContainSubstring("operation: create"),
			))
		})

		It("responds with 500 and with a descriptive message when generating the dashboard url fails", func() {
			fakeServiceAdapter.GenerateDashboardUrlReturns("", serviceadapter.NewUnknownFailureError("error message for user"))
			resp, bodyContent := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the error for the CF user")
			var errorResponse brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())
			Expect(errorResponse.Description).To(ContainSubstring("error message for user"))
		})
	})

	It("succeeds when the plan has post-deploy errands configured", func() {
		fakeTaskBoshClient.DeployReturns(taskID, nil)

		resp, bodyContent := doProvisionRequest(instanceID, planWithErrandID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)

		By("returning http status code 202")
		Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

		By("including the operation data in the response")

		var provisioningResponse brokerapi.ProvisioningResponse
		Expect(json.Unmarshal(bodyContent, &provisioningResponse)).To(Succeed())
		Expect(provisioningResponse.DashboardURL).To(BeEmpty())

		var operationData broker.OperationData
		Expect(json.NewDecoder(strings.NewReader(provisioningResponse.OperationData)).Decode(&operationData)).To(Succeed())

		Expect(operationData.OperationType).To(Equal(broker.OperationTypeCreate), "operation type")
		Expect(operationData.BoshTaskID).To(Equal(taskID), "task id")
		Expect(operationData.BoshContextID).NotTo(BeEmpty(), "context id")
		Expect(operationData.PlanID).To(BeEmpty(), "plan id")
		Expect(operationData.Errands[0].Name).To(Equal("health-check"), "post-deploy errand name")
		Expect(operationData.Errands[0].Instances).To(Equal([]string{"health-check-instance/0", "health-check-instance/1"}), "post-deploy errand instances")
	})

	It("responds with 409 when another instance with the same id is provisioned", func() {
		fakeBoshClient.GetDeploymentReturns(nil, true, nil)

		resp, _ := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)
		Expect(resp.StatusCode).To(Equal(http.StatusConflict))

		Expect(loggerBuffer).To(gbytes.Say("already exists"))
	})

	It("responds with 500 when deployer fails to create", func() {
		fakeTaskBoshClient.DeployReturns(0, errors.New("cant create"))

		resp, bodyContent := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)
		Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

		var errorResponse brokerapi.ErrorResponse
		Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())
		Expect(errorResponse.Description).To(SatisfyAll(
			ContainSubstring(
				"There was a problem completing your request. Please contact your operations team providing the following information: ",
			),
			Not(MatchRegexp(
				`error-message:.*`,
			)),
			MatchRegexp(
				`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
			),
			ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
			ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
			Not(ContainSubstring("task-id")),
			ContainSubstring("operation: create"),
		))

		Expect(loggerBuffer).To(gbytes.Say("cant create"))
	})

	Context("when expose_operational_errors is enabled", func() {
		BeforeEach(func() {
			conf.Broker.ExposeOperationalErrors = true
		})

		It("returns the operator error if the deployer fails", func() {
			fakeBoshClient.GetDeploymentReturns([]byte{}, false, errors.New("bosh_server_error!"))
			resp, bodyContent := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			var errorResponse brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())
			Expect(errorResponse.Description).To(SatisfyAll(
				ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information: ",
				),
				MatchRegexp(
					`error-message:.*bosh_server_error!`,
				),
				MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				Not(ContainSubstring("task-id")),
				ContainSubstring("operation: create"),
			))
		})

	})

	It("responds with 500 when the plan quota is reached", func() {
		instanceLimit := 5
		planCounts := map[cf.ServicePlan]int{
			cf.ServicePlan{
				ServicePlanEntity: cf.ServicePlanEntity{
					UniqueID: planWithQuotaID,
				},
			}: instanceLimit,
		}
		fakeCfClient.CountInstancesOfServiceOfferingReturns(planCounts, nil)
		resp, bodyContent := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)
		Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

		var errorResponse map[string]string
		Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())
		Expect(errorResponse).To(Equal(map[string]string{
			"description": fmt.Sprintf("plan instance limit exceeded for service ID: %s. Total instances: %d", serviceID, instanceLimit),
		}))
	})

	It("responds with 500 when the global quota is reached", func() {
		servicePlan := cf.ServicePlan{
			ServicePlanEntity: cf.ServicePlanEntity{
				UniqueID: planWithQuotaID,
			},
		}
		instanceLimit := 12
		fakeCfClient.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{servicePlan: globalQuota}, nil)
		resp, bodyContent := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)
		Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

		var errorResponse map[string]string
		Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())
		Expect(errorResponse).To(Equal(map[string]string{
			"description": fmt.Sprintf("plan instance limit exceeded for service ID: %s. Total instances: %d, global instance limit exceeded for service ID: %s. Total instances: %d", serviceID, instanceLimit, serviceID, instanceLimit),
		}))
	})

	It("responds with 422 when async is set to false", func() {
		resp, bodyContent := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, false)
		Expect(resp.StatusCode).To(Equal(http.StatusUnprocessableEntity))

		Expect(bodyContent).To(MatchJSON(`{
			"error":"AsyncRequired",
			"description":"This service plan requires client support for asynchronous service operations."
		}`))
	})

	It("responds with 500 when bosh is unavailable", func() {
		fakeBoshClient.GetInfoReturns(boshdirector.Info{}, errors.New("boom"))
		resp, bodyContent := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, brokerapi.MaintenanceInfo{}, true)
		Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

		var errorResponse brokerapi.ErrorResponse
		Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())
		Expect(errorResponse.Description).To(ContainSubstring("Currently unable to create service instance, please try again later"))
	})

	Context("passing maintenance_info with the provision request", func() {
		// have the maintenance_info we're going to pass in
		brokerMaintenanceInfo := brokerConfig.MaintenanceInfo{
			Public: map[string]string{
				"foo": "bar",
			},
			Private: map[string]string{
				"Secret": "superSecret",
			},
		}
		requestMaintenanceInfo := brokerapi.MaintenanceInfo{
			Public: map[string]string{
				"foo": "bar",
			},
			Private: "Secret:superSecret;", // this matches the hash stub function
		}

		When("maintenance_info matches that stored on the broker", func() {
			// set maintenance_info in the conf (before each)
			BeforeEach(func() {
				conf.ServiceCatalog.MaintenanceInfo = &brokerMaintenanceInfo
			})

			It("accepts the provision request", func() {
				resp, _ := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, requestMaintenanceInfo, true)
				Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
			})
		})

		When("maintenance_info does not match that stored on the broker", func() {
			// set dodgy maintenance_info on the conf (before each)
			BeforeEach(func() {
				conf.ServiceCatalog.MaintenanceInfo = &brokerMaintenanceInfo
				requestMaintenanceInfo.Public["extra"] = "testing"
			})
			It("returns UnprocessableEntity with maintenanceInfoConflict error", func() {
				resp, bodyContent := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, requestMaintenanceInfo, true)
				Expect(resp.StatusCode).To(Equal(http.StatusUnprocessableEntity))

				Expect(bodyContent).To(MatchJSON(`{
					"error":"MaintenanceInfoConflict",
					"description":"passed maintenance_info does not match the catalog maintenance_info"
				}`))
			})
		})

		When("the broker does not have maintenance_info", func() {
			// leave no maintenance_info on the conf (before each)
			It("returns UnprocessableEntity with maintenanceInfoConflict error", func() {
				resp, bodyContent := doProvisionRequest(instanceID, planWithQuotaID, arbitraryParams, requestMaintenanceInfo, true)
				Expect(resp.StatusCode).To(Equal(http.StatusUnprocessableEntity))

				Expect(bodyContent).To(MatchJSON(`{
					"error":"MaintenanceInfoConflict",
					"description":"maintenance_info was passed, but the broker catalog contains no maintenance_info"
				}`))
			})
		})
	})
})

func doProvisionRequest(instanceID, planID string, arbitraryParams map[string]interface{}, maintenanceInfo brokerapi.MaintenanceInfo, asyncAllowed bool) (*http.Response, []byte) {
	reqBody := map[string]interface{}{
		"plan_id":           planID,
		"space_guid":        spaceGUID,
		"organization_guid": organizationGUID,
		"parameters":        arbitraryParams,
		"service_id":        serviceID,
		"maintenance_info":  maintenanceInfo,
	}
	bodyBytes, err := json.Marshal(reqBody)
	Expect(err).NotTo(HaveOccurred())

	return doRequest(
		http.MethodPut,
		fmt.Sprintf("http://%s/v2/service_instances/%s?accepts_incomplete=%t&plan_id=%s&service_id=%s",
			serverURL, instanceID, asyncAllowed, planID, serviceID,
		),
		bytes.NewReader(bodyBytes),
	)
}
