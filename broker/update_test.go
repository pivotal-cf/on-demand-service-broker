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

	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/pivotal-cf/brokerapi/domain/apiresponses"
	"github.com/pivotal-cf/on-demand-service-broker/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	brokerfakes "github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

var _ = Describe("Update", func() {
	var (
		instanceID                              = "some-instance-id"
		updateSpec                              domain.UpdateServiceSpec
		arbitraryParams                         map[string]interface{}
		arbContext                              map[string]interface{}
		expectedSecretsMap                      map[string]string
		serviceID                               string
		orgGUID                                 string
		spaceGUID                               string
		boshTaskID                              int
		updateError                             error
		newPlanID                               string
		oldPlanID                               string
		serialisedArbitraryContext              []byte
		async                                   = true
		maintenanceInfo, oldPlanMaintenanceInfo domain.MaintenanceInfo
		err                                     error
		testBroker                              *broker.Broker
	)

	BeforeEach(func() {
		arbitraryParams = map[string]interface{}{"foo": "bar"}
		serviceID = "serviceID"
		orgGUID = "organizationGUID"
		spaceGUID = "spaceGUID"
		boshTaskID = 447
		arbContext = map[string]interface{}{"platform": "cloudfoundry", "space_guid": "final"}

		newPlanID = secondPlanID
		oldPlanID = existingPlanID

		planCounts := map[cf.ServicePlan]int{}
		cfClient.CountInstancesOfServiceOfferingReturns(planCounts, nil)
		fakeDeployer.UpdateReturns(boshTaskID, []byte("new-manifest-fetched-from-adapter"), nil)

		expectedSecretsMap = map[string]string{
			"foo": "b4r",
		}
		fakeSecretManager.ResolveManifestSecretsReturns(expectedSecretsMap, nil)
		maintenanceInfo = domain.MaintenanceInfo{}
		testBroker = createDefaultBroker()
		catalog, err := testBroker.Services(context.Background())
		Expect(err).NotTo(HaveOccurred())
		oldPlanMaintenanceInfo = *catalog[0].Plans[0].MaintenanceInfo

	})

	When("it is an update", func() {
		var updateDetails domain.UpdateDetails
		JustBeforeEach(func() {
			serialisedArbitraryContext, err := json.Marshal(arbContext)
			Expect(err).NotTo(HaveOccurred())

			serialisedArbitraryParameters, err := json.Marshal(arbitraryParams)
			Expect(err).NotTo(HaveOccurred())

			updateDetails = domain.UpdateDetails{
				PlanID:     newPlanID,
				ServiceID:  serviceID,
				RawContext: serialisedArbitraryContext,
				PreviousValues: domain.PreviousValues{
					PlanID: oldPlanID,
				},
				MaintenanceInfo: maintenanceInfo,
				RawParameters:   serialisedArbitraryParameters,
			}
			updateSpec, updateError = b.Update(context.Background(), instanceID, updateDetails, async)
		})

		It("invokes the deployer with the correct arguments", func() {
			Expect(fakeDeployer.UpdateCallCount()).To(Equal(1))
			_, planID, actualRequestParams, _, _, actualSecretsMap, _ := fakeDeployer.UpdateArgsForCall(0)

			Expect(actualRequestParams).To(Equal(map[string]interface{}{
				"plan_id":    planID,
				"context":    arbContext,
				"parameters": arbitraryParams,
				"service_id": serviceID,
				"previous_values": map[string]interface{}{
					"space_id":        "",
					"organization_id": "",
					"plan_id":         oldPlanID,
					"service_id":      "",
				},
				"maintenance_info": map[string]interface{}{},
			}))

			Expect(actualSecretsMap).To(Equal(expectedSecretsMap))
		})

		Context("the request is switching plan", func() {
			Context("and the new plan's quota has not been met", func() {
				It("does not error", func() {
					Expect(updateError).NotTo(HaveOccurred())
				})

				It("calls the deployer without a bosh context id", func() {
					Expect(fakeDeployer.UpdateCallCount()).To(Equal(1))
					_, _, _, _, actualBoshContextID, _, _ := fakeDeployer.UpdateArgsForCall(0)
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

			Context("and the new plan does not have a quota", func() {
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
					_, _, _, _, actualBoshContextID, _, _ := fakeDeployer.UpdateArgsForCall(0)
					Expect(actualBoshContextID).NotTo(BeEmpty())
				})
			})

			Context("and the incoming maintenance_info matches the broker maintenance_info", func() {
				BeforeEach(func() {
					arbitraryParams = nil
					catalog, err := b.Services(context.Background())
					Expect(err).NotTo(HaveOccurred())
					maintenanceInfo = *catalog[0].Plans[1].MaintenanceInfo
					Expect(secondPlan.ID).To(Equal(catalog[0].Plans[1].ID))
					newPlanID = secondPlan.ID
				})

				It("does not error", func() {
					Expect(updateError).NotTo(HaveOccurred())
				})
			})

			Context("but there are pending changes", func() {
				BeforeEach(func() {
					fakeDeployer.UpdateReturns(boshTaskID, nil, broker.PendingChangesNotAppliedError{})
				})

				It("reports a pending changes are present error", func() {
					expectedFailureResponse := apiresponses.NewFailureResponse(
						errors.New(broker.PendingChangesErrorMessage),
						http.StatusUnprocessableEntity,
						broker.UpdateLoggerAction,
					)
					Expect(updateError).To(Equal(expectedFailureResponse))
				})
			})
		})

		Context("changing arbitrary params", func() {
			BeforeEach(func() {
				newPlanID = secondPlanID
				oldPlanID = secondPlanID
				arbitraryParams = map[string]interface{}{"new": "value"}
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
						_, _, _, _, actualBoshContextID, _, _ := fakeDeployer.UpdateArgsForCall(0)
						Expect(actualBoshContextID).NotTo(BeEmpty())
					})
				})
			})
		})

		When("the service instances for the plan cannot be counted", func() {
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

		When("plan schemas are enabled", func() {
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
					updateDetails = domain.UpdateDetails{
						PlanID:        newPlanID,
						RawContext:    serialisedArbitraryContext,
						RawParameters: schemaParams,
						ServiceID:     serviceOfferingID,
						PreviousValues: domain.PreviousValues{
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

				Context("when the provision request params has extra params", func() {
					BeforeEach(func() {
						arbitraryParams = map[string]interface{}{
							"this-is": "clearly-wrong",
						}
						schemaParams, err = json.Marshal(arbitraryParams)
						Expect(err).NotTo(HaveOccurred())
					})

					It("requests the json schemas from the service adapter", func() {
						Expect(updateError).To(HaveOccurred())
						Expect(updateError.Error()).To(ContainSubstring("this-is: Additional property this-is is not allowed"))

						actualErr := updateError.(*apiresponses.FailureResponse)
						Expect(actualErr.ValidatedStatusCode(nil)).To(Equal(http.StatusBadRequest))

						response := actualErr.ErrorResponse()
						Expect(response).To(BeAssignableToTypeOf(apiresponses.ErrorResponse{}))
						errorResponse := response.(apiresponses.ErrorResponse)
						Expect(errorResponse.Description).To(ContainSubstring("this-is: Additional property this-is is not allowed"))
					})
				})

				Context("when the provision has invalid parameters", func() {
					BeforeEach(func() {
						arbitraryParams = map[string]interface{}{
							"auto_create_topic": "maybe",
						}
						schemaParams, err = json.Marshal(arbitraryParams)
						Expect(err).NotTo(HaveOccurred())
					})

					It("requests the json schemas from the service adapter", func() {
						Expect(updateError).To(HaveOccurred())
						Expect(updateError.Error()).To(ContainSubstring("auto_create_topic: Additional property auto_create_topic is not allowed"))

						actualErr := updateError.(*apiresponses.FailureResponse)
						Expect(actualErr.ValidatedStatusCode(nil)).To(Equal(http.StatusBadRequest))

						response := actualErr.ErrorResponse()
						Expect(response).To(BeAssignableToTypeOf(apiresponses.ErrorResponse{}))
						errorResponse := response.(apiresponses.ErrorResponse)
						Expect(errorResponse.Description).To(ContainSubstring("auto_create_topic: Additional property auto_create_topic is not allowed"))
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

		When("the manifest cannot be retrieved", func() {
			It("returns an error", func() {
				boshClient.GetDeploymentReturns(nil, false, errors.New("no deployment"))

				updateSpec, updateError = testBroker.Update(context.Background(), instanceID, updateDetails, async)

				Expect(updateError).To(MatchError(ContainSubstring("There was a problem completing your request")))
				Expect(logBuffer.String()).To(ContainSubstring("no deployment"))
			})
		})

		When("BOSH variables cannot be retrieved", func() {
			It("returns an error", func() {
				boshClient.VariablesReturns(nil, errors.New("no variables"))

				updateSpec, updateError = testBroker.Update(context.Background(), instanceID, updateDetails, async)

				Expect(updateError).To(MatchError(ContainSubstring("There was a problem completing your request")))
				Expect(logBuffer.String()).To(ContainSubstring("no variables"))
			})
		})

		When("secrets cannot be resolved", func() {
			It("returns an error", func() {
				fakeSecretManager.ResolveManifestSecretsReturns(nil, errors.New("no secrets"))

				updateSpec, updateError = testBroker.Update(context.Background(), instanceID, updateDetails, async)

				Expect(updateError).To(MatchError(ContainSubstring("There was a problem completing your request")))
				Expect(logBuffer.String()).To(ContainSubstring("no secrets"))
			})
		})

		When("quotas are enabled", func() {
			updateWithQuotas := func(q quotaCase, oldPlanInstanceCount, newPlanInstanceCount int, arbitraryParams, arbitraryContext map[string]interface{}) error {
				newPlan := existingPlan
				oldPlan := secondPlan
				newPlan.Quotas = config.Quotas{}
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
				if len(q.PlanResourceQuota) > 0 {
					newPlan.Quotas.Resources = q.PlanResourceQuota
				}
				if len(q.GlobalResourceQuota) > 0 {
					catalogWithResourceQuotas.GlobalQuotas.Resources = q.GlobalResourceQuota
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

				updateDetails := domain.UpdateDetails{
					PlanID:        newPlan.ID,
					RawParameters: serialisedArbitraryParameters,
					RawContext:    serialisedArbitraryContext,
					ServiceID:     "serviceID",
					PreviousValues: domain.PreviousValues{
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
					quotaCase{map[string]config.ResourceQuota{"ips": {Limit: 4}}, map[string]config.ResourceQuota{"ips": {Cost: 1}}, nil, nil},
					1,
					4,
					map[string]interface{}{}, map[string]interface{}{},
				)
				Expect(updateErr).To(MatchError("global quotas [ips: (limit 4, used 4, requires 1)] would be exceeded by this deployment"))
			})

			It("fails if the instance would exceed the plan resource limit", func() {
				updateErr := updateWithQuotas(
					quotaCase{nil, map[string]config.ResourceQuota{"ips": {Limit: 4, Cost: 1}}, nil, nil},
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
			})

			It("fails and output multiple errors when more than one quotas is exceeded", func() {
				count := 4
				updateErr := updateWithQuotas(
					quotaCase{map[string]config.ResourceQuota{"ips": {Limit: 4}}, map[string]config.ResourceQuota{"ips": {Limit: 4, Cost: 1}}, nil, &count},
					1,
					4,
					map[string]interface{}{}, map[string]interface{}{},
				)
				Expect(updateErr.Error()).To(SatisfyAll(
					ContainSubstring("global quotas [ips: (limit 4, used 4, requires 1)] would be exceeded by this deployment"),
					ContainSubstring("plan quotas [ips: (limit 4, used 4, requires 1)] would be exceeded by this deployment"),
					ContainSubstring("plan instance limit exceeded for service ID: service-id. Total instances: 4"),
				))
			})
		})

	})

	When("it is an upgrade", func() {
		var testCases []domain.UpdateDetails

		BeforeEach(func() {
			fakeDeployer.UpgradeReturns(50, nil, nil)
			fakeDeployer.UpdateReturns(-1, nil, errors.New("fail"))

			testCases = []domain.UpdateDetails{
				{
					PlanID:     oldPlanID,
					ServiceID:  serviceID,
					RawContext: serialisedArbitraryContext,
					PreviousValues: domain.PreviousValues{
						PlanID: oldPlanID,
					},
					MaintenanceInfo: domain.MaintenanceInfo{
						Private: "secret:secret;",
					},
				},
				{
					PlanID:     oldPlanID,
					ServiceID:  serviceID,
					RawContext: serialisedArbitraryContext,
					PreviousValues: domain.PreviousValues{
						PlanID: oldPlanID,
					},
					MaintenanceInfo: domain.MaintenanceInfo{
						Public: map[string]string{
							"something": "fancy",
						},
					},
				},
				{
					PlanID:     oldPlanID,
					ServiceID:  serviceID,
					RawContext: serialisedArbitraryContext,
					PreviousValues: domain.PreviousValues{
						PlanID: oldPlanID,
					},
					MaintenanceInfo: domain.MaintenanceInfo{
						Version: "1.2.3",
					},
				},
			}
		})

		It("accepts the upgrade when maintenance_info is set", func() {
			for i, updateDetails := range testCases {
				fakeMaintenanceInfoChecker.CheckReturns(nil)

				updateSpec, updateError = testBroker.Update(context.Background(), instanceID, updateDetails, async)

				Expect(fakeDeployer.UpdateCallCount()).To(Equal(0), fmt.Sprintf("Update was called - test case %d", i))
				Expect(updateError).NotTo(HaveOccurred())

				opData, err := json.Marshal(broker.OperationData{
					BoshTaskID:    50,
					OperationType: broker.OperationTypeUpgrade,
					Errands:       nil,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(updateSpec).To(Equal(domain.UpdateServiceSpec{
					IsAsync:       true,
					OperationData: string(opData),
				}))

				Expect(logBuffer.String()).To(MatchRegexp(`\[[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} upgrading instance`))
			}
		})
	})

	Describe("regardless of the type of update", func() {
		var testCases []domain.UpdateDetails

		BeforeEach(func() {
			testCases = []domain.UpdateDetails{
				{
					PlanID:    newPlanID,
					ServiceID: serviceID,
					PreviousValues: domain.PreviousValues{
						PlanID: oldPlanID,
					},
				},
				{
					PlanID:    oldPlanID,
					ServiceID: serviceID,
					PreviousValues: domain.PreviousValues{
						PlanID: oldPlanID,
					},
					RawParameters: []byte(`{"foo":"bar"}`),
				},
				{
					PlanID:    oldPlanID,
					ServiceID: serviceID,
					PreviousValues: domain.PreviousValues{
						PlanID: oldPlanID,
					},
					MaintenanceInfo: oldPlanMaintenanceInfo,
				},
			}
		})

		It("fails when plan is not found", func() {
			for i, t := range testCases {
				invalidPlanId := "invalid-plan-guid"
				planNotFoundError := broker.PlanNotFoundError{PlanGUID: invalidPlanId}

				t.PlanID = invalidPlanId

				updateSpec, updateError = testBroker.Update(context.Background(), instanceID, t, async)

				Expect(updateError).To(Equal(planNotFoundError), fmt.Sprintf("test case %d", i))

				Expect(boshClient.GetDeploymentCallCount()).To(BeZero())
				Expect(fakeDeployer.UpdateCallCount()).To(BeZero())
			}
		})

		It("responds with async required error when asked for a synchronous update", func() {
			for i, t := range testCases {
				updateSpec, updateError = testBroker.Update(context.Background(), instanceID, t, false)

				Expect(updateError).To(MatchError(
					ContainSubstring("This service plan requires client support for asynchronous service operations")),
					fmt.Sprintf("test case %d", i))
			}
		})

		It("returns a 'try again' message when deployer reports service errors", func() {
			for i, t := range testCases {
				fakeDeployer.UpdateReturns(0, []byte{}, broker.NewServiceError(fmt.Errorf("network timeout")))
				fakeDeployer.UpgradeReturns(0, []byte{}, broker.NewServiceError(fmt.Errorf("network timeout")))

				updateSpec, updateError = testBroker.Update(context.Background(), instanceID, t, async)

				Expect(logBuffer.String()).To(ContainSubstring("error: error deploying instance: network timeout"), fmt.Sprintf("test case %d", i))
				Expect(updateError).To(MatchError(ContainSubstring("Currently unable to update service instance, please try again later")), fmt.Sprintf("test case %d", i))
			}
		})

		It("returns an operation in progress error when deployer reports task in progress", func() {
			for i, t := range testCases {
				fakeDeployer.UpdateReturns(boshTaskID, nil, broker.TaskInProgressError{})
				fakeDeployer.UpgradeReturns(boshTaskID, nil, broker.TaskInProgressError{})

				updateSpec, updateError = testBroker.Update(context.Background(), instanceID, t, async)

				Expect(updateError).To(MatchError(ContainSubstring(broker.OperationInProgressMessage)), fmt.Sprintf("test case %d", i))
			}
		})

		It("returns an API error when the adapter client fails with UnknownFailureError", func() {
			for i, t := range testCases {
				unknownFailureError := serviceadapter.NewUnknownFailureError("unknown failure")
				fakeDeployer.UpdateReturns(boshTaskID, nil, unknownFailureError)
				fakeDeployer.UpgradeReturns(boshTaskID, nil, unknownFailureError)

				updateSpec, updateError = testBroker.Update(context.Background(), instanceID, t, async)

				Expect(updateError).To(Equal(unknownFailureError), fmt.Sprintf("test case %d", i))
			}
		})

		It("fails when the requested maintenance info check fails", func() {

			for i, t := range testCases {
				fakeMaintenanceInfoChecker.CheckReturns(fmt.Errorf("nope"))

				updateSpec, updateError = testBroker.Update(context.Background(), instanceID, t, async)

				Expect(fakeMaintenanceInfoChecker.CheckCallCount()).To(Equal(i+1), fmt.Sprintf("Check was not called - test case %d", i))
				Expect(updateError).To(
					MatchError("nope"),
					fmt.Sprintf("test case %d", i),
				)
			}
		})
	})
})

func unmarshalOperationData(updateSpec domain.UpdateServiceSpec) broker.OperationData {
	var data broker.OperationData
	err := json.Unmarshal([]byte(updateSpec.OperationData), &data)
	Expect(err).NotTo(HaveOccurred())
	return data
}
