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

	"code.cloudfoundry.org/brokerapi/v13/domain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

var _ = Describe("Recreate", func() {
	var (
		instanceID    = "an-instance"
		logger        *log.Logger
		boshTaskID    = 63967
		details       domain.UpdateDetails
		operationData broker.OperationData
	)

	BeforeEach(func() {
		logger = loggerFactory.NewWithRequestID()
		details = domain.UpdateDetails{
			PlanID: existingPlanID,
		}

		b = createDefaultBroker()
		fakeDeployer.RecreateReturns(boshTaskID, nil)
	})

	It("asks the deployer to perform a recreate", func() {
		operationData, err := b.Recreate(context.Background(), instanceID, details, logger)

		Expect(err).NotTo(HaveOccurred())
		Expect(fakeDeployer.RecreateCallCount()).To(Equal(1), "expected the deployer to be called once")

		actualDeploymentName, _, actualBoshContextID, _ := fakeDeployer.RecreateArgsForCall(0)

		Expect(actualDeploymentName).To(Equal(broker.InstancePrefix + instanceID))
		Expect(actualBoshContextID).To(BeEmpty())
		Expect(operationData).To(Equal(
			broker.OperationData{
				BoshTaskID:    boshTaskID,
				OperationType: broker.OperationTypeRecreate,
				Errands:       nil,
			},
		))
	})

	It("when a post-deploy errand is configured, it recreates with a context id", func() {
		details = domain.UpdateDetails{
			PlanID: postDeployErrandPlanID,
		}

		operationData, _ = b.Recreate(context.Background(), instanceID, details, logger)

		_, _, contextID, _ := fakeDeployer.RecreateArgsForCall(0)
		Expect(contextID).NotTo(BeEmpty())
		Expect(operationData.BoshContextID).NotTo(BeEmpty())
		Expect(contextID).To(Equal(operationData.BoshContextID))

		Expect(operationData).To(Equal(
			broker.OperationData{
				BoshTaskID:    boshTaskID,
				OperationType: broker.OperationTypeRecreate,
				BoshContextID: operationData.BoshContextID,
				Errands: []config.Errand{{
					Name:      "health-check",
					Instances: []string{"redis-server/0"},
				}},
			},
		))

		Expect(logBuffer.String()).To(MatchRegexp(`\[[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} recreating instance`))
	})

	It("when no update details are provided returns an error", func() {
		details = domain.UpdateDetails{}
		_, err := b.Recreate(context.Background(), instanceID, details, logger)

		Expect(err).To(MatchError(ContainSubstring("no plan ID provided in recreate request body")))
	})

	It("when the plan cannot be found, recreate fails and does not redeploy", func() {
		planID := "plan-id-doesnt-exist"

		details = domain.UpdateDetails{
			PlanID: planID,
		}
		_, err := b.Recreate(context.Background(), instanceID, details, logger)

		Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("plan %s not found", planID))))
		Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("error: finding plan ID %s", planID)))
		Expect(fakeDeployer.RecreateCallCount()).To(BeZero())
	})

	It("when there is a task in progress on the instance, recreate returns an OperationInProgressError", func() {
		fakeDeployer.RecreateReturns(0, broker.TaskInProgressError{})
		_, err := b.Recreate(context.Background(), instanceID, details, logger)

		Expect(err).To(BeAssignableToTypeOf(broker.OperationInProgressError{}))
	})

	It("when the service adapter returns a UnknownFailureError with a user message, it returns the error for the user", func() {
		expectedErr := serviceadapter.NewUnknownFailureError("error for cf user")
		fakeDeployer.RecreateReturns(0, expectedErr)
		_, err := b.Recreate(context.Background(), instanceID, details, logger)

		Expect(err).To(Equal(expectedErr))
	})

	It("returns a generic error", func() {
		expectedErr := errors.New("oops")
		fakeDeployer.RecreateReturns(0, expectedErr)
		_, err := b.Recreate(context.Background(), instanceID, details, logger)

		Expect(err).To(Equal(expectedErr))
	})
})
