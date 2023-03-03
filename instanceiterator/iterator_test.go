// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package instanceiterator_test

import (
	"errors"
	"fmt"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"time"

	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator"
	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

var _ = Describe("Iterator", func() {

	var (
		emptyBusyList            = []string{}
		emptyFailedList          = []string{}
		fakeListener             *fakes.FakeListener
		fakeBrokerServicesClient *fakes.FakeBrokerServices
		fakeSleeper              *fakes.FakeSleeper
		fakeTriggerer            *fakes.FakeTriggerer

		builder instanceiterator.Configurator

		iteratorError error
	)

	BeforeEach(func() {
		fakeListener = new(fakes.FakeListener)
		fakeBrokerServicesClient = new(fakes.FakeBrokerServices)
		fakeSleeper = new(fakes.FakeSleeper)

		fakeTriggerer = new(fakes.FakeTriggerer)

		builder = instanceiterator.Configurator{
			BrokerServices:  fakeBrokerServicesClient,
			Listener:        fakeListener,
			PollingInterval: 10 * time.Second,
			AttemptLimit:    5,
			AttemptInterval: 60 * time.Second,
			MaxInFlight:     1,
			Canaries:        0,
			Sleeper:         fakeSleeper,
			Triggerer:       fakeTriggerer,
		}
	})

	Context("requests error", func() {
		It("fails when cannot get the list of all the instances", func() {
			fakeBrokerServicesClient.InstancesReturns([]service.Instance{}, errors.New("oops"))
			u := instanceiterator.New(&builder)
			err := u.Iterate()
			Expect(err).To(MatchError(ContainSubstring("oops")))
		})

		It("fails when cannot start upgrading an instance", func() {
			fakeBrokerServicesClient.InstancesReturns([]service.Instance{{GUID: "1"}}, nil)
			fakeBrokerServicesClient.LatestInstanceInfoStub = func(inst service.Instance) (service.Instance, error) {
				return inst, nil
			}
			fakeTriggerer.TriggerOperationReturns(instanceiterator.TriggeredOperation{}, errors.New("oops"))

			u := instanceiterator.New(&builder)
			err := u.Iterate()
			Expect(err).To(MatchError(ContainSubstring("oops")))
		})

		It("fails when cannot poll last operation", func() {
			fakeBrokerServicesClient.InstancesReturns([]service.Instance{{GUID: "1"}}, nil)

			fakeTriggerer.TriggerOperationReturns(instanceiterator.TriggeredOperation{State: instanceiterator.OperationAccepted}, nil)
			fakeTriggerer.CheckReturns(instanceiterator.TriggeredOperation{State: instanceiterator.OperationFailed}, errors.New("oops"))

			u := instanceiterator.New(&builder)
			err := u.Iterate()
			Expect(err).To(MatchError(ContainSubstring("oops")))
		})
	})

	Context("plan change in-flight", func() {
		It("uses the new plan for an upgrade", func() {
			fakeBrokerServicesClient.InstancesReturnsOnCall(0, []service.Instance{{GUID: "1", PlanUniqueID: "plan-id-1"}}, nil)
			fakeBrokerServicesClient.LatestInstanceInfoReturnsOnCall(0, service.Instance{GUID: "1", PlanUniqueID: "plan-id-2"}, nil)

			fakeTriggerer.TriggerOperationReturns(instanceiterator.TriggeredOperation{State: instanceiterator.OperationAccepted}, nil)
			fakeTriggerer.CheckReturns(instanceiterator.TriggeredOperation{State: instanceiterator.OperationSucceeded}, nil)

			iterator := instanceiterator.New(&builder)
			err := iterator.Iterate()
			Expect(err).NotTo(HaveOccurred())

			instance := fakeTriggerer.TriggerOperationArgsForCall(0)
			Expect(instance.PlanUniqueID).To(Equal("plan-id-2"))
		})

		It("continues the operation using the previously fetched info if latest instance info call errors", func() {
			instances := []service.Instance{{GUID: "1", PlanUniqueID: "plan-id-1"}}
			fakeBrokerServicesClient.InstancesReturnsOnCall(0, instances, nil)
			fakeBrokerServicesClient.LatestInstanceInfoReturnsOnCall(0, service.Instance{}, errors.New("unexpected error"))

			fakeTriggerer.TriggerOperationReturns(instanceiterator.TriggeredOperation{State: instanceiterator.OperationAccepted}, nil)
			fakeTriggerer.CheckReturns(instanceiterator.TriggeredOperation{State: instanceiterator.OperationSucceeded}, nil)

			iterator := instanceiterator.New(&builder)
			err := iterator.Iterate()
			Expect(err).NotTo(HaveOccurred())

			actualInstance := fakeTriggerer.TriggerOperationArgsForCall(0)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualInstance).To(Equal(instances[0]))
			Expect(fakeListener.FailedToRefreshInstanceInfoCallCount()).To(Equal(1))
		})

		It("marks an instance-not-found on refresh as deleted", func() {
			fakeBrokerServicesClient.InstancesReturnsOnCall(0, []service.Instance{{GUID: "1", PlanUniqueID: "plan-id-1"}}, nil)
			fakeBrokerServicesClient.LatestInstanceInfoReturnsOnCall(0, service.Instance{}, services.InstanceNotFoundError)

			iterator := instanceiterator.New(&builder)
			err := iterator.Iterate()
			Expect(err).NotTo(HaveOccurred())

			hasReportedFinished(fakeListener, 0, 0, 1, emptyBusyList, emptyFailedList)
			hasReportedInstanceOperationStartResult(fakeListener, 0, "1", instanceiterator.InstanceNotFound)
		})
	})

	Context("processes instances without canaries", func() {
		AfterEach(func() {
			hasReportedStarting(fakeListener, builder.MaxInFlight)
			Expect(fakeListener.CanariesStartingCallCount()).To(Equal(0), "Expected canaries starting to not be called")
			Expect(fakeListener.CanariesFinishedCallCount()).To(Equal(0), "Expected canaries finished to not be called")
		})

		It("succeeds", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
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

			Expect(iteratorError).NotTo(HaveOccurred())

			hasReportedFinished(fakeListener, 0, 3, 0, []string{}, []string{})
		})

		It("does not fail and reports a deleted instance", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.InstanceNotFound}, checkStatusOutput: []instanceiterator.OperationState{}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
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

			Expect(iteratorError).NotTo(HaveOccurred())

			hasReportedFinished(fakeListener, 0, 2, 1, []string{}, []string{})
		})

		It("does not fail and reports an orphaned instance", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OrphanDeployment}, checkStatusOutput: []instanceiterator.OperationState{}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
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

			Expect(iteratorError).NotTo(HaveOccurred())

			hasReportedFinished(fakeListener, 1, 2, 0, []string{}, []string{})
		})

		It("polls last_operation endpoint when process is not synchronous", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted, instanceiterator.OperationAccepted, instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
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

			Expect(iteratorError).NotTo(HaveOccurred())

			hasSlept(fakeSleeper, 0, builder.PollingInterval)
			hasSlept(fakeSleeper, 1, builder.PollingInterval)
			hasReportedFinished(fakeListener, 0, 3, 0, []string{}, []string{})
		})

		It("retries busy instances until the upgrade request is accepted", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationInProgress, instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
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

			Expect(iteratorError).NotTo(HaveOccurred())

			hasReportedRetries(fakeListener, 1, 1, 0)
			hasReportedAttempts(fakeListener, 2, 3, 5)
			hasReportedFinished(fakeListener, 0, 3, 0, []string{}, []string{})
		})

		It("fails when retrying busy instances reach the attempt limit", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationInProgress, instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.AttemptLimit = 1
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)

			allowToProceed(states[0].controller)
			expectToHaveStarted(states[1].controller)

			expectToHaveStarted(states[2].controller)
			allowToProceed(states[2].controller)

			wg.Wait()

			Expect(iteratorError).To(MatchError(ContainSubstring("The following instances could not be processed:")))

			hasReportedRetries(fakeListener, 1)
			hasReportedAttempts(fakeListener, 0, 1, 1)
			hasReportedFinished(fakeListener, 0, 2, 0, []string{states[1].instance.GUID}, []string{})
		})

		It("returns an error when an last operation returns a failure", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationFailed}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.AttemptLimit = 1
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)

			allowToProceed(states[0].controller)
			expectToHaveStarted(states[1].controller)

			allowToProceed(states[1].controller)

			wg.Wait()

			Expect(iteratorError).To(MatchError(ContainSubstring(fmt.Sprintf("[%s] Operation failed: bosh task id %d", states[1].instance.GUID, states[1].taskID))))

			hasReportedFinished(fakeListener, 0, 1, 0, []string{}, []string{states[1].instance.GUID})
		})

		It("retries until a deleted instance is detected", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationInProgress, instanceiterator.InstanceNotFound}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
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

			Expect(iteratorError).NotTo(HaveOccurred())

			hasReportedRetries(fakeListener, 1, 1, 0)
			hasReportedAttempts(fakeListener, 2, 3, 5)
			hasReportedFinished(fakeListener, 0, 2, 1, []string{}, []string{})
		})

		It("retries until an orphaned instance is detected", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationInProgress, instanceiterator.OrphanDeployment}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)

			allowToProceed(states[0].controller)
			expectToHaveStarted(states[1].controller)

			expectToHaveStarted(states[2].controller)
			allowToProceed(states[2].controller)

			expectToHaveStarted(states[1].controller)

			wg.Wait()

			Expect(iteratorError).NotTo(HaveOccurred())

			hasReportedRetries(fakeListener, 1, 1, 0)
			hasReportedAttempts(fakeListener, 2, 3, 5)
			hasReportedFinished(fakeListener, 1, 2, 0, []string{}, []string{})
		})

		It("processes in batches when max_in_flight is greater than 1", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
				{instance: service.Instance{GUID: "4"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 4},
				{instance: service.Instance{GUID: "5"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 5},
				{instance: service.Instance{GUID: "6"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 6},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.MaxInFlight = 4
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller, states[2].controller, states[3].controller)
			expectToHaveNotStarted(states[4].controller, states[5].controller)

			allowToProceed(states[0].controller, states[1].controller, states[2].controller, states[3].controller)
			expectToHaveStarted(states[4].controller, states[5].controller)

			allowToProceed(states[4].controller, states[5].controller)

			wg.Wait()

			Expect(iteratorError).NotTo(HaveOccurred())

			hasReportedFinished(fakeListener, 0, 6, 0, []string{}, []string{})
		})

		It("returns multiple errors if multiple instances fail to upgrade", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationFailed}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationFailed}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.MaxInFlight = 2
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller)
			expectToHaveNotStarted(states[2].controller)
			allowToProceed(states[0].controller, states[1].controller)

			wg.Wait()

			Expect(iteratorError).To(HaveOccurred())
			Expect(iteratorError.Error()).To(SatisfyAll(
				ContainSubstring("2 errors occurred"),
				ContainSubstring(fmt.Sprintf("[%s] Operation failed: bosh task id %d: ", states[0].instance.GUID, states[0].taskID)),
				ContainSubstring(fmt.Sprintf("[%s] Operation failed: bosh task id %d: ", states[1].instance.GUID, states[1].taskID)),
			))
			hasReportedOperationState(fakeListener, 0, states[0].instance.GUID, "failure")
			hasReportedOperationState(fakeListener, 1, states[1].instance.GUID, "failure")
			hasReportedFinished(fakeListener, 0, 0, 0, []string{}, []string{states[0].instance.GUID, states[1].instance.GUID})
		})
	})

	Context("upgrade instances with canaries", func() {
		AfterEach(func() {
			hasReportedStarting(fakeListener, builder.MaxInFlight)
		})

		It("succeeds upgrading first a canary instance", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
				{instance: service.Instance{GUID: "4"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 4},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.Canaries = 2
			builder.MaxInFlight = 3
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			// upgrade canary instances
			expectToHaveStarted(states[0].controller, states[1].controller)
			expectToHaveNotStarted(states[2].controller, states[3].controller)
			allowToProceed(states[0].controller, states[1].controller)

			// upgrade the rest
			expectToHaveStarted(states[2].controller, states[3].controller)
			allowToProceed(states[2].controller, states[3].controller)

			wg.Wait()

			Expect(iteratorError).NotTo(HaveOccurred())

			hasReportedCanariesStarting(fakeListener, builder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 1)
			hasReportedFinished(fakeListener, 0, 4, 0, []string{}, []string{})
		})

		It("succeeds upgrading using max_in_flight as batch size if it is smaller than the number of required canaries", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
				{instance: service.Instance{GUID: "4"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 4},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.Canaries = 4
			builder.MaxInFlight = 3
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller, states[2].controller)
			expectToHaveNotStarted(states[3].controller)
			allowToProceed(states[0].controller, states[1].controller, states[2].controller)

			expectToHaveStarted(states[3].controller)
			allowToProceed(states[3].controller)

			wg.Wait()

			Expect(iteratorError).NotTo(HaveOccurred())

			hasReportedCanariesStarting(fakeListener, builder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 1)
			hasReportedFinished(fakeListener, 0, 4, 0, []string{}, []string{})
		})

		It("does not fail if there are no instances", func() {
			setupTest([]*testState{}, fakeBrokerServicesClient, fakeTriggerer)

			builder.Canaries = 2
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			wg.Wait()

			Expect(iteratorError).NotTo(HaveOccurred())
			hasReportedFinished(fakeListener, 0, 0, 0, []string{}, []string{})
		})

		It("stops upgrading if a canary instance fails to upgrade", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationFailed}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.Canaries = 1
			builder.MaxInFlight = 3
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			expectToHaveStarted(states[0].controller)
			expectToHaveNotStarted(states[1].controller, states[2].controller)
			allowToProceed(states[0].controller)

			expectToHaveNotStarted(states[1].controller, states[2].controller)
			wg.Wait()

			Expect(iteratorError).To(MatchError(ContainSubstring("canaries didn't process successfully")))

			hasReportedCanariesStarting(fakeListener, builder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 0)
			hasReportedFinished(fakeListener, 0, 0, 0, []string{}, []string{states[0].instance.GUID})
		})

		It("picks another canary instance if one is busy", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.Canaries = 1
			builder.MaxInFlight = 3
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller, states[2].controller)
			allowToProceed(states[2].controller)

			allowToProceed(states[0].controller, states[1].controller)

			wg.Wait()

			Expect(iteratorError).NotTo(HaveOccurred())

			hasReportedCanariesStarting(fakeListener, builder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 1)
			hasReportedFinished(fakeListener, 0, 3, 0, []string{}, []string{})
		})

		It("picks another canary instance if one is deleted", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.InstanceNotFound}, checkStatusOutput: []instanceiterator.OperationState{}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.Canaries = 1
			builder.MaxInFlight = 3
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller)
			expectToHaveNotStarted(states[2].controller)
			allowToProceed(states[1].controller)

			allowToProceed(states[2].controller)

			wg.Wait()

			Expect(iteratorError).NotTo(HaveOccurred())

			hasReportedCanariesStarting(fakeListener, builder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 1)
			hasReportedFinished(fakeListener, 0, 2, 1, []string{}, []string{})
		})

		It("picks another canary instance if one is orphaned", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OrphanDeployment}, checkStatusOutput: []instanceiterator.OperationState{}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.Canaries = 1
			builder.MaxInFlight = 3
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller)
			expectToHaveNotStarted(states[2].controller)
			allowToProceed(states[1].controller)

			allowToProceed(states[2].controller)

			wg.Wait()

			Expect(iteratorError).NotTo(HaveOccurred())

			hasReportedCanariesStarting(fakeListener, builder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 1)
			hasReportedFinished(fakeListener, 1, 2, 0, []string{}, []string{})
		})

		It("retries busy canaries if needed", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.Canaries = 3
			builder.MaxInFlight = 3
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller, states[2].controller)
			allowToProceed(states[0].controller, states[1].controller, states[2].controller)

			wg.Wait()

			Expect(iteratorError).NotTo(HaveOccurred())

			hasReportedCanariesStarting(fakeListener, builder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 1)
			hasReportedFinished(fakeListener, 0, 3, 0, []string{}, []string{})
		})

		It("fails when reaching the attempt limit retrying canaries", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress}, checkStatusOutput: []instanceiterator.OperationState{}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress}, checkStatusOutput: []instanceiterator.OperationState{}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.Canaries = 3
			builder.MaxInFlight = 3
			builder.AttemptLimit = 1
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			expectToHaveStarted(states[0].controller, states[1].controller, states[2].controller)
			allowToProceed(states[0].controller, states[1].controller, states[2].controller)

			wg.Wait()

			Expect(iteratorError).To(MatchError(ContainSubstring(
				"canaries didn't process successfully: attempted to process 3 canaries, but only found 1 instances not already in use by another BOSH task.",
			)))

			hasReportedFinished(fakeListener, 0, 1, 0, []string{states[1].instance.GUID, states[2].instance.GUID}, []string{})
		})

		It("retries busy instances after all canaries have passed", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationInProgress, instanceiterator.OperationInProgress, instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.Canaries = 1
			builder.MaxInFlight = 3
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
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

			Expect(iteratorError).NotTo(HaveOccurred())

			hasReportedCanariesStarting(fakeListener, builder.Canaries, nil)
			hasReportedCanariesFinished(fakeListener, 1)
			hasReportedAttempts(fakeListener, 3, 4, 5)
			hasReportedFinished(fakeListener, 0, 3, 0, []string{}, []string{})
		})

		It("reports count status accurately when retrying in canaries and rest", func() {
			states := []*testState{
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
				{instance: service.Instance{GUID: "4"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationInProgress, instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 4},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.Canaries = 2
			builder.MaxInFlight = 3
			builder.AttemptLimit = 2
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			// Retry attempt 1: Canaries
			expectToHaveStarted(states[0].controller, states[1].controller, states[2].controller, states[3].controller)
			allowToProceed(states[0].controller)

			// Retry attempt 2: Canaries
			expectToHaveStarted(states[1].controller)
			expectToHaveNotStarted(states[2].controller, states[3].controller)
			allowToProceed(states[1].controller)
			// Canaries completed

			// Retry attempt 1: Iterate
			expectToHaveStarted(states[2].controller, states[3].controller)
			allowToProceed(states[2].controller)

			// Retry attemp 2 : Iterate
			expectToHaveStarted(states[3].controller)
			allowToProceed(states[3].controller)

			wg.Wait()

			Expect(iteratorError).NotTo(HaveOccurred())

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
				{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
				{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.InstanceNotFound}, checkStatusOutput: []instanceiterator.OperationState{}, taskID: 2},
				{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OrphanDeployment}, checkStatusOutput: []instanceiterator.OperationState{}, taskID: 3},
				{instance: service.Instance{GUID: "4"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationInProgress, instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 4},
			}
			setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

			builder.Canaries = 1
			builder.MaxInFlight = 3
			iterator := instanceiterator.New(&builder)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				iteratorError = iterator.Iterate()
			}()

			// Logs for canaries
			{
				expectToHaveStarted(states[0].controller)

				hasReportedStarting(fakeListener, 3)
				hasReportedInstancesToProcess(fakeListener, states[0].instance, states[1].instance, states[2].instance, states[3].instance)

				hasReportedCanariesStarting(fakeListener, 1, nil)
				hasReportedCanaryAttempts(fakeListener, 1, 5, 1)
				hasReportedInstanceOperationStarted(fakeListener, 0, "1", 1, 4, true)
				hasReportedInstanceOperationStartResult(fakeListener, 0, "1", instanceiterator.OperationAccepted)
				hasReportedWaitingFor(fakeListener, 0, "1", 1)
				allowToProceed(states[0].controller)

				// Retry attempt 1: Iterate
				expectToHaveStarted(states[1].controller, states[2].controller, states[3].controller)
				hasReportedOperationState(fakeListener, 0, "1", "success")
				hasReportedCanariesFinished(fakeListener, 1)
			}

			// Logs for upgrade attempt 1
			{
				hasReportedAttempts(fakeListener, 0, 1, 5)
				hasReportedInstanceOperationStarted(fakeListener, 1, "2", 2, 4, false)
				hasReportedInstanceOperationStartResult(fakeListener, 1, "2", instanceiterator.InstanceNotFound)

				hasReportedInstanceOperationStarted(fakeListener, 2, "3", 3, 4, false)
				hasReportedInstanceOperationStartResult(fakeListener, 2, "3", instanceiterator.OrphanDeployment)

				hasReportedInstanceOperationStarted(fakeListener, 3, "4", 4, 4, false)
				hasReportedInstanceOperationStartResult(fakeListener, 3, "4", instanceiterator.OperationInProgress)

				hasReportedProgress(fakeListener, 0, builder.AttemptInterval, 1, 1, 1, 1)

				// Retry attempt 2: Iterate
				expectToHaveStarted(states[3].controller)
				allowToProceed(states[3].controller)
			}

			wg.Wait()
			Expect(iteratorError).NotTo(HaveOccurred())

			// Logs for upgrade attempt 2
			{
				hasReportedAttempts(fakeListener, 1, 2, 5)
				hasReportedInstanceOperationStarted(fakeListener, 4, "4", 4, 4, false)
				hasReportedInstanceOperationStartResult(fakeListener, 4, "4", instanceiterator.OperationAccepted)
				hasReportedWaitingFor(fakeListener, 1, "4", 4)
				hasReportedOperationState(fakeListener, 1, "4", "success")
				hasReportedProgress(fakeListener, 1, builder.AttemptInterval, 1, 2, 0, 1)
				hasReportedFinished(fakeListener, 1, 2, 1, []string{}, []string{})
			}
		})

		When("canary selection params is specified", func() {
			BeforeEach(func() {
				builder.CanarySelectionParams = config.CanarySelectionParams{
					"org":   "the-org",
					"space": "the-space",
				}
			})

			AfterEach(func() {
				Expect(fakeBrokerServicesClient.InstancesCallCount()).To(Equal(2))
				params := fakeBrokerServicesClient.InstancesArgsForCall(1)
				Expect(params["org"]).To(Equal("the-org"))
				Expect(params["space"]).To(Equal("the-space"))

				params = fakeBrokerServicesClient.InstancesArgsForCall(0)
				Expect(params).To(BeEmpty())
			})

			It("uses canaries matching the selection criteria", func() {
				builder.MaxInFlight = 2
				builder.Canaries = 3
				iterator := instanceiterator.New(&builder)

				states := []*testState{
					{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
					{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
					{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
					{instance: service.Instance{GUID: "4"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 4},
					{instance: service.Instance{GUID: "5"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 5},
					{instance: service.Instance{GUID: "6"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 6},
				}
				setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

				filtered := []service.Instance{states[1].instance, states[2].instance, states[4].instance, states[5].instance}
				fakeBrokerServicesClient.InstancesStub = InstanceStub(states, filtered)

				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					iteratorError = iterator.Iterate()
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

				Expect(iteratorError).NotTo(HaveOccurred())

				hasReportedCanariesStarting(fakeListener, builder.Canaries, builder.CanarySelectionParams)
				hasReportedCanariesFinished(fakeListener, 1)
				hasReportedFinished(fakeListener, 0, 6, 0, []string{}, []string{})
			})

			It("ignores filtered canary instances when orphaned", func() {
				builder.MaxInFlight = 3
				builder.Canaries = 1
				iterator := instanceiterator.New(&builder)

				states := []*testState{
					{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
					{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OrphanDeployment}, checkStatusOutput: []instanceiterator.OperationState{}, taskID: 2},
					{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
					{instance: service.Instance{GUID: "4"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 4},
				}
				setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

				filtered := []service.Instance{states[1].instance, states[2].instance}
				fakeBrokerServicesClient.InstancesStub = InstanceStub(states, filtered)

				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					iteratorError = iterator.Iterate()
				}()

				expectToHaveNotStarted(states[0].controller, states[3].controller)
				expectToHaveStarted(states[1].controller, states[2].controller)

				allowToProceed(states[2].controller)

				expectToHaveStarted(states[0].controller, states[3].controller)
				allowToProceed(states[0].controller, states[3].controller)

				wg.Wait()

				Expect(iteratorError).NotTo(HaveOccurred())

				hasReportedCanariesStarting(fakeListener, builder.Canaries, builder.CanarySelectionParams)
				hasReportedCanariesFinished(fakeListener, 1)
				hasReportedFinished(fakeListener, 1, 3, 0, []string{}, []string{})
			})

			It("skips canary instances if the are all orphaned or deleted", func() {
				builder.MaxInFlight = 3
				builder.Canaries = 1
				iterator := instanceiterator.New(&builder)

				states := []*testState{
					{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
					{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OrphanDeployment}, checkStatusOutput: []instanceiterator.OperationState{}, taskID: 2},
					{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.InstanceNotFound}, checkStatusOutput: []instanceiterator.OperationState{}, taskID: 3},
					{instance: service.Instance{GUID: "4"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 4},
				}
				setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

				filtered := []service.Instance{states[1].instance, states[2].instance}
				fakeBrokerServicesClient.InstancesStub = InstanceStub(states, filtered)

				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					iteratorError = iterator.Iterate()
				}()

				expectToHaveStarted(states[0].controller, states[1].controller, states[2].controller, states[3].controller)
				allowToProceed(states[0].controller, states[3].controller)

				wg.Wait()

				Expect(iteratorError).NotTo(HaveOccurred())

				hasReportedCanariesStarting(fakeListener, builder.Canaries, builder.CanarySelectionParams)
				hasReportedCanariesFinished(fakeListener, 1)
				hasReportedFinished(fakeListener, 1, 2, 1, []string{}, []string{})
			})

			It("updates all the filtered canaries if the required number of canaries is higher than the size of filter canaries", func() {
				builder.MaxInFlight = 3
				builder.Canaries = 2
				iterator := instanceiterator.New(&builder)

				states := []*testState{
					{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
					{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
					{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
					{instance: service.Instance{GUID: "4"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 4},
				}
				setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

				filtered := []service.Instance{states[1].instance}
				fakeBrokerServicesClient.InstancesStub = InstanceStub(states, filtered)

				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					iteratorError = iterator.Iterate()
				}()

				expectToHaveNotStarted(states[0].controller, states[2].controller, states[3].controller)
				expectToHaveStarted(states[1].controller)

				allowToProceed(states[1].controller)
				expectToHaveStarted(states[0].controller, states[2].controller, states[3].controller)

				allowToProceed(states[0].controller, states[2].controller, states[3].controller)

				wg.Wait()

				Expect(iteratorError).NotTo(HaveOccurred())

				hasReportedCanariesStarting(fakeListener, 1, builder.CanarySelectionParams)
				hasReportedCanariesFinished(fakeListener, 1)
				hasReportedFinished(fakeListener, 0, 4, 0, []string{}, []string{})
			})

			It("fails to upgrade when there are no filtered instances but other instances exist", func() {
				builder.MaxInFlight = 3
				builder.Canaries = 1
				iterator := instanceiterator.New(&builder)

				states := []*testState{
					{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
					{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OrphanDeployment}, checkStatusOutput: []instanceiterator.OperationState{}, taskID: 2},
					{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.InstanceNotFound}, checkStatusOutput: []instanceiterator.OperationState{}, taskID: 3},
					{instance: service.Instance{GUID: "4"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 4},
				}
				setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

				fakeBrokerServicesClient.InstancesStub = InstanceStub(states, []service.Instance{})

				iteratorError = iterator.Iterate()
				Expect(iteratorError).To(HaveOccurred())
				Expect(iteratorError).To(MatchError(ContainSubstring("Failed to find a match to the canary selection criteria: ")))
				Expect(iteratorError).To(MatchError(ContainSubstring("org: the-org")))
				Expect(iteratorError).To(MatchError(ContainSubstring("space: the-space")))
				Expect(iteratorError).To(MatchError(ContainSubstring("Please ensure these selection criteria will match one or more service instances, " +
					"or remove `canary_selection_params` to disable selecting canaries from a specific org and space.")))
			})

			It("does not fail when there are no filtered instances but no other instances exist", func() {
				builder.MaxInFlight = 3
				builder.Canaries = 1
				iterator := instanceiterator.New(&builder)

				setupTest([]*testState{}, fakeBrokerServicesClient, fakeTriggerer)
				fakeBrokerServicesClient.InstancesReturns([]service.Instance{}, nil)

				iteratorError = iterator.Iterate()
				Expect(iteratorError).ToNot(HaveOccurred())
				hasReportedFinished(fakeListener, 0, 0, 0, []string{}, []string{})
			})

			It("processes all the instances matching the criteria when canaries number is not specified", func() {
				states := []*testState{
					{instance: service.Instance{GUID: "1"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 1},
					{instance: service.Instance{GUID: "2"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 2},
					{instance: service.Instance{GUID: "3"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 3},
					{instance: service.Instance{GUID: "4"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 4},
					{instance: service.Instance{GUID: "5"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 5},
					{instance: service.Instance{GUID: "6"}, triggerOutput: []instanceiterator.OperationState{instanceiterator.OperationAccepted}, checkStatusOutput: []instanceiterator.OperationState{instanceiterator.OperationSucceeded}, taskID: 6},
				}
				setupTest(states, fakeBrokerServicesClient, fakeTriggerer)

				filtered := []service.Instance{states[1].instance, states[2].instance, states[4].instance, states[5].instance}
				fakeBrokerServicesClient.InstancesStub = InstanceStub(states, filtered)
				builder.MaxInFlight = 3
				builder.Canaries = 0

				iterator := instanceiterator.New(&builder)

				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					iteratorError = iterator.Iterate()
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

				Expect(iteratorError).NotTo(HaveOccurred())

				hasReportedCanariesStarting(fakeListener, 4, builder.CanarySelectionParams)
				hasReportedCanariesFinished(fakeListener, 1)
				hasReportedFinished(fakeListener, 0, 6, 0, []string{}, []string{})
			})
		})
	})
})

func InstanceStub(states []*testState, filtered []service.Instance) func(filter map[string]string) ([]service.Instance, error) {
	return func(filter map[string]string) ([]service.Instance, error) {
		if filter == nil {
			var instances []service.Instance
			for _, s := range states {
				instances = append(instances, s.instance)
			}
			return instances, nil
		}
		return filtered, nil
	}
}
