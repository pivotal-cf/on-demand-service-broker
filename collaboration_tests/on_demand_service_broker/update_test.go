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
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/task"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"github.com/pkg/errors"
)

var _ = Describe("Update a service instance", func() {

	const (
		oldPlanID              = "old-plan-id"
		newPlanID              = "new-plan-id"
		postDeployErrandPlanID = "with-post-deploy"
		quotaReachedPlanID     = "quota-reached"
		instanceID             = "some-instance"
		updateTaskID           = 43
	)

	var (
		detailsMap      map[string]interface{}
		updateArbParams map[string]interface{}
	)

	BeforeEach(func() {
		zero := 0
		one := 1
		conf := brokerConfig.Config{
			Broker: brokerConfig.Broker{
				Port: serverPort, Username: brokerUsername, Password: brokerPassword,
			},
			ServiceCatalog: brokerConfig.ServiceOffering{
				Name: serviceName,
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
		detailsMap = map[string]interface{}{
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

		StartServer(conf)
	})

	Describe("switching plans", func() {
		It("succeeds when there are no pending changes", func() {
			fakeDeployer.UpdateReturns(updateTaskID, nil, nil)

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)
			By("returning the correct status code")
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			By("calling the adapter with the correct arguments")
			deploymentName, planId, details, previousPlanID, boshContextId, _ := fakeDeployer.UpdateArgsForCall(0)
			Expect(deploymentName).To(Equal("service-instance_" + instanceID))
			Expect(planId).To(Equal(newPlanID))
			Expect(details).To(Equal(detailsMap))
			Expect(*previousPlanID).To(Equal(oldPlanID))
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

		It("succeeds when the new plan has a post-deploy errand", func() {
			fakeDeployer.UpdateReturns(updateTaskID, nil, nil)

			detailsMap["plan_id"] = postDeployErrandPlanID
			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)
			By("returning the correct status code")
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			By("calling the adapter with the correct arguments")
			deploymentName, planId, details, previousPlanID, boshContextId, _ := fakeDeployer.UpdateArgsForCall(0)
			Expect(deploymentName).To(Equal("service-instance_" + instanceID))
			Expect(planId).To(Equal(postDeployErrandPlanID))
			Expect(details).To(Equal(detailsMap))
			Expect(*previousPlanID).To(Equal(oldPlanID))
			Expect(boshContextId).NotTo(BeEmpty())

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
			fakeDeployer.UpdateReturns(updateTaskID, nil, nil)

			detailsMap["previous_values"] = map[string]interface{}{
				"organization_id": organizationGUID,
				"service_id":      serviceID,
				"plan_id":         postDeployErrandPlanID,
				"space_id":        "space-guid",
			}
			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)
			By("returning the correct status code")
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			By("calling the adapter with the correct arguments")
			deploymentName, planId, details, previousPlanID, boshContextId, _ := fakeDeployer.UpdateArgsForCall(0)
			Expect(deploymentName).To(Equal("service-instance_" + instanceID))
			Expect(planId).To(Equal(newPlanID))
			Expect(details).To(Equal(detailsMap))
			Expect(*previousPlanID).To(Equal(postDeployErrandPlanID))
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

		It("fails with 422 if there are pending changes", func() {
			fakeDeployer.UpdateReturns(updateTaskID, nil, task.PendingChangesNotAppliedError{})

			resp, _ := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusUnprocessableEntity))
		})

		It("fails with 500 if there plan's quota has been reached", func() {
			detailsMap["plan_id"] = quotaReachedPlanID
			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(Equal(
				"The quota for this service plan has been exceeded. Please contact your Operator for help.",
			))
		})

		It("fails with 500 if BOSH deployment cannot be found", func() {
			fakeDeployer.UpdateReturns(updateTaskID, nil, task.NewDeploymentNotFoundError(fmt.Errorf("oops")))

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

			Eventually(loggerBuffer).Should(gbytes.Say("error deploying instance: oops"))
		})

		It("fails with 500 if BOSH is an operation is in progress", func() {
			fakeDeployer.UpdateReturns(updateTaskID, nil, task.TaskInProgressError{})

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(ContainSubstring("An operation is in progress for your service instance. Please try again later."))
		})

		It("fails with 500 if BOSH is unavailable", func() {
			fakeDeployer.UpdateReturns(updateTaskID, nil, task.ServiceError{})

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(ContainSubstring("Currently unable to update service instance, please try again later"))
		})

		It("fails with 500 if CF api is unavailable", func() {
			fakeCfClient.CountInstancesOfPlanReturns(0, errors.New("oops"))

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
			fakeDeployer.UpdateReturns(updateTaskID, nil, task.PlanNotFoundError{PlanGUID: "yo"})

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(ContainSubstring("plan yo does not exist"))
		})

		It("fails with 500 if the new plan cannot be found", func() {
			detailsMap["plan_id"] = "macarena"
			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(ContainSubstring("Plan macarena not found"))
		})

		It("fails with 500 when the adapter errors", func() {
			err := serviceadapter.ErrorForExitCode(400, "")
			fakeDeployer.UpdateReturns(updateTaskID, nil, err)

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

		It("fails with 500 and a descriptive message when the adapter errors", func() {
			err := serviceadapter.ErrorForExitCode(1, "some cf message")
			fakeDeployer.UpdateReturns(updateTaskID, nil, err)

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			var body brokerapi.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &body)).To(Succeed())
			Expect(body.Description).To(ContainSubstring("some cf message"))
		})
	})

	Describe("without switching plans", func() {
		It("succeeds even when the quota has been reached", func() {
			fakeDeployer.UpdateReturns(updateTaskID, nil, nil)
			detailsMap["plan_id"] = quotaReachedPlanID
			detailsMap["previous_values"] = map[string]interface{}{
				"organization_id": organizationGUID,
				"service_id":      serviceID,
				"plan_id":         quotaReachedPlanID,
				"space_id":        "space-guid",
			}

			resp, bodyContent := doUpdateRequest(detailsMap, instanceID)
			By("returning the correct status code")
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			By("calling the adapter with the correct arguments")
			deploymentName, planId, details, previousPlanID, boshContextId, _ := fakeDeployer.UpdateArgsForCall(0)
			Expect(deploymentName).To(Equal("service-instance_" + instanceID))
			Expect(planId).To(Equal(quotaReachedPlanID))
			Expect(details).To(Equal(detailsMap))
			Expect(*previousPlanID).To(Equal(quotaReachedPlanID))
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
})

func doUpdateRequest(body map[string]interface{}, instanceID string) (*http.Response, []byte) {
	bodyBytes, err := json.Marshal(body)
	Expect(err).NotTo(HaveOccurred())
	return doRequest(
		http.MethodPatch,
		fmt.Sprintf("http://%s/v2/service_instances/%s?accepts_incomplete=true", serverURL, instanceID),
		bytes.NewReader(bodyBytes),
	)
}
