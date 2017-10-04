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
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/task"
)

var _ = Describe("Upgrade", func() {
	var (
		upgradeOperationData     broker.OperationData
		instanceID               string
		serviceDeploymentName    string
		logger                   *log.Logger
		expectedPreviousManifest []byte
		boshTaskID               int
		redeployErr              error
	)

	BeforeEach(func() {
		instanceID = "some-instance"
		serviceDeploymentName = deploymentName(instanceID)
		expectedPreviousManifest = []byte("old-manifest-fetched-from-bosh")
		boshTaskID = 876
	})

	JustBeforeEach(func() {
		logger = loggerFactory.NewWithRequestID()
		b = createDefaultBroker()
		upgradeOperationData, redeployErr = b.Upgrade(context.Background(), instanceID, logger)
	})

	Context("when the deployment goes well", func() {
		BeforeEach(func() {
			fakeDeployer.UpgradeReturns(boshTaskID, []byte("new-manifest-fetched-from-adapter"), nil)
			cfClient.GetInstanceStateReturns(cf.InstanceState{PlanID: existingPlanID}, nil)
		})

		It("does not error", func() {
			Expect(redeployErr).NotTo(HaveOccurred())
		})

		It("deploys with the new planID", func() {
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
	})

	Context("when there is a previous deployment for the service instance", func() {
		BeforeEach(func() {
			fakeDeployer.UpgradeReturns(boshTaskID, []byte("new-manifest-fetched-from-adapter"), nil)
			cfClient.GetInstanceStateReturns(cf.InstanceState{PlanID: existingPlanID}, nil)
		})

		It("returns the task ID for the upgrade task", func() {
			Expect(upgradeOperationData.BoshTaskID).To(Equal(boshTaskID))
		})

		It("returns an empty context id", func() {
			Expect(upgradeOperationData.BoshContextID).To(BeEmpty())
		})

		It("returns the plan id", func() {
			Expect(upgradeOperationData.PlanID).To(BeEmpty())
		})

		It("returns the operation type", func() {
			Expect(upgradeOperationData.OperationType).To(Equal(broker.OperationTypeUpgrade))
		})

		It("fetches correct instance", func() {
			Expect(cfClient.GetInstanceStateCallCount()).To(Equal(1))
			actualInstanceID, _ := cfClient.GetInstanceStateArgsForCall(0)
			Expect(actualInstanceID).To(Equal(instanceID))
		})

		It("logs with a request ID", func() {
			Expect(logBuffer.String()).To(MatchRegexp(`\[[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} upgrading instance`))
		})

		Context("and there is an operation in progress on the instance", func() {
			BeforeEach(func() {
				cfClient.GetInstanceStateReturns(cf.InstanceState{PlanID: existingPlanID, OperationInProgress: true}, nil)
			})

			It("returns an OperationInProgressError", func() {
				Expect(redeployErr).To(BeAssignableToTypeOf(broker.OperationInProgressError{}))
			})
		})

		Context("and post-deploy errand is configured", func() {
			BeforeEach(func() {
				cfClient.GetInstanceStateReturns(cf.InstanceState{PlanID: postDeployErrandPlanID}, nil)
			})

			It("deploys with a context id", func() {
				_, _, _, contextID, _ := fakeDeployer.UpgradeArgsForCall(0)
				Expect(contextID).NotTo(BeEmpty())
				Expect(upgradeOperationData.BoshContextID).NotTo(BeEmpty())
				Expect(upgradeOperationData).To(Equal(
					broker.OperationData{
						BoshTaskID:           boshTaskID,
						PostDeployErrandName: "health-check",
						OperationType:        broker.OperationTypeUpgrade,
						BoshContextID:        upgradeOperationData.BoshContextID,
					},
				))
			})
		})

		Context("and the service adapter returns a UnknownFailureError with a user message", func() {
			var err = serviceadapter.NewUnknownFailureError("error for cf user")

			BeforeEach(func() {
				fakeDeployer.UpgradeReturns(boshTaskID, nil, err)
			})

			It("returns the error for the user", func() {
				Expect(redeployErr).To(Equal(err))
			})
		})

		Context("and the service adapter returns a UnknownFailureError with no message", func() {
			BeforeEach(func() {
				fakeDeployer.UpgradeReturns(boshTaskID, nil, serviceadapter.NewUnknownFailureError(""))
			})

			It("returns a generic error", func() {
				Expect(redeployErr).To(MatchError(ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information")))
			})
		})
	})

	Context("when the service instance cannot be found in CF", func() {
		BeforeEach(func() {
			cfClient.GetInstanceStateReturns(cf.InstanceState{}, cf.ResourceNotFoundError{})
		})

		It("returns an error", func() {
			Expect(redeployErr).To(HaveOccurred())
		})
	})

	Context("when the instance state cannot be retrieved from CF", func() {
		BeforeEach(func() {
			cfClient.GetInstanceStateReturns(cf.InstanceState{}, errors.New("get instance state error"))
		})

		It("returns the error to the cf user", func() {
			Expect(redeployErr).To(MatchError(ContainSubstring("get instance state error")))
		})
	})

	Context("when the plan cannot be found", func() {
		var planID string

		BeforeEach(func() {
			planID = "non-existent-plan-id"
			cfClient.GetInstanceStateReturns(cf.InstanceState{PlanID: planID}, nil)
		})

		It("fails and does not redeploy", func() {
			Expect(redeployErr).To(MatchError(ContainSubstring(fmt.Sprintf("plan %s not found", planID))))
			Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("error: finding plan ID %s", planID)))
			Expect(fakeDeployer.UpgradeCallCount()).To(BeZero())
		})
	})

	Context("when there is a task in progress on the instance", func() {
		BeforeEach(func() {
			cfClient.GetInstanceStateReturns(cf.InstanceState{PlanID: existingPlanID}, nil)
			fakeDeployer.UpgradeReturns(0, nil, task.TaskInProgressError{})
		})

		It("returns an OperationInProgressError", func() {
			Expect(redeployErr).To(BeAssignableToTypeOf(broker.OperationInProgressError{}))
		})
	})
})
