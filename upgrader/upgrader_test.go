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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader/fakes"
)

var _ = Describe("Upgrader", func() {
	const (
		serviceInstanceId = "serviceInstanceId"
	)

	var (
		actualErr            error
		fakeListener         *fakes.FakeListener
		brokerServicesClient *fakes.FakeBrokerServices
		instanceLister       *fakes.FakeInstanceLister
		pollingInterval      time.Duration
		attemptInterval      time.Duration
		attemptLimit         int
		upgraderBuilder      upgrader.Builder
		fakeSleeper          *fakes.FakeSleeper

		upgradeOperationAccepted = services.UpgradeOperation{
			Type: services.UpgradeAccepted,
		}
		lastOperationSucceeded  = brokerapi.LastOperation{State: brokerapi.Succeeded}
		lastOperationInProgress = brokerapi.LastOperation{State: brokerapi.InProgress}
	)

	BeforeEach(func() {
		fakeListener = new(fakes.FakeListener)
		brokerServicesClient = new(fakes.FakeBrokerServices)
		instanceLister = new(fakes.FakeInstanceLister)
		pollingInterval = 10 * time.Second
		attemptInterval = 60 * time.Second
		attemptLimit = 5
		fakeSleeper = new(fakes.FakeSleeper)
	})

	JustBeforeEach(func() {
		upgraderBuilder = upgrader.Builder{
			BrokerServices:        brokerServicesClient,
			ServiceInstanceLister: instanceLister,
			Listener:              fakeListener,
			PollingInterval:       pollingInterval,
			AttemptLimit:          attemptLimit,
			AttemptInterval:       attemptInterval,
			Sleeper:               fakeSleeper,
		}
		upgrader := upgrader.New(&upgraderBuilder)
		actualErr = upgrader.Upgrade()
	})

	Context("when upgrading one instance", func() {
		Context("and is successful", func() {
			BeforeEach(func() {
				instanceLister.InstancesReturns([]service.Instance{{GUID: serviceInstanceId}}, nil)
				brokerServicesClient.UpgradeInstanceReturns(upgradeOperationAccepted, nil)

				brokerServicesClient.LastOperationReturns(
					brokerapi.LastOperation{
						State:       brokerapi.Succeeded,
						Description: "foo",
					}, nil)
			})

			It("returns the list of successful upgrades", func() {
				hasReportedInstanceUpgradeStarted(fakeListener, serviceInstanceId, 0, 1)
				hasReportedInstanceUpgradeStartResult(fakeListener, services.UpgradeAccepted)
				hasReportedUpgraded(fakeListener, serviceInstanceId)
				Expect(actualErr).NotTo(HaveOccurred())
			})
		})

		Context("and it fails", func() {
			Context("to get a list of service instances", func() {
				BeforeEach(func() {
					instanceLister.InstancesReturns(nil, errors.New("bad status code"))
				})

				It("returns an error", func() {
					Expect(actualErr).To(MatchError("error listing service instances: bad status code"))
				})
			})

			Context("due to a malformed service instance guid", func() {
				BeforeEach(func() {
					instanceLister.InstancesReturns([]service.Instance{{GUID: "not a guid Q#$%#$%^&&*$%^#$FGRTYW${T:WED:AWSD)E@#PE{:QS:{QLWD"}}, nil)
					brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{}, errors.New("failed"))
				})

				It("returns an error", func() {
					Expect(actualErr).To(MatchError("Upgrade failed for service instance not a guid Q#$%#$%^&&*$%^#$FGRTYW${T:WED:AWSD)E@#PE{:QS:{QLWD: failed\n"))
				})
			})
		})
	})

	Context("when upgrading an instance is not instant", func() {
		BeforeEach(func() {
			instanceLister.InstancesReturns([]service.Instance{{GUID: serviceInstanceId}}, nil)

			brokerServicesClient.UpgradeInstanceReturns(upgradeOperationAccepted, nil)

			brokerServicesClient.LastOperationReturns(lastOperationInProgress, nil)
			brokerServicesClient.LastOperationReturnsOnCall(2, brokerapi.LastOperation{
				State: brokerapi.Succeeded,
			}, nil)
		})

		It("polls last operation until successful", func() {
			Expect(brokerServicesClient.LastOperationCallCount()).To(Equal(3))
			Expect(actualErr).NotTo(HaveOccurred())
			hasReportedUpgraded(fakeListener, serviceInstanceId)
		})
	})

	Context("when the CF service instance has been deleted", func() {
		BeforeEach(func() {
			instanceLister.InstancesReturns([]service.Instance{{GUID: serviceInstanceId}}, nil)
			brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
				Type: services.InstanceNotFound,
			}, nil)
		})

		It("ignores the deleted instance instance", func() {
			Expect(actualErr).NotTo(HaveOccurred())

			hasReportedInstanceUpgradeStartResult(fakeListener, services.InstanceNotFound)
			hasReportedProgress(fakeListener, 0, attemptInterval, 0, 0, 0, 1)
			hasReportedFinished(fakeListener, 0, 0, 1, 0)
		})
	})

	Context("when the bosh deployment cannot be found", func() {
		BeforeEach(func() {
			instanceLister.InstancesReturns([]service.Instance{{GUID: serviceInstanceId}}, nil)
			brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
				Type: services.OrphanDeployment,
			}, nil)
		})

		It("detects one orphan instance", func() {
			Expect(actualErr).NotTo(HaveOccurred())

			hasReportedInstanceUpgradeStartResult(fakeListener, services.OrphanDeployment)
			hasReportedProgress(fakeListener, 0, attemptInterval, 1, 0, 0, 0)
			hasReportedFinished(fakeListener, 1, 0, 0, 0)
		})
	})

	Context("when an operation is in progress for a service instance", func() {
		Context("when the number of retries is within the limit", func() {
			const serviceInstanceId = "serviceInstanceId"
			BeforeEach(func() {
				instanceLister.InstancesReturns([]service.Instance{{GUID: serviceInstanceId}}, nil)

				brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
					Type: services.OperationInProgress,
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(3, upgradeOperationAccepted, nil)

				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
			})

			It("retries until the upgrade request is accepted", func() {
				Expect(actualErr).NotTo(HaveOccurred())

				Expect(brokerServicesClient.UpgradeInstanceCallCount()).To(Equal(4), "number of service requests")
				hasReportedInstanceUpgradeStartResult(
					fakeListener,
					services.OperationInProgress,
					services.OperationInProgress,
					services.OperationInProgress,
					services.UpgradeAccepted,
				)
				hasReportedRetries(fakeListener, 1, 1, 1, 0)
				hasReportedFinished(fakeListener, 0, 1, 0, 0)
				hasReportedAttempts(fakeListener, 4, 5)
			})
		})
		Context("when the attemptLimit is reached", func() {
			const serviceInstanceId = "serviceInstanceId"
			BeforeEach(func() {
				attemptLimit = 2
				instanceLister.InstancesReturns([]service.Instance{{GUID: serviceInstanceId}}, nil)

				brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
					Type: services.OperationInProgress,
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(3, upgradeOperationAccepted, nil)

				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
			})

			It("stops retrying when the attemptLimit is reached", func() {
				Expect(actualErr).To(MatchError(fmt.Errorf("The following instances could not be upgraded: service-instance_%s", serviceInstanceId)))

				Expect(brokerServicesClient.UpgradeInstanceCallCount()).To(Equal(2), "number of service requests")
				hasReportedInstanceUpgradeStartResult(
					fakeListener,
					services.OperationInProgress,
					services.OperationInProgress,
				)
				hasReportedRetries(fakeListener, 1, 1)
				hasReportedFinished(fakeListener, 0, 0, 0, 1)
			})
		})
	})

	Context("when deletion is in progress for a service instance", func() {
		const serviceInstanceId = "serviceInstanceId"
		BeforeEach(func() {
			instanceLister.InstancesReturns([]service.Instance{{GUID: serviceInstanceId}}, nil)

			brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
				Type: services.OperationInProgress,
			}, nil)
			brokerServicesClient.UpgradeInstanceReturnsOnCall(3, services.UpgradeOperation{
				Type: services.OrphanDeployment,
			}, nil)

			brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
		})

		It("retries until an orphan is detected", func() {
			Expect(actualErr).NotTo(HaveOccurred())
			Expect(brokerServicesClient.UpgradeInstanceCallCount()).To(Equal(4), "number of service requests")

			hasReportedRetries(fakeListener, 1, 1, 1, 0)
			hasReportedOrphans(fakeListener, 0, 0, 0, 1)
			hasReportedFinished(fakeListener, 1, 0, 0, 0)
		})
	})

	Context("when upgrading multiple instances", func() {
		Context("successfully", func() {
			serviceInstance1 := "serviceInstanceId1"
			serviceInstance2 := "serviceInstanceId2"
			serviceInstance3 := "serviceInstanceId3"
			upgradeTaskID1 := 123
			upgradeTaskID2 := 456
			upgradeTaskID3 := 789

			BeforeEach(func() {
				instanceLister.InstancesReturns([]service.Instance{
					{GUID: serviceInstance1},
					{GUID: serviceInstance2},
					{GUID: serviceInstance3},
				}, nil)

				brokerServicesClient.UpgradeInstanceReturnsOnCall(0, services.UpgradeOperation{
					Type: services.UpgradeAccepted,
					Data: upgradeResponse(upgradeTaskID1),
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(1, services.UpgradeOperation{
					Type: services.UpgradeAccepted,
					Data: upgradeResponse(upgradeTaskID2),
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(2, services.UpgradeOperation{
					Type: services.UpgradeAccepted,
					Data: upgradeResponse(upgradeTaskID3),
				}, nil)

				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
			})

			It("returns a report with all instances upgraded", func() {
				Expect(actualErr).NotTo(HaveOccurred())

				hasReportedStarting(fakeListener)
				hasReportedInstancesToUpgrade(fakeListener, serviceInstance1, serviceInstance2, serviceInstance3)
				hasReportedWaitingFor(fakeListener, map[string]int{serviceInstance1: upgradeTaskID1, serviceInstance2: upgradeTaskID2, serviceInstance3: upgradeTaskID3})
				hasReportedUpgraded(fakeListener, serviceInstance1, serviceInstance2, serviceInstance3)
				hasReportedProgress(fakeListener, 0, attemptInterval, 0, 3, 0, 0)
				hasReportedFinished(fakeListener, 0, 3, 0, 0)
				hasSlept(fakeSleeper, 0, pollingInterval)
				hasSlept(fakeSleeper, 1, pollingInterval)
				hasSlept(fakeSleeper, 2, pollingInterval)
			})
		})

		Context("and the second upgrade request fails", func() {
			serviceInstance1 := "serviceInstanceId1"
			serviceInstance2 := "serviceInstanceId2"
			serviceInstance3 := "serviceInstanceId3"

			BeforeEach(func() {
				instanceLister.InstancesReturns([]service.Instance{
					{GUID: serviceInstance1},
					{GUID: serviceInstance2},
					{GUID: serviceInstance3},
				}, nil)

				brokerServicesClient.UpgradeInstanceReturnsOnCall(0, upgradeOperationAccepted, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(1, services.UpgradeOperation{}, errors.New("upgrade failed"))

				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
			})

			It("returns the upgrade request error", func() {
				message := fmt.Sprintf(
					"Upgrade failed for service instance %s: upgrade failed\n",
					serviceInstance2,
				)
				Expect(actualErr).To(MatchError(message))
				Expect(fakeSleeper.SleepCallCount()).To(Equal(1))
			})
		})

		Context("and the second upgrade operation fails", func() {
			serviceInstance1 := "serviceInstanceId1"
			serviceInstance2 := "serviceInstanceId2"
			serviceInstance3 := "serviceInstanceId3"
			upgradeTaskID1 := 432
			upgradeTaskID2 := 987

			BeforeEach(func() {
				instanceLister.InstancesReturns([]service.Instance{
					{GUID: serviceInstance1},
					{GUID: serviceInstance2},
					{GUID: serviceInstance3},
				}, nil)

				brokerServicesClient.UpgradeInstanceReturnsOnCall(0, services.UpgradeOperation{
					Type: services.UpgradeAccepted,
					Data: upgradeResponse(upgradeTaskID1),
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(1, services.UpgradeOperation{
					Type: services.UpgradeAccepted,
					Data: upgradeResponse(upgradeTaskID2),
				}, nil)

				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
				brokerServicesClient.LastOperationReturnsOnCall(1, brokerapi.LastOperation{
					State:       brokerapi.Failed,
					Description: "everything went wrong",
				}, nil)
			})

			It("reports the upgrade operation error", func() {
				failureMessage := fmt.Sprintf(
					"Upgrade failed for service instance %s: bosh task id %d: everything went wrong",
					serviceInstance2,
					upgradeTaskID2,
				)
				Expect(actualErr).To(MatchError(failureMessage))

				hasReportedWaitingFor(fakeListener, map[string]int{serviceInstance1: upgradeTaskID1, serviceInstance2: upgradeTaskID2})
				hasReportedFailureFor(fakeListener, serviceInstance2)
				Expect(fakeSleeper.SleepCallCount()).To(Equal(2))
			})
		})

		Context("and the second instance is orphaned", func() {
			serviceInstance1 := "serviceInstanceId1"
			serviceInstance2 := "serviceInstanceId2"
			serviceInstance3 := "serviceInstanceId3"

			BeforeEach(func() {
				instanceLister.InstancesReturns([]service.Instance{
					{GUID: serviceInstance1},
					{GUID: serviceInstance2},
					{GUID: serviceInstance3},
				}, nil)

				brokerServicesClient.UpgradeInstanceReturnsOnCall(0, upgradeOperationAccepted, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(1, services.UpgradeOperation{
					Type: services.OrphanDeployment,
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(2, upgradeOperationAccepted, nil)
				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
			})

			It("reports one orphaned instance", func() {
				Expect(actualErr).NotTo(HaveOccurred())
				hasReportedFinished(fakeListener, 1, 2, 0, 0)
				Expect(fakeSleeper.SleepCallCount()).To(Equal(2))
			})
		})

		Context("and one has an operation in progress", func() {
			serviceInstance1 := "serviceInstanceId1"
			serviceInstance2 := "serviceInstanceId2"
			serviceInstance3 := "serviceInstanceId3"

			BeforeEach(func() {
				instanceLister.InstancesReturns([]service.Instance{
					{GUID: serviceInstance1},
					{GUID: serviceInstance2},
					{GUID: serviceInstance3},
				}, nil)

				brokerServicesClient.UpgradeInstanceReturnsOnCall(0, upgradeOperationAccepted, nil)
				brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
					Type: services.OperationInProgress,
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(2, upgradeOperationAccepted, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(5, upgradeOperationAccepted, nil)

				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
			})

			It("retries until all are upgraded", func() {
				Expect(actualErr).NotTo(HaveOccurred())

				upgradeServiceInstance2CallCount := 0
				for x := 0; x < brokerServicesClient.UpgradeInstanceCallCount(); x++ {
					instance := brokerServicesClient.UpgradeInstanceArgsForCall(x)
					if instance.GUID == serviceInstance2 {
						upgradeServiceInstance2CallCount++
					}
				}

				Expect(upgradeServiceInstance2CallCount).To(Equal(4), "number of service requests")
				hasReportedRetries(fakeListener, 1, 1, 1, 0)
				hasReportedFinished(fakeListener, 0, 3, 0, 0)
				hasReportedProgress(fakeListener, 0, attemptInterval, 0, 2, 1, 0)

				hasSlept(fakeSleeper, 0, pollingInterval)
				hasSlept(fakeSleeper, 1, pollingInterval)
				hasSlept(fakeSleeper, 2, attemptInterval)
			})
		})
	})
})

func upgradeResponse(taskId int) broker.OperationData {
	return broker.OperationData{BoshTaskID: taskId, OperationType: broker.OperationTypeUpgrade}
}

func hasReportedStarting(fakeListener *fakes.FakeListener) {
	Expect(fakeListener.StartingCallCount()).To(Equal(1))
}

func hasReportedInstancesToUpgrade(fakeListener *fakes.FakeListener, instanceIds ...string) {
	Expect(fakeListener.InstancesToUpgradeCallCount()).To(Equal(1))
	Expect(fakeListener.InstancesToUpgradeArgsForCall(0)).To(Equal(makeInstanceMapFromIds(instanceIds)))
}

func hasReportedWaitingFor(fakeListener *fakes.FakeListener, instances map[string]int) {
	calls := fakeListener.WaitingForCallCount()
	Expect(calls).To(Equal(len(instances)))
	for i := 0; i < calls; i++ {
		instanceId, taskId := fakeListener.WaitingForArgsForCall(i)
		Expect(instances[instanceId]).To(Equal(taskId), "Task Id for "+instanceId)
	}
}

func hasReportedInstanceUpgradeStarted(fakeListener *fakes.FakeListener, expectedInstance string, expectedIndex, expectedTotalInstances int) {
	Expect(fakeListener.InstanceUpgradeStartingCallCount()).To(
		Equal(1), "instance upgrade started call count",
	)

	actualInstance,
		actualIndex,
		actualTotalInstances := fakeListener.InstanceUpgradeStartingArgsForCall(0)
	Expect(actualInstance).To(Equal(expectedInstance))
	Expect(actualIndex).To(Equal(expectedIndex), "expected index for instance upgrade started")
	Expect(actualTotalInstances).To(Equal(expectedTotalInstances), "expected total num of instances for instance upgrade started")
}

func hasReportedInstanceUpgradeStartResult(fakeListener *fakes.FakeListener, expectedStatuses ...services.UpgradeOperationType) {
	Expect(fakeListener.InstanceUpgradeStartResultCallCount()).To(
		Equal(len(expectedStatuses)), "instance upgrade start result call count",
	)

	for i, expectedStatus := range expectedStatuses {
		Expect(fakeListener.InstanceUpgradeStartResultArgsForCall(i)).To(
			Equal(expectedStatus),
		)
	}
}

func hasReportedUpgraded(fakeListener *fakes.FakeListener, expectedInstanceIds ...string) {
	hasReportedUpgradeStates(fakeListener, "success", expectedInstanceIds...)
}

func hasReportedFailureFor(fakeListener *fakes.FakeListener, expectedInstanceIds ...string) {
	hasReportedUpgradeStates(fakeListener, "failure", expectedInstanceIds...)
}

func hasReportedUpgradeStates(fakeListener *fakes.FakeListener, expectedStatus string, expectedInstanceIds ...string) {
	upgraded := make([]service.Instance, 0)
	for i := 0; i < fakeListener.InstanceUpgradedCallCount(); i++ {
		id, status := fakeListener.InstanceUpgradedArgsForCall(i)
		if status == expectedStatus {
			upgraded = append(upgraded, service.Instance{GUID: id})
		}
	}

	expectedInstances := makeInstanceMapFromIds(expectedInstanceIds)

	Expect(upgraded).To(Equal(expectedInstances), "status="+expectedStatus)
}

func makeInstanceMapFromIds(expectedInstanceIds []string) []service.Instance {
	var expectedInstances []service.Instance
	for _, expectedInstanceId := range expectedInstanceIds {
		expectedInstances = append(expectedInstances, service.Instance{GUID: expectedInstanceId})
	}
	return expectedInstances
}

func hasSlept(fakeSleeper *fakes.FakeSleeper, callIndex int, expectedInterval time.Duration) {
	Expect(fakeSleeper.SleepCallCount()).To(BeNumerically(">", callIndex))
	Expect(fakeSleeper.SleepArgsForCall(callIndex)).To(Equal(expectedInterval))
}

func hasReportedRetries(fakeListener *fakes.FakeListener, expectedRetryCounts ...int) {
	for i, expectedRetryCount := range expectedRetryCounts {
		_, _, _, toRetryCount, _ := fakeListener.ProgressArgsForCall(i)
		Expect(toRetryCount).To(Equal(expectedRetryCount), "Retry count: "+string(i))
	}
}

func hasReportedOrphans(fakeListener *fakes.FakeListener, expectedOrphanCounts ...int) {
	for i, expectedOrphanCount := range expectedOrphanCounts {
		_, orphanCount, _, _, _ := fakeListener.ProgressArgsForCall(i)
		Expect(orphanCount).To(Equal(expectedOrphanCount), "Orphan count: "+string(i))
	}
}

func hasReportedProgress(fakeListener *fakes.FakeListener, callIndex int, expectedInterval time.Duration, expectedOrphans, expectedUpgraded, expectedToRetry, expectedDeleted int) {
	Expect(fakeListener.ProgressCallCount()).To(BeNumerically(">", callIndex))
	attemptInterval, orphanCount, upgradedCount, toRetryCount, deletedCount := fakeListener.ProgressArgsForCall(callIndex)
	Expect(attemptInterval).To(Equal(expectedInterval), "attempt interval")
	Expect(orphanCount).To(Equal(expectedOrphans), "orphans")
	Expect(upgradedCount).To(Equal(expectedUpgraded), "upgraded")
	Expect(toRetryCount).To(Equal(expectedToRetry), "to retry")
	Expect(deletedCount).To(Equal(expectedDeleted), "deleted")
}

func hasReportedFinished(fakeListener *fakes.FakeListener, expectedOrphans, expectedUpgraded, expectedDeleted, expectedCouldNotStart int) {
	Expect(fakeListener.FinishedCallCount()).To(Equal(1))
	orphanCount, upgradedCount, deletedCount, couldNotStartCount := fakeListener.FinishedArgsForCall(0)
	Expect(orphanCount).To(Equal(expectedOrphans), "orphans")
	Expect(upgradedCount).To(Equal(expectedUpgraded), "upgraded")
	Expect(deletedCount).To(Equal(expectedDeleted), "deleted")
	Expect(couldNotStartCount).To(Equal(expectedCouldNotStart), "couldNotStart")
}

func hasReportedAttempts(fakeListener *fakes.FakeListener, count, limit int) {
	Expect(fakeListener.RetryAttemptCallCount()).To(Equal(count))
	for i := 0; i < count; i++ {
		c, l := fakeListener.RetryAttemptArgsForCall(i)
		Expect(c).To(Equal(i + 1))
		Expect(l).To(Equal(limit))
	}
}
