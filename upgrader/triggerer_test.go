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

var _ = Describe("Upgrade triggerer", func() {
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

	It("returns UpgradeAccepted when the the instance is ready to be upgraded", func() {
		fakeInstanceLister.LatestInstanceInfoReturns(latestInstance, nil)
		fakeBrokerService.UpgradeInstanceReturns(services.UpgradeOperation{Type: services.UpgradeAccepted}, nil)

		operation, err := t.TriggerUpgrade(instance)
		Expect(err).NotTo(HaveOccurred())
		Expect(operation).To(Equal(services.UpgradeOperation{Type: services.UpgradeAccepted}))

		By("checking the latest instance info")
		Expect(fakeInstanceLister.LatestInstanceInfoCallCount()).To(Equal(1))
		instanceToCheck := fakeInstanceLister.LatestInstanceInfoArgsForCall(0)
		Expect(instanceToCheck).To(Equal(instance))

		By("requesting to upgrade an instance")
		Expect(fakeBrokerService.UpgradeInstanceCallCount()).To(Equal(1))
		instanceToUpgrade := fakeBrokerService.UpgradeInstanceArgsForCall(0)
		Expect(instanceToUpgrade).To(Equal(latestInstance))
	})

	It("does not return an error if cannot check the latest instance info", func() {
		fakeInstanceLister.LatestInstanceInfoReturns(service.Instance{}, errors.New("oops"))
		operation, err := t.TriggerUpgrade(instance)
		Expect(err).NotTo(HaveOccurred())
		Expect(operation).To(Equal(services.UpgradeOperation{}))

		Expect(fakeBrokerService.UpgradeInstanceCallCount()).To(Equal(1))
		instanceToUpgrade := fakeBrokerService.UpgradeInstanceArgsForCall(0)
		Expect(instanceToUpgrade).To(Equal(instance))

		Expect(fakeListener.FailedToRefreshInstanceInfoCallCount()).To(Equal(1))
		guidArg := fakeListener.FailedToRefreshInstanceInfoArgsForCall(0)
		Expect(guidArg).To(Equal(guid))
	})

	It("returns InstanceNotFound if the instance could not be found", func() {
		fakeInstanceLister.LatestInstanceInfoReturns(service.Instance{}, service.InstanceNotFound)
		operation, err := t.TriggerUpgrade(instance)
		Expect(err).NotTo(HaveOccurred())
		Expect(operation).To(Equal(services.UpgradeOperation{Type: services.InstanceNotFound}))
	})

	It("returns an error if the upgrade request fails", func() {
		fakeBrokerService.UpgradeInstanceReturns(services.UpgradeOperation{}, errors.New("oops"))
		_, err := t.TriggerUpgrade(instance)
		Expect(err).To(MatchError(fmt.Sprintf("Upgrade failed for service instance %s: oops", guid)))
	})

	DescribeTable("when upgrades returns",
		func(upgradeResult services.UpgradeOperationType, expectedOperation services.UpgradeOperation) {
			fakeBrokerService.UpgradeInstanceReturns(services.UpgradeOperation{Type: upgradeResult}, nil)
			op, err := t.TriggerUpgrade(instance)
			Expect(err).NotTo(HaveOccurred())
			Expect(op).To(Equal(expectedOperation))
		},
		Entry("orphan", services.OrphanDeployment, services.UpgradeOperation{Type: services.OrphanDeployment}),
		Entry("instance not found", services.InstanceNotFound, services.UpgradeOperation{Type: services.InstanceNotFound}),
		Entry("operation in progress", services.OperationInProgress, services.UpgradeOperation{Type: services.OperationInProgress}),
	)
})
