// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrader_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader/fakes"
)

func TestUpgrader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Upgrader Suite")
}

type testState struct {
	instance               service.Instance
	upgradeOutput          []services.BOSHOperationType
	upgradeCallCount       int
	lastOperationOutput    []brokerapi.LastOperationState
	lastOperationCallCount int
	taskID                 int
	controller             *processController
}

func setupTest(states []*testState, instanceLister *fakes.FakeInstanceLister, brokerServices *fakes.FakeBrokerServices) {
	var instances []service.Instance
	for i, s := range states {
		instances = append(instances, s.instance)
		s.controller = newProcessController(fmt.Sprintf("si_%d", i))
	}
	instanceLister.InstancesReturns(instances, nil)
	instanceLister.LatestInstanceInfoStub = func(i service.Instance) (service.Instance, error) {
		return i, nil
	}

	brokerServices.UpgradeInstanceStub = func(instance service.Instance) (services.BOSHOperation, error) {
		for _, s := range states {
			if instance.GUID == s.instance.GUID {
				s.controller.NotifyStart()
				s.upgradeCallCount++
				return services.BOSHOperation{
					Type: s.upgradeOutput[s.upgradeCallCount-1],
					Data: broker.OperationData{BoshTaskID: s.taskID, OperationType: broker.OperationTypeUpgrade},
				}, nil
			}
		}
		return services.BOSHOperation{}, errors.New("unexpected instance GUID")
	}

	brokerServices.LastOperationStub = func(guid string, operationData broker.OperationData) (brokerapi.LastOperation, error) {
		for _, s := range states {
			if guid == s.instance.GUID {
				s.controller.WaitForSignalToProceed()
				s.lastOperationCallCount++
				return brokerapi.LastOperation{
					State: s.lastOperationOutput[s.lastOperationCallCount-1],
				}, nil
			}
		}
		return brokerapi.LastOperation{}, errors.New("unexpected instance GUID")
	}
}

func hasReportedFinished(fakeListener *fakes.FakeListener, expectedOrphans, expectedUpgraded, expectedDeleted int, expectedBusyInstances []string, expectedFailedInstances []string) {
	Expect(fakeListener.FinishedCallCount()).To(Equal(1), "Finished call count")
	orphanCount, upgradedCount, deletedCount, busyInstances, failedInstances := fakeListener.FinishedArgsForCall(0)
	Expect(orphanCount).To(Equal(expectedOrphans), "orphans")
	Expect(upgradedCount).To(Equal(expectedUpgraded), "upgraded")
	Expect(deletedCount).To(Equal(expectedDeleted), "deleted")
	Expect(busyInstances).To(ConsistOf(expectedBusyInstances), "busyInstances")
	Expect(failedInstances).To(ConsistOf(expectedFailedInstances), "failedInstances")
}

func hasSlept(fakeSleeper *fakes.FakeSleeper, callIndex int, expectedInterval time.Duration) {
	Expect(fakeSleeper.SleepCallCount()).To(BeNumerically(">", callIndex))
	Expect(fakeSleeper.SleepArgsForCall(callIndex)).To(Equal(expectedInterval))
}

func hasReportedAttempts(fakeListener *fakes.FakeListener, index, attempt, limit int) {
	Expect(fakeListener.RetryAttemptCallCount()).To(BeNumerically(">", index), "Retries call count")
	c, l := fakeListener.RetryAttemptArgsForCall(index)
	Expect(c).To(Equal(attempt))
	Expect(l).To(Equal(limit))
}

func hasReportedCanaryAttempts(fakeListener *fakes.FakeListener, count, limit, remaining int) {
	Expect(fakeListener.RetryCanariesAttemptCallCount()).To(Equal(count), "Canary retries call count")
	for i := 0; i < count; i++ {
		c, l, r := fakeListener.RetryCanariesAttemptArgsForCall(i)
		Expect(c).To(Equal(i + 1))
		Expect(l).To(Equal(limit))
		Expect(r).To(Equal(remaining))
	}
}

func hasReportedRetries(fakeListener *fakes.FakeListener, expectedPendingInstancesCount ...int) {
	for i, expectedRetryCount := range expectedPendingInstancesCount {
		_, _, _, toRetryCount, _ := fakeListener.ProgressArgsForCall(i)
		Expect(toRetryCount).To(Equal(expectedRetryCount), "Retry count: "+string(i))
	}
}

func hasReportedStarting(fakeListener *fakes.FakeListener, maxInFlight int) {
	Expect(fakeListener.StartingCallCount()).To(Equal(1))
	threads := fakeListener.StartingArgsForCall(0)
	Expect(threads).To(Equal(maxInFlight))
}

