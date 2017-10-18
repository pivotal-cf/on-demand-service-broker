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
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/task"
)

var _ = Describe("Update", func() {
	var (
		updateSpec      brokerapi.UpdateServiceSpec
		updateDetails   brokerapi.UpdateDetails
		arbitraryParams map[string]interface{}
		serviceID       string
		orgGUID         string
		spaceGUID       string
		boshTaskID      int
	)

	BeforeEach(func() {
		arbitraryParams = map[string]interface{}{"foo": "bar"}
		serviceID = "serviceID"
		orgGUID = "organizationGUID"
		spaceGUID = "spaceGUID"
		boshTaskID = 447
	})

	Context("when there is a previous deployment for the service instance", func() {
		var (
			updateError error
			newPlanID   string
			oldPlanID   string
			instanceID  = "some-instance-id"
			async       = true
		)

		BeforeEach(func() {
			newPlanID = existingPlanID
			oldPlanID = secondPlanID

			cfClient.CountInstancesOfPlanReturns(0, nil)
			fakeDeployer.UpdateReturns(boshTaskID, []byte("new-manifest-fetched-from-adapter"), nil)
		})

		JustBeforeEach(func() {
			serialisedArbitraryParameters, err := json.Marshal(arbitraryParams)
			Expect(err).NotTo(HaveOccurred())

			updateDetails = brokerapi.UpdateDetails{
				PlanID:        newPlanID,
				RawParameters: serialisedArbitraryParameters,
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

		Context("and the request is switching plan", func() {
			Context("but the new plan's quota has not been met", func() {
				It("does not error", func() {
					Expect(updateError).NotTo(HaveOccurred())
				})

				It("counts the instances of the plan in Cloud Controller", func() {
					Expect(cfClient.CountInstancesOfPlanCallCount()).To(Equal(1))
					actualServiceOfferingID, actualPlanID, _ := cfClient.CountInstancesOfPlanArgsForCall(0)
					Expect(actualServiceOfferingID).To(Equal(serviceID))
					Expect(actualPlanID).To(Equal(newPlanID))
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

			Context("but the new plan's quota has been reached", func() {
				BeforeEach(func() {
					cfClient.CountInstancesOfPlanReturns(existingPlanServiceInstanceLimit, nil)
				})

				It("returns an error", func() {
					Expect(updateError).To(HaveOccurred())
				})

				It("does not redeploy", func() {
					Expect(boshClient.GetDeploymentCallCount()).To(BeZero())
					Expect(fakeDeployer.UpdateCallCount()).To(BeZero())
				})
			})

			Context("but the new plan does not have a quota", func() {
				BeforeEach(func() {
					newPlanID = secondPlanID
					oldPlanID = existingPlanID
				})

				It("does not count the instances of the plan in Cloud Controller", func() {
					Expect(cfClient.CountInstancesOfPlanCallCount()).To(Equal(0))
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
					Expect(data.PostDeployErrand.Name).To(Equal("health-check"))
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
						Expect(data.PostDeployErrand.Name).To(Equal("health-check"))
						Expect(data.PostDeployErrand.Instances).To(Equal([]string{"redis-server/0"}))
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
				cfClient.CountInstancesOfPlanReturns(0, fmt.Errorf("count error"))
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
				Expect(logBuffer.String()).To(ContainSubstring("error: error counting instances of plan: count error"))
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
	})
})

func unmarshalOperationData(updateSpec brokerapi.UpdateServiceSpec) broker.OperationData {
	var data broker.OperationData
	err := json.Unmarshal([]byte(updateSpec.OperationData), &data)
	Expect(err).NotTo(HaveOccurred())
	return data
}
