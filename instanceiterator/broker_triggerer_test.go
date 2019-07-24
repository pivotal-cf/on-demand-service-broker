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

package instanceiterator_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator"
	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

var _ = Describe("Broker Operation Triggerer", func() {
	var (
		guid              string
		instance          service.Instance
		fakeBrokerService *fakes.FakeBrokerServices
		subject           instanceiterator.Triggerer
	)

	Context("TriggerOperation()", func() {
		BeforeEach(func() {
			guid = "some-guid"
			instance = service.Instance{GUID: guid}
			fakeBrokerService = new(fakes.FakeBrokerServices)

			subject = instanceiterator.NewUpgradeTriggerer(fakeBrokerService)
		})

		It("returns OperationAccepted when the the instance is ready to be processed", func() {
			fakeBrokerService.ProcessInstanceReturns(services.BOSHOperation{Type: services.OperationAccepted}, nil)

			operation, err := subject.TriggerOperation(instance)
			Expect(err).NotTo(HaveOccurred())
			Expect(operation).To(Equal(services.BOSHOperation{Type: services.OperationAccepted}))

			By("requesting to process an instance")
			Expect(fakeBrokerService.ProcessInstanceCallCount()).To(Equal(1))
			instanceToProcess, _ := fakeBrokerService.ProcessInstanceArgsForCall(0)
			Expect(instanceToProcess).To(Equal(instance))
		})

		It("returns an error if the process instance request fails", func() {
			fakeBrokerService.ProcessInstanceReturns(services.BOSHOperation{}, errors.New("oops"))

			_, err := subject.TriggerOperation(instance)
			Expect(err).To(MatchError(fmt.Sprintf("operation type: upgrade failed for service instance %s: oops", guid)))
		})

		DescribeTable("when operation returns",
			func(operationResult services.BOSHOperationType, expectedOperation services.BOSHOperation) {
				fakeBrokerService.ProcessInstanceReturns(services.BOSHOperation{Type: operationResult}, nil)

				op, err := subject.TriggerOperation(instance)
				Expect(err).NotTo(HaveOccurred())
				Expect(op).To(Equal(expectedOperation))
			},
			Entry("orphan", services.OrphanDeployment, services.BOSHOperation{Type: services.OrphanDeployment}),
			Entry("instance not found", services.InstanceNotFound, services.BOSHOperation{Type: services.InstanceNotFound}),
			Entry("operation in progress", services.OperationInProgress, services.BOSHOperation{Type: services.OperationInProgress}),
		)

		When("it is an Upgrade Triggerer", func() {
			It("sets the operation type to upgrade", func() {
				subject = instanceiterator.NewUpgradeTriggerer(fakeBrokerService)

				_, err := subject.TriggerOperation(service.Instance{GUID: "1234"})
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeBrokerService.ProcessInstanceCallCount()).To(Equal(1))
				_, operationType := fakeBrokerService.ProcessInstanceArgsForCall(0)
				Expect(operationType).To(Equal("upgrade"))
			})
		})

		When("it is a Recreate Triggerer", func() {
			It("sets the operation type to recreate", func() {
				subject = instanceiterator.NewRecreateTriggerer(fakeBrokerService)

				_, err := subject.TriggerOperation(service.Instance{GUID: "1234"})
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeBrokerService.ProcessInstanceCallCount()).To(Equal(1))
				_, operationType := fakeBrokerService.ProcessInstanceArgsForCall(0)
				Expect(operationType).To(Equal("recreate"))
			})
		})
	})

	Context("Check()", func() {
		var (
			expectedOperationData broker.OperationData
		)

		BeforeEach(func() {
			guid = "some-guid"
			expectedOperationData = broker.OperationData{BoshTaskID: 123}
			fakeBrokerService = new(fakes.FakeBrokerServices)
			subject = instanceiterator.NewUpgradeTriggerer(fakeBrokerService)
		})

		It("returns OperationSucceeded when last operation reports success", func() {
			fakeBrokerService.LastOperationReturns(domain.LastOperation{State: domain.Succeeded}, nil)

			state, err := subject.Check(guid, expectedOperationData)
			Expect(err).NotTo(HaveOccurred())

			By("pulling the last operation with the right arguments")
			Expect(fakeBrokerService.LastOperationCallCount()).To(Equal(1))
			guidArg, operationData := fakeBrokerService.LastOperationArgsForCall(0)
			Expect(guidArg).To(Equal(guid))
			Expect(operationData).To(Equal(expectedOperationData))

			Expect(state).To(Equal(services.BOSHOperation{Type: services.OperationSucceeded, Data: expectedOperationData}))
		})

		It("returns an error if it fails to pull last operation", func() {
			fakeBrokerService.LastOperationReturns(domain.LastOperation{}, errors.New("oops"))

			_, err := subject.Check(guid, expectedOperationData)
			Expect(err).To(MatchError("error getting last operation: oops"))
		})

		It("returns OperationFailed when last operation reports failure", func() {
			fakeBrokerService.LastOperationReturns(domain.LastOperation{State: domain.Failed}, nil)

			state, err := subject.Check(guid, expectedOperationData)
			Expect(err).NotTo(HaveOccurred())

			Expect(state).To(Equal(services.BOSHOperation{Type: services.OperationFailed, Data: expectedOperationData}))
		})

		It("returns OperationAccepted when last operation reports the operation is in progress", func() {
			fakeBrokerService.LastOperationReturns(domain.LastOperation{State: domain.InProgress}, nil)

			state, err := subject.Check(guid, expectedOperationData)
			Expect(err).NotTo(HaveOccurred())

			Expect(state).To(Equal(services.BOSHOperation{Type: services.OperationAccepted, Data: expectedOperationData}))
		})

		It("returns an error if last operation returns an unknown state", func() {
			fakeBrokerService.LastOperationReturns(domain.LastOperation{State: "not-a-state"}, nil)

			_, err := subject.Check(guid, expectedOperationData)
			Expect(err).To(MatchError("unknown state from last operation: not-a-state"))
		})
	})
})
