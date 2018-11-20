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
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator"
	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

var _ = Describe("Operation triggerer", func() {
	var (
		guid              string
		instance          service.Instance
		fakeBrokerService *fakes.FakeBrokerServices
		t                 instanceiterator.Triggerer
		operationType     = "upgrade"
	)

	Context("with an upgradeTriggerer", func() {

		BeforeEach(func() {
			guid = "some-guid"
			instance = service.Instance{GUID: guid}
			fakeBrokerService = new(fakes.FakeBrokerServices)

			var err error
			t = instanceiterator.NewUpgradeTriggerer(fakeBrokerService)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns OperationAccepted when the the instance is ready to be processed", func() {
			fakeBrokerService.ProcessInstanceReturns(services.BOSHOperation{Type: services.OperationAccepted}, nil)

			operation, err := t.TriggerOperation(instance)
			Expect(err).NotTo(HaveOccurred())
			Expect(operation).To(Equal(services.BOSHOperation{Type: services.OperationAccepted}))

			By("requesting to process an instance")
			Expect(fakeBrokerService.ProcessInstanceCallCount()).To(Equal(1))
			instanceToProcess, _ := fakeBrokerService.ProcessInstanceArgsForCall(0)
			Expect(instanceToProcess).To(Equal(instance))
		})

		It("returns an error if the process instance request fails", func() {
			fakeBrokerService.ProcessInstanceReturns(services.BOSHOperation{}, errors.New("oops"))

			_, err := t.TriggerOperation(instance)
			Expect(err).To(MatchError(fmt.Sprintf("Operation type: %s failed for service instance %s: oops", operationType, guid)))
		})

		DescribeTable("when operation returns",
			func(operationResult services.BOSHOperationType, expectedOperation services.BOSHOperation) {
				fakeBrokerService.ProcessInstanceReturns(services.BOSHOperation{Type: operationResult}, nil)

				op, err := t.TriggerOperation(instance)
				Expect(err).NotTo(HaveOccurred())
				Expect(op).To(Equal(expectedOperation))
			},
			Entry("orphan", services.OrphanDeployment, services.BOSHOperation{Type: services.OrphanDeployment}),
			Entry("instance not found", services.InstanceNotFound, services.BOSHOperation{Type: services.InstanceNotFound}),
			Entry("operation in progress", services.OperationInProgress, services.BOSHOperation{Type: services.OperationInProgress}),
		)
	})

	Context("with a recreateTriggerer", func() {
		BeforeEach(func() {
			guid = "some-guid"
			instance = service.Instance{GUID: guid}
			fakeBrokerService = new(fakes.FakeBrokerServices)

			var err error
			t = instanceiterator.NewRecreateTriggerer(fakeBrokerService)
			Expect(err).NotTo(HaveOccurred())
		})
		It("returns OperationAccepted when the the instance is ready to be processed", func() {
			fakeBrokerService.ProcessInstanceReturns(services.BOSHOperation{Type: services.OperationAccepted}, nil)

			operation, err := t.TriggerOperation(instance)
			Expect(err).NotTo(HaveOccurred())
			Expect(operation).To(Equal(services.BOSHOperation{Type: services.OperationAccepted}))

			By("requesting to process an instance")
			Expect(fakeBrokerService.ProcessInstanceCallCount()).To(Equal(1))
			instanceToProcess, _ := fakeBrokerService.ProcessInstanceArgsForCall(0)
			Expect(instanceToProcess).To(Equal(instance))
		})

		It("returns an error if the process instance request fails", func() {
			fakeBrokerService.ProcessInstanceReturns(services.BOSHOperation{}, errors.New("oops"))

			_, err := t.TriggerOperation(instance)
			Expect(err).To(MatchError(fmt.Sprintf("Operation type: recreate failed for service instance %s: oops", guid)))
		})

		DescribeTable("when operation returns",
			func(operationResult services.BOSHOperationType, expectedOperation services.BOSHOperation) {
				fakeBrokerService.ProcessInstanceReturns(services.BOSHOperation{Type: operationResult}, nil)

				op, err := t.TriggerOperation(instance)
				Expect(err).NotTo(HaveOccurred())
				Expect(op).To(Equal(expectedOperation))
			},
			Entry("orphan", services.OrphanDeployment, services.BOSHOperation{Type: services.OrphanDeployment}),
			Entry("instance not found", services.InstanceNotFound, services.BOSHOperation{Type: services.InstanceNotFound}),
			Entry("operation in progress", services.OperationInProgress, services.BOSHOperation{Type: services.OperationInProgress}),
		)
	})
})