func hasReportedProgress(fakeListener *fakes.FakeListener, callIndex int, expectedInterval time.Duration, expectedOrphans, expectedUpgraded, expectedToRetry, expectedDeleted int) {
	Expect(fakeListener.ProgressCallCount()).To(BeNumerically(">", callIndex), "callCount")
	attemptInterval, orphanCount, upgradedCount, toRetryCount, deletedCount := fakeListener.ProgressArgsForCall(callIndex)
	Expect(attemptInterval).To(Equal(expectedInterval), "attempt interval")
	Expect(orphanCount).To(Equal(expectedOrphans), "orphans")
	Expect(upgradedCount).To(Equal(expectedUpgraded), "upgraded")
	Expect(toRetryCount).To(Equal(expectedToRetry), "to retry")
	Expect(deletedCount).To(Equal(expectedDeleted), "deleted")
}

func hasReportedCanariesStarting(fakeListener *fakes.FakeListener, count int, filter config.CanarySelectionParams) {
	Expect(fakeListener.CanariesStartingCallCount()).To(Equal(1), "CanariesStarting() call count")
	canaryCount, actualFilter := fakeListener.CanariesStartingArgsForCall(0)
	Expect(canaryCount).To(Equal(count), "canaryCount")
	Expect(actualFilter).To(Equal(filter), "filter")
}

func hasReportedCanariesFinished(fakeListener *fakes.FakeListener, count int) {
	Expect(fakeListener.CanariesFinishedCallCount()).To(Equal(count), "CanariesFinished() call count")
}

func hasReportedInstanceUpgradeStartResult(fakeListener *fakes.FakeListener, idx int,
	expectedGuid string, expectedStatus services.BOSHOperationType) {

	Expect(fakeListener.InstanceUpgradeStartResultCallCount()).To(BeNumerically(">", idx))
	guid, upgradeType := fakeListener.InstanceUpgradeStartResultArgsForCall(idx)
	Expect(guid).To(Equal(expectedGuid))
	Expect(upgradeType).To(Equal(expectedStatus))
}

func hasReportedInstanceUpgradeStarted(fakeListener *fakes.FakeListener, idx int,
	expectedInstance string, expectedIndex, expectedTotalInstances int, expectedIsDoingCanaries bool) {

	Expect(fakeListener.InstanceUpgradeStartingCallCount()).To(BeNumerically(">", idx))
	instance, index, total, canaryFlag := fakeListener.InstanceUpgradeStartingArgsForCall(idx)
	Expect(instance).To(Equal(expectedInstance))
	Expect(index).To(Equal(expectedIndex), "expected index for instance upgrade started")
	Expect(total).To(Equal(expectedTotalInstances), "expected total num of instances for instance upgrade started")
	Expect(canaryFlag).To(Equal(expectedIsDoingCanaries), "expected is doing canaries")
}

func hasReportedWaitingFor(fakeListener *fakes.FakeListener, idx int, expectedGuid string, expectedTaskID int) {
	Expect(fakeListener.WaitingForCallCount()).To(BeNumerically(">", idx))
	guid, taskID := fakeListener.WaitingForArgsForCall(idx)
	Expect(guid).To(Equal(expectedGuid))
	Expect(taskID).To(Equal(expectedTaskID))
}

func hasReportedUpgradeState(fakeListener *fakes.FakeListener, idx int, expectedGuid, expectedStatus string) {
	Expect(fakeListener.InstanceUpgradedCallCount()).To(BeNumerically(">", idx))

	guid, status := fakeListener.InstanceUpgradedArgsForCall(idx)
	Expect(guid).To(Equal(expectedGuid))
	Expect(status).To(Equal(expectedStatus))
}

func hasReportedInstancesToUpgrade(fakeListener *fakes.FakeListener, instances ...service.Instance) {
	Expect(fakeListener.InstancesToUpgradeCallCount()).To(Equal(1))
	Expect(fakeListener.InstancesToUpgradeArgsForCall(0)).To(Equal(instances))
}

func expectToHaveStarted(controllers ...*processController) {
	for _, c := range controllers {
		c.HasStarted()
	}
}

func expectToHaveNotStarted(controllers ...*processController) {
	for _, c := range controllers {
		c.DoesNotStart()
	}
}

func allowToProceed(controllers ...*processController) {
	for _, c := range controllers {
		c.AllowToProceed()
	}
}

type processController struct {
	name         string
	startedState bool
	started      chan bool
	canProceed   chan bool
}

func newProcessController(name string) *processController {
	return &processController{
		started:    make(chan bool, 1),
		canProceed: make(chan bool, 1),
		name:       name,
	}
}

func (p *processController) NotifyStart() {
	p.started <- true
}

func (p *processController) WaitForSignalToProceed() {
	<-p.canProceed
}

func (p *processController) HasStarted() {
	Eventually(p.started).Should(Receive(), fmt.Sprintf("Process %s expected to be in a started state", p.name))
}

func (p *processController) DoesNotStart() {
	Consistently(p.started).ShouldNot(Receive(), fmt.Sprintf("Process %s expected to be in a non-started state", p.name))
}

func (p *processController) AllowToProceed() {
	p.canProceed <- true
}
