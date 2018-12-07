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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

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
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"github.com/pkg/errors"
)

var _ = Describe("Update a service instance", func() {

	const (
		oldPlanID              = "old-plan-id"
		newPlanID              = "new-plan-id"
		postDeployErrandPlanID = "with-post-deploy"
		quotaReachedPlanID     = "quota-reached"
		instanceID             = "some-instance-id"
		updateTaskID           = 43
	)

	var (
		detailsMap         brokerapi.UpdateDetails
		updateArbParams    map[string]interface{}
		expectedSecretsMap map[string]string
		conf               brokerConfig.Config
		oldManifest        []byte
	)

	BeforeEach(func() {
		zero := 0
		one := 1
		conf = brokerConfig.Config{
			Broker: brokerConfig.Broker{
				Port: serverPort, Username: brokerUsername, Password: brokerPassword,
			},
			ServiceCatalog: brokerConfig.ServiceOffering{
				Name: serviceName,
				ID:   "service-id",
				MaintenanceInfo: &brokerConfig.MaintenanceInfo{
					Public: map[string]string{
						"version": "2",
					},
				},
				Plans: brokerConfig.Plans{
					{Name: "some-plan", ID: oldPlanID},
					{Name: "other-plan", ID: newPlanID, Quotas: brokerConfig.Quotas{ServiceInstanceLimit: &one}},
					{
						Name: "post-deploy-errand-plan",
						ID:   postDeployErrandPlanID,
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
								Name: "health-check",
							}},
						},
					},
					{
						Name:   "failed-quota-plan",
						ID:     quotaReachedPlanID,
						Quotas: brokerConfig.Quotas{ServiceInstanceLimit: &zero},
					},
				},
			},
		}

		updateArbParams = map[string]interface{}{"some": "params"}
		detailsMap = brokerapi.UpdateDetails{
			PlanID:        newPlanID,
			RawParameters: toJson(updateArbParams),
			ServiceID:     serviceID,
			PreviousValues: brokerapi.PreviousValues{
				OrgID:     organizationGUID,
				ServiceID: serviceID,
				PlanID:    oldPlanID,
				SpaceID:   "space-guid",
			},
		}
		expectedSecretsMap = map[string]string{
			"secret": "value",
		}
		oldManifest = []byte(`name: service-instance_some-instance-id`)
		StartServer(conf)
	})

	Describe("switching plans", func() {
		It("succeeds when there are no pending changes", func() {
			fakeTaskBoshClient.GetDeploymentReturns(oldManifest, true, nil)
			fakeTaskBoshClient.DeployReturns(updateTaskID, nil)

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)
			By("returning the correct status code")
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			By("including the operation data in the response")
			var updateResponse brokerapi.UpdateResponse
			Expect(json.Unmarshal(bodyContent, &updateResponse)).To(Succeed())

			var operationData broker.OperationData
			Expect(json.NewDecoder(strings.NewReader(updateResponse.OperationData)).Decode(&operationData)).To(Succeed())
			Expect(operationData).To(Equal(broker.OperationData{
				OperationType: broker.OperationTypeUpdate,
				BoshTaskID:    updateTaskID,
			}))

			By("logging the bind request")
			Eventually(loggerBuffer).Should(gbytes.Say(`updating instance ` + instanceID))
		})

		It("succeeds when the new plan has a post-deploy errand", func() {
			fakeTaskBoshClient.GetDeploymentReturns(oldManifest, true, nil)
			fakeTaskBoshClient.DeployReturns(updateTaskID, nil)

			detailsMap.PlanID = postDeployErrandPlanID
			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)
			By("returning the correct status code")
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			By("calling the adapter with the correct arguments")
			_, boshContextId, _, _ := fakeTaskBoshClient.DeployArgsForCall(0)

			input, actualOthers := fakeCommandRunner.RunWithInputParamsArgsForCall(1)
			actualInput, ok := input.(sdk.InputParams)
			Expect(ok).To(BeTrue(), "command runner takes a sdk.inputparams obj")
			Expect(actualOthers[1]).To(Equal("generate-manifest"))
			Expect(actualInput.GenerateManifest.ServiceDeployment).To(ContainSubstring("service-instance_" + instanceID))

			var plan sdk.Plan
			err := json.Unmarshal([]byte(actualInput.GenerateManifest.Plan), &plan)
			Expect(err).NotTo(HaveOccurred())

			Expect(plan.LifecycleErrands.PostDeploy).To(Equal(conf.ServiceCatalog.Plans[2].LifecycleErrands.PostDeploy))

			var reqParams brokerapi.UpdateDetails
			err = json.Unmarshal([]byte(actualInput.GenerateManifest.RequestParameters), &reqParams)
			Expect(err).NotTo(HaveOccurred())

			Expect(reqParams).To(Equal(detailsMap))

			By("including the operation data in the response")
			var updateResponse brokerapi.UpdateResponse
			Expect(json.Unmarshal(bodyContent, &updateResponse)).To(Succeed())

			var operationData broker.OperationData
			Expect(json.NewDecoder(strings.NewReader(updateResponse.OperationData)).Decode(&operationData)).To(Succeed())
			Expect(operationData).To(Equal(broker.OperationData{
				OperationType: broker.OperationTypeUpdate,
				BoshTaskID:    updateTaskID,
				BoshContextID: boshContextId,
				Errands:       []brokerConfig.Errand{{Name: "health-check"}},
			}))

			By("logging the bind request")
			Eventually(loggerBuffer).Should(gbytes.Say(`updating instance ` + instanceID))
		})

		It("succeeds when the old plan has a post-deploy errand but the new one doesn't", func() {
			fakeTaskBoshClient.GetDeploymentReturns(oldManifest, true, nil)
			fakeTaskBoshClient.DeployReturns(updateTaskID, nil)

			detailsMap.PreviousValues = brokerapi.PreviousValues{
				OrgID:     organizationGUID,
				ServiceID: serviceID,
				PlanID:    postDeployErrandPlanID,
				SpaceID:   "space-guid",
			}

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)
			By("returning the correct status code")
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			By("calling the adapter with the correct arguments")
			_, boshContextId, _, _ := fakeTaskBoshClient.DeployArgsForCall(0)
			Expect(boshContextId).To(BeEmpty())

			input, actualOthers := fakeCommandRunner.RunWithInputParamsArgsForCall(1)
			actualInput, ok := input.(sdk.InputParams)
			Expect(ok).To(BeTrue(), "command runner takes a sdk.inputparams obj")
			Expect(actualOthers[1]).To(Equal("generate-manifest"))
			Expect(actualInput.GenerateManifest.ServiceDeployment).To(ContainSubstring("service-instance_" + instanceID))

			var plan sdk.Plan
			err := json.Unmarshal([]byte(actualInput.GenerateManifest.Plan), &plan)
			Expect(err).NotTo(HaveOccurred())
			Expect(plan.LifecycleErrands.PostDeploy).To(BeNil())

			var previousPlan sdk.Plan
			err = json.Unmarshal([]byte(actualInput.GenerateManifest.PreviousPlan), &previousPlan)
			Expect(err).NotTo(HaveOccurred())
			Expect(previousPlan.LifecycleErrands.PostDeploy).To(Equal(conf.ServiceCatalog.Plans[2].LifecycleErrands.PostDeploy))

			var reqParams brokerapi.UpdateDetails
			err = json.Unmarshal([]byte(actualInput.GenerateManifest.RequestParameters), &reqParams)
			Expect(err).NotTo(HaveOccurred())

			Expect(reqParams).To(Equal(detailsMap))

			By("including the operation data in the response")
			var updateResponse brokerapi.UpdateResponse
			Expect(json.Unmarshal(bodyContent, &updateResponse)).To(Succeed())

			var operationData broker.OperationData
			Expect(json.NewDecoder(strings.NewReader(updateResponse.OperationData)).Decode(&operationData)).To(Succeed())
			Expect(operationData).To(Equal(broker.OperationData{
				OperationType: broker.OperationTypeUpdate,
				BoshTaskID:    updateTaskID,
			}))

			By("logging the bind request")
			Eventually(loggerBuffer).Should(gbytes.Say(`updating instance ` + instanceID))
		})

		It("fails with 422 if there are pending changes", func() {
			fakeTaskBoshClient.GetDeploymentReturns([]byte("name: service-instance_some-instance-id"), true, nil)

			boshManifest := []byte(`---
name: service-instance_some-instance-id
properties:
  bob: like-a-duck`)
			manifest := sdk.MarshalledGenerateManifest{
				Manifest: string(boshManifest),
			}
			manifestBytes, err := json.Marshal(manifest)
			Expect(err).NotTo(HaveOccurred())
			zero := 0

			fakeCommandRunner.RunWithInputParamsReturns(manifestBytes, []byte{}, &zero, nil)

			resp, _ := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusUnprocessableEntity))
		})

		It("fails with 500 if there plan's quota has been reached", func() {
			fakeCfClient.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{
				cf.ServicePlan{ServicePlanEntity: cf.ServicePlanEntity{UniqueID: quotaReachedPlanID}}: 1,
			}, nil)
			detailsMap.PlanID = quotaReachedPlanID
			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(ContainSubstring(
				"plan instance limit exceeded for service ID: service-id. Total instances: 1",
			))
		})

		It("fails with 500 if BOSH deployment cannot be found", func() {
			fakeTaskBoshClient.GetDeploymentReturns(nil, false, nil)

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(SatisfyAll(
				Not(ContainSubstring("task-id:")),
				ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information: "),
				MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				ContainSubstring("operation: update"),
			))

			Eventually(loggerBuffer).Should(gbytes.Say("error deploying instance: bosh deployment 'service-instance_some-instance-id' not found"))
		})

		It("fails with 500 if BOSH is an operation is in progress", func() {
			fakeTaskBoshClient.GetTasksReturns(boshdirector.BoshTasks{
				boshdirector.BoshTask{State: boshdirector.TaskProcessing},
			}, nil)

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(ContainSubstring("An operation is in progress for your service instance. Please try again later."))
		})

		It("fails with 500 if BOSH is unavailable", func() {
			fakeTaskBoshClient.GetTasksReturns(nil, errors.New("oops"))

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(ContainSubstring("Currently unable to update service instance, please try again later"))
		})

		It("fails with 500 if CF api is unavailable", func() {
			fakeCfClient.CountInstancesOfServiceOfferingReturns(nil, errors.New("oops"))

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(SatisfyAll(
				Not(ContainSubstring("task-id:")),
				ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information: "),
				MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				ContainSubstring("operation: update"),
			))
		})

		It("fails with 500 if the previous plan cannot be found", func() {
			fakeTaskBoshClient.GetDeploymentReturns(nil, true, nil)

			detailsMap.PlanID = "yo"
			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(ContainSubstring("Plan yo not found"))
		})

		It("fails with 500 if the new plan cannot be found", func() {
			detailsMap.PlanID = "macarena"
			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(ContainSubstring("Plan macarena not found"))
		})

		It("fails with 500 when the adapter errors", func() {
			err := serviceadapter.ErrorForExitCode(400, "")
			fourHundred := 400
			fakeCommandRunner.RunWithInputParamsReturns([]byte{}, []byte{}, &fourHundred, err)
			fakeTaskBoshClient.GetDeploymentReturns(nil, true, nil)

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(SatisfyAll(
				Not(ContainSubstring("task-id:")),
				ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information: "),
				MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				ContainSubstring("operation: update"),
			))
		})

		It("fails with 500 and a generic message, with adapter stderr in the logs, when adapter returns non-zero exit code", func() {
			one := 1
			fakeCommandRunner.RunWithInputParamsReturns([]byte{}, []byte("stderr error message"), &one, nil)
			fakeTaskBoshClient.GetDeploymentReturns(nil, true, nil)

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(ContainSubstring("There was a problem completing your request"))
			Eventually(loggerBuffer).Should(gbytes.Say("stderr error message"))
		})

		It("fails with 500 when BOSH cannot get the deployment for manifest secrets", func() {
			fakeBoshClient.GetDeploymentReturns(nil, false, errors.New("error getting deployment"))

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(SatisfyAll(
				ContainSubstring("There was a problem completing your request. "),
				ContainSubstring("operation: update"),
			))

			Eventually(loggerBuffer).Should(gbytes.Say("error getting deployment"))
		})

		It("fails with 500 when BOSH cannot get deployment variables", func() {
			fakeBoshClient.VariablesReturns(nil, errors.New("error getting variables"))

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(SatisfyAll(
				ContainSubstring("There was a problem completing your request. "),
				ContainSubstring("operation: update"),
			))

			Eventually(loggerBuffer).Should(gbytes.Say("error getting variables"))
		})

		It("fails with 500 when the secret manager cannot resolve secrets", func() {
			fakeCredhubOperator.BulkGetReturns(nil, errors.New("could not get secrets"))

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(SatisfyAll(
				ContainSubstring("There was a problem completing your request. "),
				ContainSubstring("operation: update"),
			))

			Eventually(loggerBuffer).Should(gbytes.Say("could not get secrets"))
		})
	})

	Describe("without switching plans", func() {
		It("succeeds even when the quota has been reached", func() {
			fakeTaskBoshClient.GetDeploymentReturns(oldManifest, true, nil)
			fakeTaskBoshClient.DeployReturns(updateTaskID, nil)
			fakeCredhubOperator.BulkGetReturns(expectedSecretsMap, nil)

			detailsMap.PlanID = quotaReachedPlanID
			detailsMap.PreviousValues = brokerapi.PreviousValues{
				OrgID:     organizationGUID,
				ServiceID: serviceID,
				PlanID:    quotaReachedPlanID,
				SpaceID:   "space-guid",
			}

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)
			By("returning the correct status code")
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			By("calling the adapter with the correct arguments")
			input, actualOthers := fakeCommandRunner.RunWithInputParamsArgsForCall(1)
			actualInput, ok := input.(sdk.InputParams)
			Expect(ok).To(BeTrue(), "command runner takes a sdk.inputparams obj")
			Expect(actualOthers[1]).To(Equal("generate-manifest"))
			Expect(actualInput.GenerateManifest.ServiceDeployment).To(ContainSubstring("service-instance_" + instanceID))

			expectedSecretsMapJson, err := json.Marshal(expectedSecretsMap)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualInput.GenerateManifest.PreviousSecrets).To(Equal(string(expectedSecretsMapJson)))

			var plan sdk.Plan
			err = json.Unmarshal([]byte(actualInput.GenerateManifest.Plan), &plan)
			Expect(err).NotTo(HaveOccurred())

			var previousPlan sdk.Plan
			err = json.Unmarshal([]byte(actualInput.GenerateManifest.PreviousPlan), &previousPlan)
			Expect(err).NotTo(HaveOccurred())
			Expect(plan).To(Equal(previousPlan))

			var reqParams brokerapi.UpdateDetails
			err = json.Unmarshal([]byte(actualInput.GenerateManifest.RequestParameters), &reqParams)
			Expect(err).NotTo(HaveOccurred())

			Expect(reqParams).To(Equal(detailsMap))

			_, boshContextId, _, _ := fakeTaskBoshClient.DeployArgsForCall(0)
			Expect(boshContextId).To(BeEmpty())

			By("including the operation data in the response")
			var updateResponse brokerapi.UpdateResponse
			Expect(json.Unmarshal(bodyContent, &updateResponse)).To(Succeed())

			var operationData broker.OperationData
			Expect(json.NewDecoder(strings.NewReader(updateResponse.OperationData)).Decode(&operationData)).To(Succeed())
			Expect(operationData).To(Equal(broker.OperationData{
				OperationType: broker.OperationTypeUpdate,
				BoshTaskID:    updateTaskID,
			}))

			By("logging the bind request")
			Eventually(loggerBuffer).Should(gbytes.Say(`updating instance ` + instanceID))
		})
	})

	When("upgrading with maintenance_info", func() {
		BeforeEach(func() {
			oldManifest := "name: service-instance_some-instance-id"
			fakeTaskBoshClient.GetDeploymentReturns([]byte(oldManifest), true, nil)

			regeneratedManifest := []byte(`
name: service-instance_some-instance-id
properties:
  bob: like-a-duck`)
			manifest := sdk.MarshalledGenerateManifest{Manifest: string(regeneratedManifest)}
			manifestBytes, err := json.Marshal(manifest)
			Expect(err).NotTo(HaveOccurred())
			zero := 0
			fakeCommandRunner.RunWithInputParamsReturns(manifestBytes, []byte{}, &zero, nil)
			detailsMap.MaintenanceInfo = brokerapi.MaintenanceInfo{
				Public: map[string]string{
					"version": "2",
				},
			}
			detailsMap.PlanID = oldPlanID
			detailsMap.RawParameters = []byte("{}")
		})

		It("succeeds", func() {
			resp, _ := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
		})

		It("fails when maintenance_info doesn't match the catalog maintenance_info", func() {
			detailsMap.MaintenanceInfo = brokerapi.MaintenanceInfo{
				Public: map[string]string{
					"version": "3",
				},
			}

			resp, body := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusUnprocessableEntity))
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/json"))
			bodyJSON := map[string]interface{}{}
			err := json.Unmarshal(body, &bodyJSON)
			Expect(err).NotTo(HaveOccurred())
			Expect(bodyJSON["error"]).To(Equal("MaintenanceInfoConflict"))
		})

		It("fails when arbitrary params are passed in", func() {
			detailsMap.RawParameters = []byte(`{"foo":"bar"}`)
			resp, _ := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusUnprocessableEntity))
		})

		It("fails when a new plan is passed", func() {
			detailsMap.PlanID = newPlanID
			resp, _ := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusUnprocessableEntity))
		})
	})
})

func doUpdateRequest(body interface{}, instanceID string) (*http.Response, []byte) {
	bodyBytes, err := json.Marshal(body)
	Expect(err).NotTo(HaveOccurred())
	return doRequest(
		http.MethodPatch,
		fmt.Sprintf("http://%s/v2/service_instances/%s?accepts_incomplete=true", serverURL, instanceID),
		bytes.NewReader(bodyBytes),
	)
}
