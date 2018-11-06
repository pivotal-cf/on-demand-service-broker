// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrader_test

import (
	"errors"
	"fmt"
	"time"

	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader/fakes"
)

var _ = Describe("Upgrader", func() {

	var (
		emptyBusyList        = []string{}
		emptyFailedList      = []string{}
		fakeListener         *fakes.FakeListener
		brokerServicesClient *fakes.FakeBrokerServices
		instanceLister       *fakes.FakeInstanceLister
		fakeSleeper          *fakes.FakeSleeper

		upgraderBuilder upgrader.Builder

		upgradeErr error
	)

	BeforeEach(func() {
		fakeListener = new(fakes.FakeListener)
		brokerServicesClient = new(fakes.FakeBrokerServices)
		instanceLister = new(fakes.FakeInstanceLister)
		fakeSleeper = new(fakes.FakeSleeper)

		upgraderBuilder = upgrader.Builder{
			BrokerServices:        brokerServicesClient,
			ServiceInstanceLister: instanceLister,
			Listener:              fakeListener,
			PollingInterval:       10 * time.Second,
			AttemptLimit:          5,
			AttemptInterval:       60 * time.Second,
			MaxInFlight:           1,
			Canaries:              0,
			Sleeper:               fakeSleeper,
		}
	})

	Context("requests error", func() {
		It("fails when cannot get the list of all the instances", func() {
			instanceLister.InstancesReturns([]service.Instance{}, errors.New("oops"))
			u := upgrader.New(&upgraderBuilder)
			err := u.Upgrade()
			Expect(err).To(MatchError(ContainSubstring("oops")))
		})

		It("fails when cannot start upgrading an instance", func() {
			instanceLister.InstancesReturns([]service.Instance{{GUID: "1"}}, nil)
			brokerServicesClient.UpgradeInstanceReturns(services.BOSHOperation{}, errors.New("oops"))

			u := upgrader.New(&upgraderBuilder)
			err := u.Upgrade()
			Expect(err).To(MatchError(ContainSubstring("oops")))
		})

		It("fails when cannot poll last operation", func() {
			instanceLister.InstancesReturns([]service.Instance{{GUID: "1"}}, nil)
			brokerServicesClient.UpgradeInstanceReturns(services.BOSHOperation{Type: services.OperationAccepted}, nil)
			brokerServicesClient.LastOperationReturns(brokerapi.LastOperation{}, errors.New("oops"))

			u := upgrader.New(&upgraderBuilder)
			err := u.Upgrade()
			Expect(err).To(MatchError(ContainSubstring("oops")))
		})
	})

	Context("plan change in-flight", func() {
		It("uses the new plan for the upgrade", func() {
			instanceLister.InstancesReturnsOnCall(0, []service.Instance{{GUID: "1", PlanUniqueID: "plan-id-1"}}, nil)
			instanceLister.LatestInstanceInfoReturnsOnCall(0, service.Instance{GUID: "1", PlanUniqueID: "plan-id-2"}, nil)
			brokerServicesClient.UpgradeInstanceReturns(services.BOSHOperation{Type: services.OperationAccepted}, nil)
			brokerServicesClient.LastOperationReturns(brokerapi.LastOperation{State: brokerapi.Succeeded}, nil)

			upgradeTool := upgrader.New(&upgraderBuilder)
			err := upgradeTool.Upgrade()
			Expect(err).NotTo(HaveOccurred())

			instance := brokerServicesClient.UpgradeInstanceArgsForCall(0)
			Expect(instance.PlanUniqueID).To(Equal("plan-id-2"))
		})

		It("continues the upgrade using the previously fetched info if latest instance info call errors", func() {
			instanceLister.InstancesReturnsOnCall(0, []service.Instance{{GUID: "1", PlanUniqueID: "plan-id-1"}}, nil)
			instanceLister.LatestInstanceInfoReturnsOnCall(0, service.Instance{}, errors.New("unexpected error"))
			brokerServicesClient.UpgradeInstanceReturns(services.BOSHOperation{Type: services.OperationAccepted}, nil)
			brokerServicesClient.LastOperationReturns(brokerapi.LastOperation{State: brokerapi.Succeeded}, nil)

			upgradeTool := upgrader.New(&upgraderBuilder)
			err := upgradeTool.Upgrade()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeListener.FailedToRefreshInstanceInfoCallCount()).To(Equal(1))
		})

		It("marks an instance-not-found on refresh as deleted", func() {
			instanceLister.InstancesReturnsOnCall(0, []service.Instance{{GUID: "1", PlanUniqueID: "plan-id-1"}}, nil)
			instanceLister.LatestInstanceInfoReturnsOnCall(0, service.Instance{}, service.InstanceNotFound)

			upgradeTool := upgrader.New(&upgraderBuilder)
			err := upgradeTool.Upgrade()
			Expect(err).NotTo(HaveOccurred())

			hasReportedFinished(fakeListener, 0, 0, 1, emptyBusyList, emptyFailedList)
			hasReportedInstanceUpgradeStartResult(fakeListener, 0, "1", services.InstanceNotFound)
		})
	})

	Context("upgrades instances without canaries", func() {
		AfterEach(func() {
			hasReportedStarting(fakeListener, upgraderBuilder.MaxInFlight)
			Expect(fakeListener.CanariesStartingCallCount()).To(Equal(0), "Expected canaries starting to not be called")
			Expect(fakeListener.CanariesFinishedCallCount()).To(Equal(0), "Expected canaries finished to not be called")
		})

		It("succeeds", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)

			allowToProceed(states[0].controller)

			expectToHaveStarted(states[1].controller)
			expectToHaveNotStarted(states[2].controller)

			allowToProceed(states[1].controller)
			expectToHaveStarted(states[2].controller)

			allowToProceed(states[2].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasReportedFinished(fakeListener, 0, 3, 0, []string{}, []string{})
		})

		It("does not fail and reports a deleted instance", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.InstanceNotFound}, lastOperationOutput: []brokerapi.LastOperationState{}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)

			allowToProceed(states[0].controller)

			expectToHaveStarted(states[1].controller)
			expectToHaveNotStarted(states[2].controller)

			allowToProceed(states[1].controller)
			expectToHaveStarted(states[2].controller)

			allowToProceed(states[2].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasReportedFinished(fakeListener, 0, 2, 1, []string{}, []string{})
		})

		It("does not fail and reports an orphaned instance", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OrphanDeployment}, lastOperationOutput: []brokerapi.LastOperationState{}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)

			allowToProceed(states[0].controller)

			expectToHaveStarted(states[1].controller)
			expectToHaveNotStarted(states[2].controller)

			allowToProceed(states[1].controller)
			expectToHaveStarted(states[2].controller)

			allowToProceed(states[2].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasReportedFinished(fakeListener, 1, 2, 0, []string{}, []string{})
		})

		It("polls last_operation endpoint when ugprade is not instant", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.InProgress, brokerapi.InProgress, brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)

			allowToProceed(states[0].controller)

			expectToHaveStarted(states[1].controller)
			expectToHaveNotStarted(states[2].controller)

			allowToProceed(states[1].controller)
			expectToHaveNotStarted(states[2].controller)

			allowToProceed(states[1].controller)
			expectToHaveNotStarted(states[2].controller)

			allowToProceed(states[1].controller)
			expectToHaveStarted(states[2].controller)

			allowToProceed(states[2].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasSlept(fakeSleeper, 0, upgraderBuilder.PollingInterval)
			hasSlept(fakeSleeper, 1, upgraderBuilder.PollingInterval)
			hasReportedFinished(fakeListener, 0, 3, 0, []string{}, []string{})
		})

		It("retries busy instances until the upgrade request is accepted", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationInProgress, services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)

			allowToProceed(states[0].controller)
			expectToHaveStarted(states[1].controller)

			expectToHaveStarted(states[2].controller)
			allowToProceed(states[2].controller)

			expectToHaveStarted(states[1].controller)
			allowToProceed(states[1].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasReportedRetries(fakeListener, 1, 1, 0)
			hasReportedAttempts(fakeListener, 2, 3, 5)
			hasReportedFinished(fakeListener, 0, 3, 0, []string{}, []string{})
		})

		It("fails when retrying busy instances reach the attempt limit", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationInProgress, services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.AttemptLimit = 1
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)

			allowToProceed(states[0].controller)
			expectToHaveStarted(states[1].controller)

			expectToHaveStarted(states[2].controller)
			allowToProceed(states[2].controller)

			wg.Wait()

			Expect(upgradeErr).To(MatchError(ContainSubstring("The following instances could not be processed:")))

			hasReportedRetries(fakeListener, 1)
			hasReportedAttempts(fakeListener, 0, 1, 1)
			hasReportedFinished(fakeListener, 0, 2, 0, []string{states[1].instance.GUID}, []string{})
		})

		It("returns an error when an last operation returns a failure", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Failed}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.AttemptLimit = 1
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)

			allowToProceed(states[0].controller)
			expectToHaveStarted(states[1].controller)

			allowToProceed(states[1].controller)

			wg.Wait()

			Expect(upgradeErr).To(MatchError(ContainSubstring(fmt.Sprintf("[%s] Operation failed: bosh task id %d", states[1].instance.GUID, states[1].taskID))))

			hasReportedFinished(fakeListener, 0, 1, 0, []string{}, []string{states[1].instance.GUID})
		})

		It("retries until a deleted instance is detected", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationInProgress, services.InstanceNotFound}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)

			allowToProceed(states[0].controller)
			expectToHaveStarted(states[1].controller)

			expectToHaveStarted(states[2].controller)
			allowToProceed(states[2].controller)

			expectToHaveStarted(states[1].controller)
			allowToProceed(states[1].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasReportedRetries(fakeListener, 1, 1, 0)
			hasReportedAttempts(fakeListener, 2, 3, 5)
			hasReportedFinished(fakeListener, 0, 2, 1, []string{}, []string{})
		})

		It("retries until an orphaned instance is detected", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationInProgress, services.OrphanDeployment}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)

			allowToProceed(states[0].controller)
			expectToHaveStarted(states[1].controller)

			expectToHaveStarted(states[2].controller)
			allowToProceed(states[2].controller)

			expectToHaveStarted(states[1].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasReportedRetries(fakeListener, 1, 1, 0)
			hasReportedAttempts(fakeListener, 2, 3, 5)
			hasReportedFinished(fakeListener, 1, 2, 0, []string{}, []string{})
		})

		It("upgrades in batches when max_in_flight is greater than 1", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
				{instance: service.Instance{GUID: "4"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 4},
				{instance: service.Instance{GUID: "5"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 5},
				{instance: service.Instance{GUID: "6"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 6},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.MaxInFlight = 4
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller, states[2].controller, states[3].controller)
			expectToHaveNotStarted(states[4].controller, states[5].controller)

			allowToProceed(states[0].controller, states[1].controller, states[2].controller, states[3].controller)
			expectToHaveStarted(states[4].controller, states[5].controller)

			allowToProceed(states[4].controller, states[5].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasReportedFinished(fakeListener, 0, 6, 0, []string{}, []string{})
		})

		It("returns multiple errors if multiple instances fail to upgrade", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Failed}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Failed}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.MaxInFlight = 2
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller)
			expectToHaveNotStarted(states[2].controller)
			allowToProceed(states[0].controller, states[1].controller)

			wg.Wait()

			Expect(upgradeErr).To(HaveOccurred())
			Expect(upgradeErr.Error()).To(SatisfyAll(
				ContainSubstring("2 errors occurred"),
				ContainSubstring(fmt.Sprintf("[%s] Operation failed: bosh task id %d: ", states[0].instance.GUID, states[0].taskID)),
				ContainSubstring(fmt.Sprintf("[%s] Operation failed: bosh task id %d: ", states[1].instance.GUID, states[1].taskID)),
			))
			hasReportedUpgradeState(fakeListener, 0, states[0].instance.GUID, "failure")
			hasReportedUpgradeState(fakeListener, 1, states[1].instance.GUID, "failure")
			hasReportedFinished(fakeListener, 0, 0, 0, []string{}, []string{states[0].instance.GUID, states[1].instance.GUID})
		})
	})

	Context("upgrade instances with canaries", func() {
		AfterEach(func() {
			hasReportedStarting(fakeListener, upgraderBuilder.MaxInFlight)
		})

		It("succeeds upgrading first a canary instance", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
				{instance: service.Instance{GUID: "4"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 4},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.Canaries = 2
			upgraderBuilder.MaxInFlight = 3
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			// upgrade canary instances
			expectToHaveStarted(states[0].controller, states[1].controller)
			expectToHaveNotStarted(states[2].controller, states[3].controller)
			allowToProceed(states[0].controller, states[1].controller)

			// upgrade the rest
			expectToHaveStarted(states[2].controller, states[3].controller)
			allowToProceed(states[2].controller, states[3].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasReportedCanariesStarting(fakeListener, upgraderBuilder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 1)
			hasReportedFinished(fakeListener, 0, 4, 0, []string{}, []string{})
		})

		It("succeeds upgrading using max_in_flight as batch size if it is smaller than the number of required canaries", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
				{instance: service.Instance{GUID: "4"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 4},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.Canaries = 4
			upgraderBuilder.MaxInFlight = 3
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller, states[2].controller)
			expectToHaveNotStarted(states[3].controller)
			allowToProceed(states[0].controller, states[1].controller, states[2].controller)

			expectToHaveStarted(states[3].controller)
			allowToProceed(states[3].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasReportedCanariesStarting(fakeListener, upgraderBuilder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 1)
			hasReportedFinished(fakeListener, 0, 4, 0, []string{}, []string{})
		})

		It("does not fail if there are no instances", func() {
			setupTest([]*testState{}, instanceLister, brokerServicesClient)

			upgraderBuilder.Canaries = 2
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())
			hasReportedFinished(fakeListener, 0, 0, 0, []string{}, []string{})
		})

		It("stops upgrading if a canary instance fails to upgrade", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Failed}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.Canaries = 1
			upgraderBuilder.MaxInFlight = 3
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)
			allowToProceed(states[0].controller)

			expectToHaveNotStarted(states[1].controller, states[2].controller)
			wg.Wait()

			Expect(upgradeErr).To(MatchError(ContainSubstring("canaries didn't upgrade successfully")))

			hasReportedCanariesStarting(fakeListener, upgraderBuilder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 0)
			hasReportedFinished(fakeListener, 0, 0, 0, []string{}, []string{states[0].instance.GUID})
		})

		It("picks another canary instance if one is busy", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.Canaries = 1
			upgraderBuilder.MaxInFlight = 3
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller, states[2].controller)
			allowToProceed(states[2].controller)

			allowToProceed(states[0].controller, states[1].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasReportedCanariesStarting(fakeListener, upgraderBuilder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 1)
			hasReportedFinished(fakeListener, 0, 3, 0, []string{}, []string{})
		})

		It("picks another canary instance if one is deleted", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.InstanceNotFound}, lastOperationOutput: []brokerapi.LastOperationState{}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.Canaries = 1
			upgraderBuilder.MaxInFlight = 3
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller)
			expectToHaveNotStarted(states[2].controller)
			allowToProceed(states[1].controller)

			allowToProceed(states[2].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasReportedCanariesStarting(fakeListener, upgraderBuilder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 1)
			hasReportedFinished(fakeListener, 0, 2, 1, []string{}, []string{})
		})

		It("picks another canary instance if one is orphaned", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OrphanDeployment}, lastOperationOutput: []brokerapi.LastOperationState{}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.Canaries = 1
			upgraderBuilder.MaxInFlight = 3
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller)
			expectToHaveNotStarted(states[2].controller)
			allowToProceed(states[1].controller)

			allowToProceed(states[2].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasReportedCanariesStarting(fakeListener, upgraderBuilder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 1)
			hasReportedFinished(fakeListener, 1, 2, 0, []string{}, []string{})
		})

		It("retries busy canaries if needed", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.Canaries = 3
			upgraderBuilder.MaxInFlight = 3
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller, states[2].controller)
			allowToProceed(states[0].controller, states[1].controller, states[2].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasReportedCanariesStarting(fakeListener, upgraderBuilder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 1)
			hasReportedFinished(fakeListener, 0, 3, 0, []string{}, []string{})
		})

		It("fails when reaching the attempt limit retrying canaries", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress}, lastOperationOutput: []brokerapi.LastOperationState{}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress}, lastOperationOutput: []brokerapi.LastOperationState{}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.Canaries = 3
			upgraderBuilder.MaxInFlight = 3
			upgraderBuilder.AttemptLimit = 1
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller, states[2].controller)
			allowToProceed(states[0].controller, states[1].controller, states[2].controller)

			wg.Wait()

			Expect(upgradeErr).To(MatchError(ContainSubstring(
				"canaries didn't upgrade successfully: attempted to upgrade 3 canaries, but only found 1 instances not already in use by another BOSH task.",
			)))

			hasReportedFinished(fakeListener, 0, 1, 0, []string{states[1].instance.GUID, states[2].instance.GUID}, []string{})
		})

		It("retries busy instances after all canaries have passed", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationInProgress, services.OperationInProgress, services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.Canaries = 1
			upgraderBuilder.MaxInFlight = 3
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			// canaries
			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)
			allowToProceed(states[0].controller)
			// rest attempt 1
			expectToHaveStarted(states[1].controller, states[2].controller)
			// attempt 2
			expectToHaveStarted(states[1].controller, states[2].controller)
			allowToProceed(states[1].controller)
			// attempt 3
			expectToHaveStarted(states[2].controller)
			// attempt 4
			expectToHaveStarted(states[2].controller)
			allowToProceed(states[2].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			hasReportedCanariesStarting(fakeListener, upgraderBuilder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 1)
			hasReportedAttempts(fakeListener, 3, 4, 5)
			hasReportedFinished(fakeListener, 0, 3, 0, []string{}, []string{})
		})

		It("reports count status accurately when retrying in canaries and rest", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
				{instance: service.Instance{GUID: "4"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationInProgress, services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 4},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.Canaries = 2
			upgraderBuilder.MaxInFlight = 3
			upgraderBuilder.AttemptLimit = 2
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			// Retry attempt 1: Canaries
			expectToHaveStarted(states[0].controller, states[1].controller, states[2].controller, states[3].controller)
			allowToProceed(states[0].controller)

			// Retry attempt 2: Canaries
			expectToHaveStarted(states[1].controller)
			expectToHaveNotStarted(states[2].controller, states[3].controller)
			allowToProceed(states[1].controller)
			// Canaries completed

			// Retry attempt 1: Upgrade
			expectToHaveStarted(states[2].controller, states[3].controller)
			allowToProceed(states[2].controller)

			// Retry attemp 2 : Upgrade
			expectToHaveStarted(states[3].controller)
			allowToProceed(states[3].controller)

			wg.Wait()

			Expect(upgradeErr).NotTo(HaveOccurred())

			expectCanariesRetryCallCount := 2
			Expect(fakeListener.RetryCanariesAttemptCallCount()).To(Equal(expectCanariesRetryCallCount))
			expectedCanariesParams := [][]int{
				{1, 2, 2},
				{2, 2, 1},
			}
			for i := 0; i < expectCanariesRetryCallCount; i++ {
				a, t, c := fakeListener.RetryCanariesAttemptArgsForCall(i)
				Expect(a).To(Equal(expectedCanariesParams[i][0]))
				Expect(t).To(Equal(expectedCanariesParams[i][1]))
				Expect(c).To(Equal(expectedCanariesParams[i][2]))
			}
			expectedCallCount := 2
			Expect(fakeListener.RetryAttemptCallCount()).To(Equal(expectedCallCount))
			expectedParams := [][]int{
				{1, 2},
				{2, 2},
			}
			for i := 0; i < expectedCallCount; i++ {
				a, t := fakeListener.RetryAttemptArgsForCall(i)
				Expect(a).To(Equal(expectedParams[i][0]))
				Expect(t).To(Equal(expectedParams[i][1]))
			}

			expectedInstanceCounts := [][]int{
				{1, 4, 1},
				{2, 4, 1},
				{2, 4, 1},
				{2, 4, 1},
				{2, 4, 1},
				{3, 4, 0},
				{4, 4, 0},
				{4, 4, 0},
			}
			for i := 0; i < fakeListener.InstanceOperationStartingCallCount(); i++ {
				_, index, total, isCanary := fakeListener.InstanceOperationStartingArgsForCall(i)
				Expect(index).To(Equal(expectedInstanceCounts[i][0]), fmt.Sprintf("Current instance index; i = %d", i))
				Expect(total).To(Equal(expectedInstanceCounts[i][1]), "Total pending instances")
				Expect(isCanary).To(Equal(expectedInstanceCounts[i][2] == 1), "Is Canary")
			}
			hasReportedFinished(fakeListener, 0, 4, 0, []string{}, []string{})
		})

		It("reports the progress of an upgrade", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.InstanceNotFound}, lastOperationOutput: []brokerapi.LastOperationState{}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OrphanDeployment}, lastOperationOutput: []brokerapi.LastOperationState{}, taskID: 3},
				{instance: service.Instance{GUID: "4"}, upgradeOutput: []services.BOSHOperationType{services.OperationInProgress, services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 4},
			}
			setupTest(states, instanceLister, brokerServicesClient)

			upgraderBuilder.Canaries = 1
			upgraderBuilder.MaxInFlight = 3
			upgradeTool := upgrader.New(&upgraderBuilder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				upgradeErr = upgradeTool.Upgrade()
			}()

			// Logs for canaries
			{
				expectToHaveStarted(states[0].controller)

				hasReportedStarting(fakeListener, 3)
				hasReportedInstancesToUpgrade(fakeListener, states[0].instance, states[1].instance, states[2].instance, states[3].instance)

				hasReportedCanariesStarting(fakeListener, 1, nil)
				hasReportedCanaryAttempts(fakeListener, 1, 5, 1)
				hasReportedInstanceUpgradeStarted(fakeListener, 0, "1", 1, 4, true)
				hasReportedInstanceUpgradeStartResult(fakeListener, 0, "1", services.OperationAccepted)
				hasReportedWaitingFor(fakeListener, 0, "1", 1)
				allowToProceed(states[0].controller)

				// Retry attempt 1: Upgrade
				expectToHaveStarted(states[1].controller, states[2].controller, states[3].controller)
				hasReportedUpgradeState(fakeListener, 0, "1", "success")
				hasReportedCanariesFinished(fakeListener, 1)
			}

			// Logs for upgrade attempt 1
			{
				hasReportedAttempts(fakeListener, 0, 1, 5)
				hasReportedInstanceUpgradeStarted(fakeListener, 1, "2", 2, 4, false)
				hasReportedInstanceUpgradeStartResult(fakeListener, 1, "2", services.InstanceNotFound)

				hasReportedInstanceUpgradeStarted(fakeListener, 2, "3", 3, 4, false)
				hasReportedInstanceUpgradeStartResult(fakeListener, 2, "3", services.OrphanDeployment)

				hasReportedInstanceUpgradeStarted(fakeListener, 3, "4", 4, 4, false)
				hasReportedInstanceUpgradeStartResult(fakeListener, 3, "4", services.OperationInProgress)

				hasReportedProgress(fakeListener, 0, upgraderBuilder.AttemptInterval, 1, 1, 1, 1)

				// Retry attempt 2: Upgrade
				expectToHaveStarted(states[3].controller)
				allowToProceed(states[3].controller)
			}

			wg.Wait()
			Expect(upgradeErr).NotTo(HaveOccurred())

			// Logs for upgrade attempt 2
			{
				hasReportedAttempts(fakeListener, 1, 2, 5)
				hasReportedInstanceUpgradeStarted(fakeListener, 4, "4", 4, 4, false)
				hasReportedInstanceUpgradeStartResult(fakeListener, 4, "4", services.OperationAccepted)
				hasReportedWaitingFor(fakeListener, 1, "4", 4)
				hasReportedUpgradeState(fakeListener, 1, "4", "success")
				hasReportedProgress(fakeListener, 1, upgraderBuilder.AttemptInterval, 1, 2, 0, 1)
				hasReportedFinished(fakeListener, 1, 2, 1, []string{}, []string{})
			}
		})

		When("canary selection params is specified", func() {
			BeforeEach(func() {
				upgraderBuilder.CanarySelectionParams = config.CanarySelectionParams{
					"org":   "the-org",
					"space": "the-space",
				}
			})

			AfterEach(func() {
				Expect(instanceLister.FilteredInstancesCallCount()).To(Equal(1))
				params := instanceLister.FilteredInstancesArgsForCall(0)
				Expect(params["org"]).To(Equal("the-org"))
				Expect(params["space"]).To(Equal("the-space"))
			})

			It("uses canaries matching the selection criteria", func() {
				upgraderBuilder.MaxInFlight = 2
				upgraderBuilder.Canaries = 3
				upgradeTool := upgrader.New(&upgraderBuilder)

				states := []*testState{
					{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
					{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
					{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
					{instance: service.Instance{GUID: "4"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 4},
					{instance: service.Instance{GUID: "5"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 5},
					{instance: service.Instance{GUID: "6"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 6},
				}
				setupTest(states, instanceLister, brokerServicesClient)

				filtered := []service.Instance{states[1].instance, states[2].instance, states[4].instance, states[5].instance}
				instanceLister.FilteredInstancesReturns(filtered, nil)

				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					upgradeErr = upgradeTool.Upgrade()
				}()

				expectToHaveNotStarted(states[0].controller, states[3].controller, states[4].controller, states[5].controller)
				expectToHaveStarted(states[1].controller, states[2].controller)

				allowToProceed(states[1].controller, states[2].controller)
				expectToHaveStarted(states[4].controller)

				allowToProceed(states[4].controller)

				expectToHaveStarted(states[0].controller, states[3].controller)
				allowToProceed(states[0].controller, states[3].controller)

				expectToHaveStarted(states[5].controller)
				allowToProceed(states[5].controller)

				wg.Wait()

				Expect(upgradeErr).NotTo(HaveOccurred())

				hasReportedCanariesStarting(fakeListener, upgraderBuilder.Canaries, upgraderBuilder.CanarySelectionParams)
				hasReportedCanariesFinished(fakeListener, 1)
				hasReportedFinished(fakeListener, 0, 6, 0, []string{}, []string{})
			})

			It("ignores filtered canary instances when orphaned", func() {
				upgraderBuilder.MaxInFlight = 3
				upgraderBuilder.Canaries = 1
				upgradeTool := upgrader.New(&upgraderBuilder)

				states := []*testState{
					{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
					{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OrphanDeployment}, lastOperationOutput: []brokerapi.LastOperationState{}, taskID: 2},
					{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
					{instance: service.Instance{GUID: "4"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 4},
				}
				setupTest(states, instanceLister, brokerServicesClient)

				filtered := []service.Instance{states[1].instance, states[2].instance}
				instanceLister.FilteredInstancesReturns(filtered, nil)

				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					upgradeErr = upgradeTool.Upgrade()
				}()

				expectToHaveNotStarted(states[0].controller, states[3].controller)
				expectToHaveStarted(states[1].controller, states[2].controller)

				allowToProceed(states[2].controller)

				expectToHaveStarted(states[0].controller, states[3].controller)
				allowToProceed(states[0].controller, states[3].controller)

				wg.Wait()

				Expect(upgradeErr).NotTo(HaveOccurred())

				hasReportedCanariesStarting(fakeListener, upgraderBuilder.Canaries, upgraderBuilder.CanarySelectionParams)
				hasReportedCanariesFinished(fakeListener, 1)
				hasReportedFinished(fakeListener, 1, 3, 0, []string{}, []string{})
			})

			It("skips canary instances if the are all orphaned or deleted", func() {
				upgraderBuilder.MaxInFlight = 3
				upgraderBuilder.Canaries = 1
				upgradeTool := upgrader.New(&upgraderBuilder)

				states := []*testState{
					{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
					{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OrphanDeployment}, lastOperationOutput: []brokerapi.LastOperationState{}, taskID: 2},
					{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.InstanceNotFound}, lastOperationOutput: []brokerapi.LastOperationState{}, taskID: 3},
					{instance: service.Instance{GUID: "4"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 4},
				}
				setupTest(states, instanceLister, brokerServicesClient)

				filtered := []service.Instance{states[1].instance, states[2].instance}
				instanceLister.FilteredInstancesReturns(filtered, nil)

				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					upgradeErr = upgradeTool.Upgrade()
				}()

				expectToHaveStarted(states[0].controller, states[1].controller, states[2].controller, states[3].controller)
				allowToProceed(states[0].controller, states[3].controller)

				wg.Wait()

				Expect(upgradeErr).NotTo(HaveOccurred())

				hasReportedCanariesStarting(fakeListener, upgraderBuilder.Canaries, upgraderBuilder.CanarySelectionParams)
				hasReportedCanariesFinished(fakeListener, 1)
				hasReportedFinished(fakeListener, 1, 2, 1, []string{}, []string{})
			})

			It("updates all the filtered canaries if the required number of canaries is higher than the size of filter canaries", func() {
				upgraderBuilder.MaxInFlight = 3
				upgraderBuilder.Canaries = 2
				upgradeTool := upgrader.New(&upgraderBuilder)

				states := []*testState{
					{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
					{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
					{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
					{instance: service.Instance{GUID: "4"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 4},
				}
				setupTest(states, instanceLister, brokerServicesClient)

				filtered := []service.Instance{states[1].instance}
				instanceLister.FilteredInstancesReturns(filtered, nil)

				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					upgradeErr = upgradeTool.Upgrade()
				}()

				expectToHaveNotStarted(states[0].controller, states[2].controller, states[3].controller)
				expectToHaveStarted(states[1].controller)

				allowToProceed(states[1].controller)
				expectToHaveStarted(states[0].controller, states[2].controller, states[3].controller)

				allowToProceed(states[0].controller, states[2].controller, states[3].controller)

				wg.Wait()

				Expect(upgradeErr).NotTo(HaveOccurred())

				hasReportedCanariesStarting(fakeListener, 1, upgraderBuilder.CanarySelectionParams)
				hasReportedCanariesFinished(fakeListener, 1)
				hasReportedFinished(fakeListener, 0, 4, 0, []string{}, []string{})
			})

			It("fails to upgrade when there are no filtered instances but other instances exist", func() {
				upgraderBuilder.MaxInFlight = 3
				upgraderBuilder.Canaries = 1
				upgradeTool := upgrader.New(&upgraderBuilder)

				states := []*testState{
					{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
					{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OrphanDeployment}, lastOperationOutput: []brokerapi.LastOperationState{}, taskID: 2},
					{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.InstanceNotFound}, lastOperationOutput: []brokerapi.LastOperationState{}, taskID: 3},
					{instance: service.Instance{GUID: "4"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 4},
				}
				setupTest(states, instanceLister, brokerServicesClient)

				instanceLister.FilteredInstancesReturns([]service.Instance{}, nil)

				upgradeErr = upgradeTool.Upgrade()
				Expect(upgradeErr).To(HaveOccurred())
				Expect(upgradeErr).To(MatchError(ContainSubstring("Failed to find a match to the canary selection criteria: ")))
				Expect(upgradeErr).To(MatchError(ContainSubstring("org: the-org")))
				Expect(upgradeErr).To(MatchError(ContainSubstring("space: the-space")))
				Expect(upgradeErr).To(MatchError(ContainSubstring("Please ensure these selection criteria will match one or more service instances, " +
					"or remove `canary_selection_params` to disable selecting canaries from a specific org and space.")))
			})

			It("does not fail when there are no filtered instances but no other instances exist", func() {
				upgraderBuilder.MaxInFlight = 3
				upgraderBuilder.Canaries = 1
				upgradeTool := upgrader.New(&upgraderBuilder)

				setupTest([]*testState{}, instanceLister, brokerServicesClient)
				instanceLister.FilteredInstancesReturns([]service.Instance{}, nil)

				upgradeErr = upgradeTool.Upgrade()
				Expect(upgradeErr).ToNot(HaveOccurred())
				hasReportedFinished(fakeListener, 0, 0, 0, []string{}, []string{})
			})

			It("upgrades all the instances matching the criteria when canaries number is not specified", func() {
				states := []*testState{
					{instance: service.Instance{GUID: "1"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 1},
					{instance: service.Instance{GUID: "2"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 2},
					{instance: service.Instance{GUID: "3"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 3},
					{instance: service.Instance{GUID: "4"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 4},
					{instance: service.Instance{GUID: "5"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 5},
					{instance: service.Instance{GUID: "6"}, upgradeOutput: []services.BOSHOperationType{services.OperationAccepted}, lastOperationOutput: []brokerapi.LastOperationState{brokerapi.Succeeded}, taskID: 6},
				}
				setupTest(states, instanceLister, brokerServicesClient)

				filtered := []service.Instance{states[1].instance, states[2].instance, states[4].instance, states[5].instance}
				instanceLister.FilteredInstancesReturns(filtered, nil)
				upgraderBuilder.MaxInFlight = 3
				upgraderBuilder.Canaries = 0

				upgradeTool := upgrader.New(&upgraderBuilder)

				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					upgradeErr = upgradeTool.Upgrade()
				}()

				expectToHaveStarted(states[1].controller, states[2].controller, states[4].controller)
				expectToHaveNotStarted(states[0].controller, states[3].controller, states[5].controller)

				allowToProceed(states[1].controller, states[2].controller, states[4].controller)
				expectToHaveStarted(states[5].controller)
				expectToHaveNotStarted(states[0].controller, states[3].controller)

				allowToProceed(states[5].controller)
				expectToHaveStarted(states[0].controller, states[3].controller)

				allowToProceed(states[0].controller, states[3].controller)

				wg.Wait()

				Expect(upgradeErr).NotTo(HaveOccurred())

				hasReportedCanariesStarting(fakeListener, 4, upgraderBuilder.CanarySelectionParams)
				hasReportedCanariesFinished(fakeListener, 1)
				hasReportedFinished(fakeListener, 0, 6, 0, []string{}, []string{})
			})
		})
	})
})
