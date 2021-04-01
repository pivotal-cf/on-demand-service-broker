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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/brokerapi/v8/domain/apiresponses"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("Deprovision", func() {
	const (
		instanceID = "some-instance-id"
		taskID     = 42
	)

	BeforeEach(func() {
		boshManifest := []byte(`name: 123`)
		boshVMs := bosh.BoshVMs{"some-property": []string{"some-value"}}
		fakeBoshClient.VMsReturns(boshVMs, nil)
		fakeBoshClient.GetDeploymentReturns(boshManifest, true, nil)
		fakeBoshClient.DeleteDeploymentReturnsOnCall(0, taskID, nil)
	})

	Context("without pre-delete errand", func() {
		BeforeEach(func() {
			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name: serviceName,
				},
			}

			StartServer(conf)
		})

		It("succeeds with async set to true", func() {
			response, bodyContent := doDeprovisionRequest(instanceID, dedicatedPlanID, serviceID, true, false)

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))

			By("including the operation data in the response")
			var deprovisionResponse apiresponses.DeprovisionResponse
			Expect(json.Unmarshal(bodyContent, &deprovisionResponse)).To(Succeed())

			var operationData broker.OperationData
			Expect(json.NewDecoder(strings.NewReader(deprovisionResponse.OperationData)).Decode(&operationData)).To(Succeed())

			Expect(operationData.OperationType).To(Equal(broker.OperationTypeDelete))
			Expect(operationData.BoshTaskID).To(Equal(taskID))
			Expect(operationData.BoshContextID).To(BeEmpty())

			By("logging the delete request")
			Eventually(loggerBuffer).Should(gbytes.Say(`removing deployment for instance`))
		})

		It("succeeds with async and force delete set to true", func() {
			expectedForceDelete := true
			response, bodyContent := doDeprovisionRequest(instanceID, dedicatedPlanID, serviceID, true, expectedForceDelete)

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))

			By("including the operation data in the response")
			var deprovisionResponse apiresponses.DeprovisionResponse
			Expect(json.Unmarshal(bodyContent, &deprovisionResponse)).To(Succeed())

			var operationData broker.OperationData
			Expect(json.NewDecoder(strings.NewReader(deprovisionResponse.OperationData)).Decode(&operationData)).To(Succeed())

			Expect(operationData.OperationType).To(Equal(broker.OperationTypeForceDelete))
			Expect(operationData.BoshTaskID).To(Equal(taskID))
			Expect(operationData.BoshContextID).To(BeEmpty())

			By("logging the delete request")
			Eventually(loggerBuffer).Should(gbytes.Say(`removing deployment for instance`))

			By("Passing force flag to DeleteDeployment")
			_, _, force, _, _ := fakeBoshClient.DeleteDeploymentArgsForCall(0)
			Expect(force).To(Equal(expectedForceDelete))
		})

		It("fails when async flag is not set", func() {
			response, bodyContent := doDeprovisionRequest(instanceID, dedicatedPlanID, serviceID, false, false)

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusUnprocessableEntity))

			By("returning an informative error")
			var respStructure map[string]interface{}
			Expect(json.Unmarshal(bodyContent, &respStructure)).To(Succeed())

			Expect(respStructure).To(Equal(map[string]interface{}{
				"error":       "AsyncRequired",
				"description": "This service plan requires client support for asynchronous service operations.",
			}))
		})

	})

	Context("with pre-delete errand", func() {
		const (
			preDeleteErrandPlanID = "pre-delete-errand-id"
			errandTaskID          = 187
		)

		BeforeEach(func() {
			errandName := "cleanup-resources"

			preDeleteErrandPlan := brokerConfig.Plan{
				Name: "pre-delete-errand-plan",
				ID:   preDeleteErrandPlanID,
				InstanceGroups: []sdk.InstanceGroup{
					{
						Name:      "instance-group-name",
						VMType:    "pre-delete-errand-vm-type",
						Instances: 1,
						Networks:  []string{"net1"},
						AZs:       []string{"az1"},
					},
				},
				LifecycleErrands: &sdk.LifecycleErrands{
					PreDelete: []sdk.Errand{{Name: errandName}},
				},
			}

			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name:  serviceName,
					Plans: brokerConfig.Plans{preDeleteErrandPlan},
				},
			}

			fakeBoshClient.RunErrandReturns(errandTaskID, nil)
			StartServer(conf)
		})

		It("succeeds with async flag", func() {
			response, bodyContent := doDeprovisionRequest(instanceID, preDeleteErrandPlanID, serviceID, true, false)

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))

			By("including the operation data in the response")
			var deprovisionResponse apiresponses.DeprovisionResponse
			Expect(json.Unmarshal(bodyContent, &deprovisionResponse)).To(Succeed())

			var operationData broker.OperationData
			Expect(json.NewDecoder(strings.NewReader(deprovisionResponse.OperationData)).Decode(&operationData)).To(Succeed())

			Expect(operationData.OperationType).To(Equal(broker.OperationTypeDelete))
			Expect(operationData.BoshTaskID).To(Equal(errandTaskID))
			Expect(operationData.BoshContextID).To(MatchRegexp(
				`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
			))

			By("logging the delete request")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf("running pre-delete errand for instance %s", instanceID),
			))

			By("Not calling DeleteDeployment")
			Expect(fakeBoshClient.DeleteConfigCallCount()).To(Equal(0))
		})

		It("succeeds when force deleting", func() {
			expectedForceDelete := true
			response, bodyContent := doDeprovisionRequest(instanceID, preDeleteErrandPlanID, serviceID, true, expectedForceDelete)

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))

			By("including the operation data in the response")
			var deprovisionResponse apiresponses.DeprovisionResponse
			Expect(json.Unmarshal(bodyContent, &deprovisionResponse)).To(Succeed())

			var operationData broker.OperationData
			Expect(json.NewDecoder(strings.NewReader(deprovisionResponse.OperationData)).Decode(&operationData)).To(Succeed())

			Expect(operationData.OperationType).To(Equal(broker.OperationTypeForceDelete))

			By("Not calling DeleteDeployment")
			Expect(fakeBoshClient.DeleteConfigCallCount()).To(Equal(0))
		})
	})

	Context("the failure scenarios", func() {
		var conf brokerConfig.Config

		BeforeEach(func() {
			conf = brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword, ExposeOperationalErrors: true,
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name: serviceName,
				},
			}
		})

		JustBeforeEach(func() {
			StartServer(conf)
		})

		Context("when the deployment does not exist", func() {
			BeforeEach(func() {
				fakeBoshClient.GetDeploymentReturns(nil, false, nil)
			})

			It("returns 410 when bosh configs and secrets can be removed", func() {
				response, bodyContent := doDeprovisionRequest(instanceID, dedicatedPlanID, serviceID, true, false)

				By("returning the correct HTTP status")
				Expect(response.StatusCode).To(Equal(http.StatusGone))

				By("returning no body")
				Expect(bodyContent).To(MatchJSON("{}"))

				By("logging the delete request")
				Eventually(loggerBuffer).Should(
					gbytes.Say(fmt.Sprintf("error deprovisioning: instance %s, not found.", instanceID)),
				)
			})

			It("returns 500 when configs removal fails", func() {
				fakeBoshClient.DeleteConfigsReturns(errors.New("not today, thank you"))

				response, bodyContent := doDeprovisionRequest(instanceID, dedicatedPlanID, serviceID, true, false)

				By("returning the correct HTTP status")
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

				By("returning the correct error data")
				var errorResponse apiresponses.ErrorResponse
				Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())
				Expect(errorResponse.Description).To(ContainSubstring(
					"Unable to delete service. Please try again later",
				))

				By("logging the delete request")
				Eventually(loggerBuffer).Should(
					gbytes.Say(fmt.Sprintf("error deprovisioning: failed to delete configs for instance service-instance_%s", instanceID)),
				)
			})

			It("returns 500 when config removal succeeds but secret removal fails", func() {
				fakeCredhubOperator.FindNameLikeReturns(nil, errors.New("not today, thank you"))

				response, bodyContent := doDeprovisionRequest(instanceID, dedicatedPlanID, serviceID, true, false)

				By("returning the correct HTTP status")
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

				By("returning the correct error data")
				var errorResponse apiresponses.ErrorResponse
				Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())
				Expect(errorResponse.Description).To(ContainSubstring(
					"Unable to delete service. Please try again later",
				))

				By("logging the delete request")
				Eventually(loggerBuffer).Should(
					gbytes.Say(fmt.Sprintf("error deprovisioning: failed to delete secrets for instance service-instance_%s", instanceID)),
				)
			})

			Context("and disable_bosh_configs is true", func() {
				BeforeEach(func() {
					conf.Broker.DisableBoshConfigs = true
				})

				It("should not get or delete configs", func() {
					doDeprovisionRequest(instanceID, dedicatedPlanID, serviceID, true, false)

					Expect(fakeBoshClient.DeleteConfigsCallCount()).To(Equal(0), "DeleteConfig was called")
				})
			})
		})

		It("returns 500 when a BOSH task is in flight", func() {
			task := boshdirector.BoshTask{
				ID:    99,
				State: boshdirector.TaskProcessing,
			}
			tasks := boshdirector.BoshTasks{task}
			fakeBoshClient.GetTasksInProgressReturns(tasks, nil)

			response, bodyContent := doDeprovisionRequest(instanceID, dedicatedPlanID, serviceID, true, false)

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error data")
			var errorResponse apiresponses.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())
			Expect(errorResponse.Description).To(ContainSubstring(
				"An operation is in progress for your service instance. Please try again later.",
			))

			By("logging the delete request")
			Eventually(loggerBuffer).Should(
				gbytes.Say(fmt.Sprintf("deployment service-instance_%s is still in progress:", instanceID)),
			)
		})

		It("returns 500 when the BOSH director is unavailable", func() {
			fakeBoshClient.GetInfoReturns(boshdirector.Info{}, errors.New("oops"))

			response, bodyContent := doDeprovisionRequest(instanceID, dedicatedPlanID, serviceID, true, false)

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error data")
			var errorResponse apiresponses.ErrorResponse
			Expect(json.Unmarshal(bodyContent, &errorResponse)).To(Succeed())
			Expect(errorResponse.Description).To(ContainSubstring("Currently unable to delete service instance, please try again later"))
		})
	})
})

func doDeprovisionRequest(instanceID, planID, serviceID string, asyncAllowed bool, force bool) (*http.Response, []byte) {
	return doRequestWithAuthAndHeaderSet(
		http.MethodDelete,
		fmt.Sprintf("http://%s/v2/service_instances/%s?accepts_incomplete=%t&plan_id=%s&service_id=%s&force=%v", serverURL, instanceID, asyncAllowed, planID, serviceID, force),
		nil,
		func(r *http.Request) {
			r.Header.Set("X-Broker-API-Version", "2.0")
		},
	)
}
