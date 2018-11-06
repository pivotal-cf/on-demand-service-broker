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

package upgrader_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader/fakes"
)

var _ = Describe("Operation triggerer", func() {
	var (
		guid               string
		instance           service.Instance
		latestInstance     service.Instance
		fakeBrokerService  *fakes.FakeBrokerServices
		fakeInstanceLister *fakes.FakeInstanceLister
		fakeListener       *fakes.FakeListener
		t                  upgrader.Triggerer
	)

	BeforeEach(func() {
		guid = "some-guid"
		instance = service.Instance{GUID: guid}
		latestInstance = service.Instance{GUID: guid, PlanUniqueID: "plan-unique-id"}
		fakeBrokerService = new(fakes.FakeBrokerServices)
		fakeInstanceLister = new(fakes.FakeInstanceLister)
		fakeListener = new(fakes.FakeListener)
		t = upgrader.NewTriggerer(fakeBrokerService, fakeInstanceLister, fakeListener)
	})

	It("returns OperationAccepted when the the instance is ready to be processed", func() {
		fakeInstanceLister.LatestInstanceInfoReturns(latestInstance, nil)
		fakeBrokerService.ProcessInstanceReturns(services.BOSHOperation{Type: services.OperationAccepted}, nil)

		operation, err := t.TriggerOperation(instance)
		Expect(err).NotTo(HaveOccurred())
		Expect(operation).To(Equal(services.BOSHOperation{Type: services.OperationAccepted}))

		By("checking the latest instance info")
		Expect(fakeInstanceLister.LatestInstanceInfoCallCount()).To(Equal(1))
		instanceToCheck := fakeInstanceLister.LatestInstanceInfoArgsForCall(0)
		Expect(instanceToCheck).To(Equal(instance))

		By("requesting to process an instance")
		Expect(fakeBrokerService.ProcessInstanceCallCount()).To(Equal(1))
		instanceToProcess := fakeBrokerService.ProcessInstanceArgsForCall(0)
		Expect(instanceToProcess).To(Equal(latestInstance))
	})

	It("does not return an error if cannot check the latest instance info", func() {
		fakeInstanceLister.LatestInstanceInfoReturns(service.Instance{}, errors.New("oops"))

		operation, err := t.TriggerOperation(instance)
		Expect(err).NotTo(HaveOccurred())
		Expect(operation).To(Equal(services.BOSHOperation{}))

		Expect(fakeBrokerService.ProcessInstanceCallCount()).To(Equal(1))
		instanceToProcess := fakeBrokerService.ProcessInstanceArgsForCall(0)
		Expect(instanceToProcess).To(Equal(instance))

		Expect(fakeListener.FailedToRefreshInstanceInfoCallCount()).To(Equal(1))
		guidArg := fakeListener.FailedToRefreshInstanceInfoArgsForCall(0)
		Expect(guidArg).To(Equal(guid))
	})

	It("returns InstanceNotFound if the instance could not be found", func() {
		fakeInstanceLister.LatestInstanceInfoReturns(service.Instance{}, service.InstanceNotFound)
		operation, err := t.TriggerOperation(instance)
		Expect(err).NotTo(HaveOccurred())
		Expect(operation).To(Equal(services.BOSHOperation{Type: services.InstanceNotFound}))
	})

	It("returns an error if the process instance request fails", func() {
		fakeBrokerService.ProcessInstanceReturns(services.BOSHOperation{}, errors.New("oops"))

		_, err := t.TriggerOperation(instance)
		Expect(err).To(MatchError(fmt.Sprintf("Operation type: upgrade failed for service instance %s: oops", guid)))
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
