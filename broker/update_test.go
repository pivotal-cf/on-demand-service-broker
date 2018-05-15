// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	brokerfakes "github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/task"
)

var _ = Describe("Update", func() {
	When("quotas are enabled", func() {
		updateWithQuotas := func(q quotaCase, oldPlanInstanceCount, newPlanInstanceCount int, arbitraryParams, arbitraryContext map[string]interface{}) error {
			newPlan := existingPlan
			oldPlan := secondPlan
			newPlan.Quotas = config.Quotas{}
			newPlan.ResourceCosts = map[string]int{"ips": 1, "memory": 1}
			catalogWithResourceQuotas := serviceCatalog

			planCounts := map[cf.ServicePlan]int{
				cfServicePlan("guid_1234", newPlan.ID, "url", "name"): newPlanInstanceCount,
				cfServicePlan("guid_2345", oldPlan.ID, "url", "name"): oldPlanInstanceCount,
			}
			cfClient.CountInstancesOfServiceOfferingReturns(planCounts, nil)

			// set up quotas
			if q.PlanInstanceLimit != nil {
				newPlan.Quotas.ServiceInstanceLimit = q.PlanInstanceLimit
			}
			if len(q.PlanResourceLimits) > 0 {
				newPlan.Quotas.ResourceLimits = q.PlanResourceLimits
			}
			if len(q.GlobalResourceLimits) > 0 {
				catalogWithResourceQuotas.GlobalQuotas.ResourceLimits = q.GlobalResourceLimits
			}
			if q.GlobalInstanceLimit != nil {
				catalogWithResourceQuotas.GlobalQuotas.ServiceInstanceLimit = q.GlobalInstanceLimit
			} else {
				limit := 50
				catalogWithResourceQuotas.GlobalQuotas.ServiceInstanceLimit = &limit
			}

			catalogWithResourceQuotas.Plans = config.Plans{newPlan, oldPlan}
			fakeDeployer = new(brokerfakes.FakeDeployer)
			b = createBrokerWithServiceCatalog(catalogWithResourceQuotas)

			serialisedArbitraryParameters, err := json.Marshal(arbitraryParams)
			Expect(err).NotTo(HaveOccurred())

			serialisedArbitraryContext, err := json.Marshal(arbContext)
			Expect(err).NotTo(HaveOccurred())

			updateDetails := brokerapi.UpdateDetails{
				PlanID:        newPlan.ID,
				RawParameters: serialisedArbitraryParameters,
				RawContext:    serialisedArbitraryContext,
				ServiceID:     "serviceID",
				PreviousValues: brokerapi.PreviousValues{
					PlanID:    oldPlan.ID,
					OrgID:     "organizsationGUID",
					ServiceID: "serviceID",
					SpaceID:   "spaceGUID",
				},
			}
			_, updateErr := b.Update(
				context.Background(),
				instanceID,
				updateDetails,
				true,
			)

			return updateErr
		}

		It("fails if the instance would exceed the global resource limit", func() {
			updateErr := updateWithQuotas(
				quotaCase{map[string]int{"ips": 4}, nil, nil, nil},
				1,
				4,
				map[string]interface{}{}, map[string]interface{}{},
			)
			Expect(updateErr).To(MatchError("global quotas [ips: (limit 4, used 4, requires 1)] would be exceeded by this deployment"))
		})

		It("fails if the instance would exceed the plan resource limit", func() {
			updateErr := updateWithQuotas(
				quotaCase{nil, map[string]int{"ips": 4}, nil, nil},
				1,
				4,
				map[string]interface{}{}, map[string]interface{}{},
			)
			Expect(updateErr).To(MatchError("plan quotas [ips: (limit 4, used 4, requires 1)] would be exceeded by this deployment"))
		})

		It("fails if the instance would exceed the plan instance limit", func() {
			count := 4
			updateErr := updateWithQuotas(
				quotaCase{nil, nil, nil, &count},
				1,
				4,
				map[string]interface{}{}, map[string]interface{}{},
			)
			Expect(updateErr).To(MatchError("plan instance limit exceeded for service ID: service-id. Total instances: 4"))
			Expect(boshClient.GetDeploymentCallCount()).To(BeZero())
			Expect(fakeDeployer.UpdateCallCount()).To(BeZero())
		})

		It("fails and output multiple errors when more than one quotas is exceeded", func() {
			count := 4
			updateErr := updateWithQuotas(
				quotaCase{map[string]int{"ips": 4}, map[string]int{"ips": 4}, nil, &count},
				1,
				4,
				map[string]interface{}{}, map[string]interface{}{},
			)
			Expect(updateErr.Error()).To(SatisfyAll(
				ContainSubstring("global quotas [ips: (limit 4, used 4, requires 1)] would be exceeded by this deployment"),
				ContainSubstring("plan quotas [ips: (limit 4, used 4, requires 1)] would be exceeded by this deployment"),
				ContainSubstring("plan instance limit exceeded for service ID: service-id. Total instances: 4"),
			))

			Expect(boshClient.GetDeploymentCallCount()).To(BeZero())
			Expect(fakeDeployer.UpdateCallCount()).To(BeZero())
		})
	})

	var (
		updateSpec                    brokerapi.UpdateServiceSpec
		updateDetails                 brokerapi.UpdateDetails
		arbitraryParams               map[string]interface{}
		arbContext                    map[string]interface{}
		serviceID                     string
		orgGUID                       string
		spaceGUID                     string
		boshTaskID                    int
		updateError                   error
		newPlanID                     string
		oldPlanID                     string
		serialisedArbitraryContext    []byte
		serialisedArbitraryParameters []byte
		async                         = true
		err                           error
	)

	BeforeEach(func() {
		arbitraryParams = map[string]interface{}{"foo": "bar"}
		serviceID = "serviceID"
		orgGUID = "organizationGUID"
		spaceGUID = "spaceGUID"
		boshTaskID = 447
		arbContext = map[string]interface{}{"platform": "cloudfoundry", "space_guid": "final"}
	})

	Context("when there is a previous deployment for the service instance", func() {
		var instanceID = "some-instance-id"

		BeforeEach(func() {
			newPlanID = existingPlanID
			oldPlanID = secondPlanID

			planCounts := map[cf.ServicePlan]int{}
			cfClient.CountInstancesOfServiceOfferingReturns(planCounts, nil)
			fakeDeployer.UpdateReturns(boshTaskID, []byte("new-manifest-fetched-from-adapter"), nil)
		})

		JustBeforeEach(func() {
			serialisedArbitraryParameters, err = json.Marshal(arbitraryParams)
			Expect(err).NotTo(HaveOccurred())

			serialisedArbitraryContext, err = json.Marshal(arbContext)
			Expect(err).NotTo(HaveOccurred())

			updateDetails = brokerapi.UpdateDetails{
				PlanID:        newPlanID,
				RawParameters: serialisedArbitraryParameters,
				RawContext:    serialisedArbitraryContext,
				ServiceID:     serviceID,
				PreviousValues: brokerapi.PreviousValues{
					PlanID:    oldPlanID,
					OrgID:     orgGUID,
					ServiceID: serviceID,
					SpaceID:   spaceGUID,
				},
			}

			b = createDefaultBroker()
			updateSpec, updateError = b.Update(context.Background(), instanceID, updateDetails, async)
		})

		It("invokes the deployer successfully", func() {
			Expect(fakeDeployer.UpdateCallCount()).To(Equal(1))
			_, planID, actualRequestParams, _, _, _ := fakeDeployer.UpdateArgsForCall(0)
			Expect(actualRequestParams).To(Equal(map[string]interface{}{
				"plan_id":    planID,
				"context":    arbContext,
				"parameters": arbitraryParams,
				"service_id": serviceID,
				"previous_values": map[string]interface{}{
					"space_id":        spaceGUID,
					"organization_id": orgGUID,
					"plan_id":         oldPlanID,
					"service_id":      serviceID,
				},
			}))
		})

		Context("and the request is switching plan", func() {
			Context("but the new plan's quota has not been met", func() {
				It("does not error", func() {
					Expect(updateError).NotTo(HaveOccurred())
				})

				It("calls the deployer without a bosh context id", func() {
					Expect(fakeDeployer.UpdateCallCount()).To(Equal(1))
					_, _, _, _, actualBoshContextID, _ := fakeDeployer.UpdateArgsForCall(0)
					Expect(actualBoshContextID).To(BeEmpty())
				})

				It("returns in an asynchronous fashion", func() {
					Expect(updateSpec.IsAsync).To(BeTrue())
				})

				It("returns the bosh task ID and operation type", func() {
					data := unmarshalOperationData(updateSpec)
					Expect(data).To(Equal(broker.OperationData{BoshTaskID: boshTaskID, OperationType: broker.OperationTypeUpdate}))
				})

				It("logs with a request ID", func() {
					Expect(logBuffer.String()).To(MatchRegexp(`\[[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} updating instance`))
				})
			})

			Context("but the new plan does not have a quota", func() {
				BeforeEach(func() {
					newPlanID = secondPlanID
					oldPlanID = existingPlanID
				})

				It("does not error", func() {
					Expect(updateError).NotTo(HaveOccurred())
				})

				It("returns the bosh task ID and operation type", func() {
					data := unmarshalOperationData(updateSpec)
					Expect(data).To(Equal(broker.OperationData{BoshTaskID: boshTaskID, OperationType: broker.OperationTypeUpdate}))
				})
			})

			Context("and the new plan has a post-deploy errand", func() {
				BeforeEach(func() {
					newPlanID = postDeployErrandPlanID
				})

				It("does not error", func() {
					Expect(updateError).NotTo(HaveOccurred())
				})

				It("returns that is operated asynchronously", func() {
					Expect(updateSpec.IsAsync).To(BeTrue())
				})

				It("returns the correct operation data", func() {
					data := unmarshalOperationData(updateSpec)
					Expect(data.OperationType).To(Equal(broker.OperationTypeUpdate))
					Expect(data.BoshContextID).NotTo(BeEmpty())
					Expect(data.Errands[0].Name).To(Equal("health-check"))
				})

				It("calls the deployer with a bosh context id", func() {
					Expect(fakeDeployer.UpdateCallCount()).To(Equal(1))
					_, _, _, _, actualBoshContextID, _ := fakeDeployer.UpdateArgsForCall(0)
					Expect(actualBoshContextID).NotTo(BeEmpty())
				})
			})

			Context("but there are pending changes", func() {
				BeforeEach(func() {
					fakeDeployer.UpdateReturns(boshTaskID, nil, task.PendingChangesNotAppliedError{})
				})

				It("reports a pending changes are present error", func() {
					expectedFailureResponse := brokerapi.NewFailureResponse(
						errors.New(broker.PendingChangesErrorMessage),
						http.StatusUnprocessableEntity,
						broker.UpdateLoggerAction,
					)
					Expect(updateError).To(Equal(expectedFailureResponse))
				})
			})
		})

		Context("and changing arbitrary params", func() {
			BeforeEach(func() {
				newPlanID = secondPlanID
				oldPlanID = secondPlanID
			})

			Context("and there are no pending changes", func() {
				Context("and the plan's quota has not been met", func() {
					BeforeEach(func() {
						newPlanID = existingPlanID
						oldPlanID = existingPlanID
					})

					It("does not error", func() {
						Expect(updateError).NotTo(HaveOccurred())
					})

					It("returns that is operated asynchronously", func() {
						Expect(updateSpec.IsAsync).To(BeTrue())
					})

					It("returns the bosh task ID and operation type", func() {
						data := unmarshalOperationData(updateSpec)
						Expect(data).To(Equal(broker.OperationData{BoshTaskID: boshTaskID, OperationType: broker.OperationTypeUpdate}))
					})
				})

				Context("and the plan's quota has been met", func() {
					BeforeEach(func() {
						newPlanID = existingPlanID
						oldPlanID = existingPlanID

						cfClient.CountInstancesOfPlanReturns(existingPlanServiceInstanceLimit, nil)
					})

					It("does not error", func() {
						Expect(updateError).NotTo(HaveOccurred())
					})

					It("returns that is operated asynchronously", func() {
						Expect(updateSpec.IsAsync).To(BeTrue())
					})

					It("returns the bosh task ID and operation type", func() {
						data := unmarshalOperationData(updateSpec)
						Expect(data).To(Equal(broker.OperationData{BoshTaskID: boshTaskID, OperationType: broker.OperationTypeUpdate}))
					})
				})

				Context("and the plan does not have a quota", func() {
					It("does not count the instances of the plan in Cloud Controller", func() {
						Expect(cfClient.CountInstancesOfPlanCallCount()).To(Equal(0))
					})

					It("does not error", func() {
						Expect(updateError).NotTo(HaveOccurred())
					})

					It("returns that is operated asynchronously", func() {
						Expect(updateSpec.IsAsync).To(BeTrue())
					})

					It("returns the bosh task ID and operation type", func() {
						data := unmarshalOperationData(updateSpec)
						Expect(data).To(Equal(broker.OperationData{BoshTaskID: boshTaskID, OperationType: broker.OperationTypeUpdate}))
					})
				})

				Context("and the plan has a post-deploy errand", func() {
					BeforeEach(func() {
						newPlanID = postDeployErrandPlanID
						oldPlanID = postDeployErrandPlanID
					})

					It("does not error", func() {
						Expect(updateError).NotTo(HaveOccurred())
					})

					It("returns that is operated asynchronously", func() {
						Expect(updateSpec.IsAsync).To(BeTrue())
					})

					It("returns the correct operation data", func() {
						data := unmarshalOperationData(updateSpec)
						Expect(data.OperationType).To(Equal(broker.OperationTypeUpdate))
						Expect(data.BoshContextID).NotTo(BeEmpty())
						Expect(data.Errands[0].Name).To(Equal("health-check"))
						Expect(data.Errands[0].Instances).To(Equal([]string{"redis-server/0"}))
					})

					It("calls the deployer with a bosh context id", func() {
						Expect(fakeDeployer.UpdateCallCount()).To(Equal(1))
						_, _, _, _, actualBoshContextID, _ := fakeDeployer.UpdateArgsForCall(0)
						Expect(actualBoshContextID).NotTo(BeEmpty())
					})
				})
			})
		})

		Context("when the plan cannot be found in config", func() {
			BeforeEach(func() {
				newPlanID = "non-existent-plan-id"
			})

			It("reports the error without redploying", func() {
				expectedMessage := fmt.Sprintf("Plan %s not found", newPlanID)
				Expect(updateError).To(MatchError(ContainSubstring(expectedMessage)))

				Expect(logBuffer.String()).To(ContainSubstring(expectedMessage))
				Expect(boshClient.GetDeploymentCallCount()).To(BeZero())
				Expect(fakeDeployer.UpdateCallCount()).To(BeZero())
			})
		})

		Context("when the service instances for the plan cannot be counted", func() {
			BeforeEach(func() {
				cfClient.CountInstancesOfServiceOfferingReturns(nil, fmt.Errorf("count error"))
			})

			Describe("returned error", func() {
				It("includes a standard message", func() {
					Expect(updateError).To(MatchError(ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information:",
					)))
				})

				It("includes the broker request id", func() {
					Expect(updateError).To(MatchError(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					)))
				})

				It("includes the service name", func() {
					Expect(updateError).To(MatchError(ContainSubstring(
						"service: a-cool-redis-service",
					)))
				})

				It("includes a service instance guid", func() {
					Expect(updateError).To(MatchError(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					)))
				})

				It("includes the operation type", func() {
					Expect(updateError).To(MatchError(ContainSubstring(
						"operation: update",
					)))
				})

				It("does NOT include the bosh task id", func() {
					Expect(updateError).NotTo(MatchError(ContainSubstring(
						"task-id:",
					)))
				})
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("count error"))
			})

			It("does not redeploy", func() {
				Expect(fakeDeployer.UpdateCallCount()).To(BeZero())
			})
		})

		Context("when the bosh director is unavailable", func() {
			BeforeEach(func() {
				newPlanID = existingPlanID
				oldPlanID = existingPlanID
			})

			Context("when a deploy has a bosh request error", func() {
				BeforeEach(func() {
					fakeDeployer.UpdateReturns(0, []byte{}, task.NewServiceError(fmt.Errorf("network timeout")))
				})

				It("logs the error", func() {
					Expect(logBuffer.String()).To(ContainSubstring("error: error deploying instance: network timeout"))
				})

				It("returns the try again later error for the user", func() {
					Expect(updateError).To(MatchError(ContainSubstring("Currently unable to update service instance, please try again later")))
				})
			})
		})

		Context("when the adapter client fails", func() {
			unknownFailureError := serviceadapter.NewUnknownFailureError("unknown failure")
			BeforeEach(func() {
				fakeDeployer.UpdateReturns(boshTaskID, nil, unknownFailureError)
			})

			It("returns an API error", func() {
				Expect(updateError).To(Equal(unknownFailureError))
			})
		})

		Context("when bosh is blocked", func() {
			BeforeEach(func() {
				fakeDeployer.UpdateReturns(boshTaskID, nil, task.TaskInProgressError{})
			})

			It("returns an error with the operation in progress message", func() {
				Expect(updateError).To(MatchError(ContainSubstring(broker.OperationInProgressMessage)))
			})
		})

		Context("when plan not found", func() {
			planNotFoundError := task.PlanNotFoundError{PlanGUID: "plan-guid"}
			BeforeEach(func() {
				fakeDeployer.UpdateReturns(boshTaskID, nil, planNotFoundError)
			})

			It("returns an error with the operation in progress message", func() {
				Expect(updateError).To(Equal(planNotFoundError))
			})
		})

		Context("when asked for a synchronous update", func() {
			BeforeEach(func() {
				async = false
			})

			AfterEach(func() {
				async = true
			})

			It("responds with async required error", func() {
				Expect(updateError).To(MatchError(ContainSubstring("This service plan requires client support for asynchronous service operations")))
			})
		})

		Context("when plan schemas are enabled", func() {
			var arbitraryParams map[string]interface{}
			var broker *broker.Broker
			var fakeAdapter *brokerfakes.FakeServiceAdapterClient
			var schemaParams []byte

			Context("when there is a previous deployment for the service instance", func() {
				BeforeEach(func() {
					fakeAdapter = new(brokerfakes.FakeServiceAdapterClient)
					fakeAdapter.GeneratePlanSchemaReturns(schemaFixture, nil)
					brokerConfig.EnablePlanSchemas = true
					broker = createBrokerWithAdapter(fakeAdapter)
				})

				JustBeforeEach(func() {
					updateDetails = brokerapi.UpdateDetails{
						PlanID:        newPlanID,
						RawContext:    serialisedArbitraryContext,
						RawParameters: schemaParams,
						ServiceID:     serviceOfferingID,
						PreviousValues: brokerapi.PreviousValues{
							PlanID:    oldPlanID,
							OrgID:     orgGUID,
							ServiceID: serviceID,
							SpaceID:   spaceGUID,
						},
					}
					updateSpec, updateError = broker.Update(context.Background(), instanceID, updateDetails, async)
				})

				Context("if the service adapter fails", func() {
					BeforeEach(func() {
						fakeAdapter.GeneratePlanSchemaReturns(schemaFixture, errors.New("oops"))
					})

					It("returns an error", func() {
						Expect(updateError).To(HaveOccurred())
						Expect(updateError.Error()).To(ContainSubstring("oops"))
					})

				})

				Context("if the service adapter does not implement plan schemas", func() {
					BeforeEach(func() {
						serviceAdapterError := serviceadapter.NewNotImplementedError("no.")
						fakeAdapter.GeneratePlanSchemaReturns(schemaFixture, serviceAdapterError)
					})

					It("returns an error", func() {
						Expect(updateError).To(HaveOccurred())
						Expect(updateError.Error()).To(ContainSubstring("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas"))
						Expect(logBuffer.String()).To(ContainSubstring("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas"))
					})
				})

				Context("when the provision request params are not valid", func() {
					BeforeEach(func() {
						arbitraryParams = map[string]interface{}{
							"this-is": "clearly-wrong",
						}
						schemaParams, err = json.Marshal(arbitraryParams)
						Expect(err).NotTo(HaveOccurred())
					})

					It("requests the json schemas from the service adapter", func() {
						Expect(updateError).To(HaveOccurred())
						Expect(fakeAdapter.GeneratePlanSchemaCallCount()).To(Equal(1))
						Expect(updateError.Error()).To(ContainSubstring("Additional property this-is is not allowed"))
					})

					It("fails", func() {
						Expect(updateError).To(HaveOccurred())
					})
				})

				Context("when the provision request params are empty", func() {
					BeforeEach(func() {
						schemaParams = []byte{}
					})

					It("succeeds", func() {
						Expect(updateError).NotTo(HaveOccurred())
					})
				})

				Context("when the provision request params are valid", func() {
					var err error

					BeforeEach(func() {
						arbitraryParams = map[string]interface{}{
							"update_auto_create_topics":         true,
							"update_default_replication_factor": 55,
						}
						schemaParams, err = json.Marshal(arbitraryParams)
						Expect(err).NotTo(HaveOccurred())
					})

					It("requests the json schemas from the service adapter", func() {
						Expect(fakeAdapter.GeneratePlanSchemaCallCount()).To(Equal(1))
					})

					It("succeeds", func() {
						Expect(updateError).NotTo(HaveOccurred())
					})
				})

				Context("when the schema allows additional properties", func() {
					var err error

					BeforeEach(func() {
						arbitraryParams = map[string]interface{}{
							"foo": true,
							"bar": 55,
						}
						schemaParams, err = json.Marshal(arbitraryParams)
						Expect(err).NotTo(HaveOccurred())
						fakeAdapter.GeneratePlanSchemaReturns(schemaWithAdditionalPropertiesAllowedFixture, nil)
					})

					It("succeeds", func() {
						Expect(updateError).NotTo(HaveOccurred())
					})
				})

				Context("when the schema has required properties", func() {
					var err error

					BeforeEach(func() {
						arbitraryParams = map[string]interface{}{
							"foo": true,
							"bar": 55,
						}
						schemaParams, err = json.Marshal(arbitraryParams)
						Expect(err).NotTo(HaveOccurred())
						fakeAdapter.GeneratePlanSchemaReturns(schemaWithRequiredPropertiesFixture, nil)
					})

					It("reports the required error", func() {
						Expect(updateError).To(MatchError(ContainSubstring("auto_create_topics is required")))
					})
				})
			})
		})
	})

})

func unmarshalOperationData(updateSpec brokerapi.UpdateServiceSpec) broker.OperationData {
	var data broker.OperationData
	err := json.Unmarshal([]byte(updateSpec.OperationData), &data)
	Expect(err).NotTo(HaveOccurred())
	return data
}
