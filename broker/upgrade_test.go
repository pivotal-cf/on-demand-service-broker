// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	brokerfakes "github.com/pivotal-cf/on-demand-service-broker/broker/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

var _ = Describe("Upgrade", func() {
	var (
		upgradeOperationData broker.OperationData
		details              domain.UpdateDetails
		instanceID           string
		logger               *log.Logger
		boshTaskID           int
		redeployErr          error
	)

	BeforeEach(func() {
		instanceID = "some-instance"
		boshTaskID = 876
		details = domain.UpdateDetails{
			PlanID: existingPlanID,
		}
		logger = loggerFactory.NewWithRequestID()
		b = createDefaultBroker()
		fakeDeployer.UpgradeReturns(boshTaskID, []byte("new-manifest-fetched-from-adapter"), nil)
	})

	It("when the deployment goes well deploys with the new planID", func() {
		upgradeOperationData, redeployErr = b.Upgrade(context.Background(), instanceID, details, logger)

		Expect(redeployErr).NotTo(HaveOccurred())
		Expect(fakeDeployer.CreateCallCount()).To(Equal(0))
		Expect(fakeDeployer.UpgradeCallCount()).To(Equal(1))
		Expect(fakeDeployer.UpdateCallCount()).To(Equal(0))
		actualDeploymentName, actualPlanID, actualPreviousPlanID, actualBoshContextID, _ := fakeDeployer.UpgradeArgsForCall(0)
		Expect(actualPlanID).To(Equal(existingPlanID))
		Expect(actualDeploymentName).To(Equal(broker.InstancePrefix + instanceID))
		oldPlanIDCopy := existingPlanID
		Expect(actualPreviousPlanID).To(Equal(&oldPlanIDCopy))
		Expect(actualBoshContextID).To(BeEmpty())
	})

	Context("when instance is already up to date", func() {
		It("should return error", func() {
			expectedError := broker.NewOperationAlreadyCompletedError(errors.New("instance is already up to date"))
			fakeDeployer.UpgradeReturns(0, nil, expectedError)

			_, redeployErr = b.Upgrade(context.Background(), instanceID, details, logger)

			Expect(redeployErr).To(Equal(expectedError))
		})
	})

	Context("when there is a previous deployment for the service instance", func() {
		It("responds with the correct upgradeOperationData", func() {
			upgradeOperationData, _ = b.Upgrade(context.Background(), instanceID, details, logger)

			Expect(upgradeOperationData).To(Equal(
				broker.OperationData{
					BoshTaskID:    boshTaskID,
					OperationType: broker.OperationTypeUpgrade,
				},
			))

			Expect(logBuffer.String()).To(MatchRegexp(`\[[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} upgrading instance`))
		})

		It("and post-deploy errand is configured deploys with a context id", func() {
			details = domain.UpdateDetails{
				PlanID: postDeployErrandPlanID,
			}

			upgradeOperationData, _ = b.Upgrade(context.Background(), instanceID, details, logger)

			_, _, _, contextID, _ := fakeDeployer.UpgradeArgsForCall(0)
			Expect(contextID).NotTo(BeEmpty())
			Expect(upgradeOperationData.BoshContextID).NotTo(BeEmpty())
			Expect(upgradeOperationData).To(Equal(
				broker.OperationData{
					BoshTaskID:    boshTaskID,
					OperationType: broker.OperationTypeUpgrade,
					BoshContextID: upgradeOperationData.BoshContextID,
					Errands: []config.Errand{{
						Name:      "health-check",
						Instances: []string{"redis-server/0"},
					}},
				},
			))
		})

		It("and the service adapter returns a UnknownFailureError with a user message returns the error for the user", func() {
			err := serviceadapter.NewUnknownFailureError("error for cf user")
			fakeDeployer.UpgradeReturns(boshTaskID, nil, err)
			_, redeployErr = b.Upgrade(context.Background(), instanceID, details, logger)

			Expect(redeployErr).To(Equal(err))
		})

		It("and the service adapter returns a UnknownFailureError with no message returns a generic error", func() {
			fakeDeployer.UpgradeReturns(boshTaskID, nil, serviceadapter.NewUnknownFailureError(""))
			_, redeployErr = b.Upgrade(context.Background(), instanceID, details, logger)

			Expect(redeployErr).To(MatchError(ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information")))
		})
	})

	It("when no update details are provided returns an error", func() {
		details = domain.UpdateDetails{}
		_, redeployErr = b.Upgrade(context.Background(), instanceID, details, logger)

		Expect(redeployErr).To(MatchError(ContainSubstring("no plan ID provided in upgrade request body")))
	})

	It("when the plan cannot be found upgrade fails and does not redeploy", func() {
		planID := "plan-id-doesnt-exist"

		details = domain.UpdateDetails{
			PlanID: planID,
		}
		_, redeployErr = b.Upgrade(context.Background(), instanceID, details, logger)

		Expect(redeployErr).To(MatchError(ContainSubstring(fmt.Sprintf("plan %s not found", planID))))
		Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("error: finding plan ID %s", planID)))
		Expect(fakeDeployer.UpgradeCallCount()).To(BeZero())
	})

	It("when there is a task in progress on the instance upgrade returns an OperationInProgressError", func() {
		fakeDeployer.UpgradeReturns(0, nil, broker.TaskInProgressError{})
		_, redeployErr = b.Upgrade(context.Background(), instanceID, details, logger)

		Expect(redeployErr).To(BeAssignableToTypeOf(broker.OperationInProgressError{}))
	})

	It("should not request the json schemas from the service adapter", func() {
		fakeAdapter := new(brokerfakes.FakeServiceAdapterClient)
		fakeAdapter.GeneratePlanSchemaReturns(domain.ServiceSchemas{}, fmt.Errorf("derp!"))
		broker := createBrokerWithAdapter(fakeAdapter)

		_, upgradeErr := broker.Upgrade(context.Background(), instanceID, details, logger)

		Expect(fakeAdapter.GeneratePlanSchemaCallCount()).To(Equal(0))
		Expect(upgradeErr).NotTo(HaveOccurred())
	})

})
