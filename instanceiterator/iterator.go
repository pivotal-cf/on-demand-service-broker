// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package instanceiterator

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"strings"

	"github.com/pivotal-cf/brokerapi/v7/domain"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_listener.go . Listener
type Listener interface {
	FailedToRefreshInstanceInfo(instance string)
	Starting(maxInFlight int)
	RetryAttempt(num, limit int)
	RetryCanariesAttempt(num, limit, remainingCanaries int)
	InstancesToProcess(instances []service.Instance)
	InstanceOperationStarting(instance string, index int, totalInstances int, isCanary bool)
	InstanceOperationStartResult(instance string, status OperationState)
	InstanceOperationFinished(instance string, result string)
	WaitingFor(instance string, boshTaskId int)
	Progress(pollingInterval time.Duration, orphanCount, processedCount, skippedCount, toRetryCount, deletedCount int)
	Finished(orphanCount, finishedCount, skippedCount, deletedCount int, busyInstances, failedInstances []string)
	CanariesStarting(canaries int, filter config.CanarySelectionParams)
	CanariesFinished()
	UpgradeStrategy(strategy string)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_broker_services.go . BrokerServices
type BrokerServices interface {
	ProcessInstance(instance service.Instance, operationType string) (services.BOSHOperation, error)
	LastOperation(instance string, operationData broker.OperationData) (domain.LastOperation, error)
	Instances(filter map[string]string) ([]service.Instance, error)
	LatestInstanceInfo(inst service.Instance) (service.Instance, error)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_sleeper.go . sleeper
type sleeper interface {
	Sleep(d time.Duration)
}

type instanceFailure struct {
	guid string
	err  error
}

type TriggeredOperation struct {
	State       OperationState
	Data        broker.OperationData
	Description string
}

type OperationState string

const (
	OperationAccepted   OperationState = "accepted"
	OperationSucceeded  OperationState = "succeeded"
	OperationSkipped    OperationState = "skipped"
	OperationFailed     OperationState = "failed"
	OperationInProgress OperationState = "busy"
	InstanceNotFound    OperationState = "instance-not-found"
	OperationPending    OperationState = "not-started"
	OrphanDeployment    OperationState = "orphan-deployment"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_triggerer.go . Triggerer
type Triggerer interface {
	TriggerOperation(service.Instance) (TriggeredOperation, error)
	Check(string, broker.OperationData) (TriggeredOperation, error)
}

type Iterator struct {
	brokerServices  BrokerServices
	pollingInterval time.Duration
	attemptInterval time.Duration
	attemptLimit    int
	maxInFlight     int
	listener        Listener
	sleeper         sleeper

	failures              []instanceFailure
	canaries              int
	canarySelectionParams config.CanarySelectionParams
	iteratorState         *iteratorState
	triggerer             Triggerer
}

func New(builder *Configurator) *Iterator {
	return &Iterator{
		brokerServices:        builder.BrokerServices,
		pollingInterval:       builder.PollingInterval,
		attemptInterval:       builder.AttemptInterval,
		attemptLimit:          builder.AttemptLimit,
		maxInFlight:           builder.MaxInFlight,
		listener:              builder.Listener,
		sleeper:               builder.Sleeper,
		canaries:              builder.Canaries,
		canarySelectionParams: builder.CanarySelectionParams,
		triggerer:             builder.Triggerer,
	}
}

func (it *Iterator) Iterate() error {
	it.listener.Starting(it.maxInFlight)

	if err := it.registerInstancesAndCanaries(); err != nil {
		return err
	}

	it.listener.InstancesToProcess(it.iteratorState.AllInstances())

	if it.iteratorState.IsProcessingCanaries() {
		it.listener.CanariesStarting(it.iteratorState.OutstandingCanaryCount(), it.canarySelectionParams)
		if err := it.IterateInstancesWithAttempts(); err != nil {
			it.printSummary()
			return err
		}
		it.iteratorState.MarkCanariesCompleted()
		it.listener.CanariesFinished()
	}

	if err := it.IterateInstancesWithAttempts(); err != nil {
		it.printSummary()
		return err
	}
	it.printSummary()
	return nil
}

func (it *Iterator) IterateInstancesWithAttempts() error {
	for attempt := 1; attempt <= it.attemptLimit; attempt++ {
		it.iteratorState.RewindAndResetBusyInstances()
		it.logRetryAttempt(attempt)

		for it.iteratorState.HasInstancesToProcess() {
			if !it.iteratorState.HasFailures() {
				it.triggerOperation()
			}
			it.pollRunningTasks()

			if it.iteratorState.HasInstancesProcessing() {
				it.sleeper.Sleep(it.pollingInterval)
				continue
			}

			if it.iteratorState.HasFailures() {
				return it.formatError()
			}

			if it.iteratorState.IsProcessingCanaries() && it.iteratorState.CurrentPhaseIsComplete() {
				return nil
			}
		}

		it.reportProgress()

		if it.iteratorState.CurrentPhaseIsComplete() {
			break
		}

		it.sleeper.Sleep(it.attemptInterval)
	}
	return it.checkStillBusyInstances()
}

func (it *Iterator) registerInstancesAndCanaries() error {
	var canaryInstances []service.Instance

	allInstances, err := it.brokerServices.Instances(nil)
	if err != nil {
		return fmt.Errorf("error listing service instances: %s", err)
	}

	if len(it.canarySelectionParams) > 0 {
		canaryInstances, err = it.brokerServices.Instances(it.canarySelectionParams)
		if err != nil {
			return fmt.Errorf("error listing service instances: %s", err)
		}
		if len(canaryInstances) == 0 && len(allInstances) > 0 {
			return fmt.Errorf("Failed to find a match to the canary selection criteria: %s. "+
				"Please ensure these selection criteria will match one or more service instances, "+
				"or remove `canary_selection_params` to disable selecting canaries from a specific org and space.", it.canarySelectionParams)
		}
		if len(canaryInstances) < it.canaries {
			it.canaries = len(canaryInstances)
		}
	} else {
		if it.canaries > 0 {
			canaryInstances = allInstances
		} else {
			canaryInstances = []service.Instance{}
		}
	}
	it.iteratorState, err = NewIteratorState(canaryInstances, allInstances, it.canaries)
	if err != nil {
		return fmt.Errorf("error with canary instance listing: %s", err)
	}
	return nil
}

func (it *Iterator) logRetryAttempt(attempt int) {
	if it.iteratorState.IsProcessingCanaries() {
		it.listener.RetryCanariesAttempt(attempt, it.attemptLimit, it.iteratorState.OutstandingCanaryCount())
	} else {
		it.listener.RetryAttempt(attempt, it.attemptLimit)
	}
}

func (it *Iterator) operationsToTriggerCount() int {
	inProg := it.iteratorState.CountInProgressInstances()
	needed := it.maxInFlight - inProg
	if it.iteratorState.IsProcessingCanaries() {
		outstandingCanaries := it.iteratorState.OutstandingCanaryCount()
		if needed > outstandingCanaries {
			needed = outstandingCanaries
		}
	}
	return needed
}

func (it *Iterator) triggerOperation() {
	needed := it.operationsToTriggerCount()
	if needed == 0 {
		return
	}

	totalInstances := it.iteratorState.CountInstancesInCurrentPhase()

	acceptedCount := 0
	for acceptedCount < needed {
		instance, err := it.iteratorState.NextPending()
		if err != nil {
			break
		}
		it.listener.InstanceOperationStarting(instance.GUID, it.iteratorState.GetIteratorIndex(), totalInstances, it.iteratorState.IsProcessingCanaries())

		var operation TriggeredOperation
		latestInstance, err := it.brokerServices.LatestInstanceInfo(instance)

		if err == services.InstanceNotFoundError {
			operation, err = TriggeredOperation{State: InstanceNotFound}, nil
		} else {
			if err != nil {
				it.listener.FailedToRefreshInstanceInfo(instance.GUID)
				latestInstance = instance
			}
			operation, err = it.triggerer.TriggerOperation(latestInstance)
		}

		if err != nil {
			it.iteratorState.SetState(instance.GUID, OperationFailed)
			it.failures = append(it.failures, instanceFailure{guid: instance.GUID, err: err})
			return
		}
		it.iteratorState.SetOperation(instance.GUID, operation)
		it.iteratorState.SetState(instance.GUID, operation.State)
		it.listener.InstanceOperationStartResult(instance.GUID, operation.State)

		if operation.State == OperationAccepted {
			it.listener.WaitingFor(instance.GUID, operation.Data.BoshTaskID)
			acceptedCount++
		}
	}
}

func (it *Iterator) pollRunningTasks() {
	for _, inst := range it.iteratorState.InProgressInstances() {
		guid := inst.GUID
		triggedOperation, err := it.triggerer.Check(guid, it.iteratorState.GetOperation(guid).Data)
		if err != nil {
			it.iteratorState.SetState(guid, OperationFailed)
			it.failures = append(it.failures, instanceFailure{guid: guid, err: err})
			continue
		}
		it.iteratorState.SetState(guid, triggedOperation.State)

		switch triggedOperation.State {
		case OperationSucceeded:
			it.listener.InstanceOperationFinished(guid, "success")
		case OperationFailed:
			it.listener.InstanceOperationFinished(guid, "failure")
			err := fmt.Errorf("[%s] Operation failed: bosh task id %d: %s", guid, triggedOperation.Data.BoshTaskID, triggedOperation.Description)
			it.failures = append(it.failures, instanceFailure{guid: guid, err: err})
		}
	}
}

func (it *Iterator) reportProgress() {
	summary := it.iteratorState.Summary()
	it.listener.Progress(it.attemptInterval, summary.orphaned, summary.succeeded, summary.skipped, summary.busy, summary.deleted)
}

func (it *Iterator) printSummary() {
	summary := it.iteratorState.Summary()

	busyInstances := it.iteratorState.GetGUIDsInStates(OperationInProgress)
	failedList := it.failures
	var failedInstances []string
	for _, failure := range failedList {
		failedInstances = append(failedInstances, failure.guid)
	}

	it.listener.Finished(summary.orphaned, summary.succeeded, summary.skipped, summary.deleted, busyInstances, failedInstances)
}

func (it *Iterator) checkStillBusyInstances() error {
	busyInstances := it.iteratorState.GetGUIDsInStates(OperationInProgress)
	busyInstancesCount := len(busyInstances)

	if busyInstancesCount == 0 {
		return nil
	}

	if it.iteratorState.IsProcessingCanaries() {
		if !it.iteratorState.canariesCompleted() {
			return fmt.Errorf(
				"canaries didn't process successfully: attempted to process %d canaries, but only found %d instances not already in use by another BOSH task.",
				it.canaries,
				it.canaries-busyInstancesCount,
			)
		}
		return nil
	}
	return fmt.Errorf("The following instances could not be processed: %s", strings.Join(busyInstances, ", "))
}

func (it *Iterator) formatError() error {
	err := it.errorFromList()
	if it.iteratorState.IsProcessingCanaries() {
		return errors.Wrap(err, "canaries didn't process successfully")
	}
	return err
}

func (it *Iterator) errorFromList() error {
	failureList := it.failures
	if len(failureList) == 1 {
		return failureList[0].err
	} else if len(failureList) > 1 {
		var out string
		out = fmt.Sprintf("%d errors occurred:\n", len(failureList))
		for _, e := range failureList {
			out += "\n* " + e.err.Error()
		}
		return fmt.Errorf(out)
	}
	return nil
}
