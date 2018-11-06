// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrader

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"strings"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

//go:generate counterfeiter -o fakes/fake_listener.go . Listener
type Listener interface {
	FailedToRefreshInstanceInfo(instance string)
	Starting(maxInFlight int)
	RetryAttempt(num, limit int)
	RetryCanariesAttempt(num, limit, remainingCanaries int)
	InstancesToProcess(instances []service.Instance)
	InstanceOperationStarting(instance string, index int, totalInstances int, isCanary bool)
	InstanceOperationStartResult(instance string, status services.BOSHOperationType)
	InstanceOperationFinished(instance string, result string)
	WaitingFor(instance string, boshTaskId int)
	Progress(pollingInterval time.Duration, orphanCount, upgradedCount, upgradesLeftCount, deletedCount int)
	Finished(orphanCount, upgradedCount, deletedCount int, busyInstances, failedInstances []string)
	CanariesStarting(canaries int, filter config.CanarySelectionParams)
	CanariesFinished()
}

//go:generate counterfeiter -o fakes/fake_broker_services.go . BrokerServices
type BrokerServices interface {
	UpgradeInstance(instance service.Instance) (services.BOSHOperation, error)
	LastOperation(instance string, operationData broker.OperationData) (brokerapi.LastOperation, error)
}

//go:generate counterfeiter -o fakes/fake_instance_lister.go . InstanceLister
type InstanceLister interface {
	Instances() ([]service.Instance, error)
	FilteredInstances(filter map[string]string) ([]service.Instance, error)
	LatestInstanceInfo(inst service.Instance) (service.Instance, error)
}

//go:generate counterfeiter -o fakes/fake_sleeper.go . sleeper
type sleeper interface {
	Sleep(d time.Duration)
}

type instanceFailure struct {
	guid string
	err  error
}

type Triggerer interface {
	TriggerUpgrade(service.Instance) (services.BOSHOperation, error)
}

type StateChecker interface {
	Check(string, broker.OperationData) (services.BOSHOperation, error)
}

type Upgrader struct {
	brokerServices  BrokerServices
	instanceLister  InstanceLister
	pollingInterval time.Duration
	attemptInterval time.Duration
	attemptLimit    int
	maxInFlight     int
	listener        Listener
	sleeper         sleeper

	failures              []instanceFailure
	canaries              int
	canarySelectionParams config.CanarySelectionParams
	upgradeState          *upgradeState
	triggerer             Triggerer
	stateChecker          StateChecker
}

func New(builder *Builder) *Upgrader {
	return &Upgrader{
		brokerServices:        builder.BrokerServices,
		instanceLister:        builder.ServiceInstanceLister,
		pollingInterval:       builder.PollingInterval,
		attemptInterval:       builder.AttemptInterval,
		attemptLimit:          builder.AttemptLimit,
		maxInFlight:           builder.MaxInFlight,
		listener:              builder.Listener,
		sleeper:               builder.Sleeper,
		canaries:              builder.Canaries,
		canarySelectionParams: builder.CanarySelectionParams,
		triggerer:             NewTriggerer(builder.BrokerServices, builder.ServiceInstanceLister, builder.Listener),
		stateChecker:          NewStateChecker(builder.BrokerServices),
	}
}

func (u *Upgrader) Upgrade() error {
	u.listener.Starting(u.maxInFlight)

	if err := u.registerInstancesAndCanaries(); err != nil {
		return err
	}

	u.listener.InstancesToProcess(u.upgradeState.AllInstances())

	if u.upgradeState.IsProcessingCanaries() {
		u.listener.CanariesStarting(u.upgradeState.OutstandingCanaryCount(), u.canarySelectionParams)
		if err := u.UpgradeInstancesWithAttempts(); err != nil {
			u.printSummary()
			return err
		}
		u.upgradeState.MarkCanariesCompleted()
		u.listener.CanariesFinished()
	}

	if err := u.UpgradeInstancesWithAttempts(); err != nil {
		u.printSummary()
		return err
	}
	u.printSummary()
	return nil
}

func (u *Upgrader) UpgradeInstancesWithAttempts() error {
	for attempt := 1; attempt <= u.attemptLimit; attempt++ {
		u.upgradeState.RewindAndResetBusyInstances()
		u.logRetryAttempt(attempt)

		for u.upgradeState.HasInstancesToProcess() {
			if !u.upgradeState.HasFailures() {
				u.triggerUpgrades()
			}
			u.pollRunningTasks()

			if u.upgradeState.HasInstancesProcessing() {
				u.sleeper.Sleep(u.pollingInterval)
				continue
			}

			if u.upgradeState.HasFailures() {
				return u.formatError()
			}

			if u.upgradeState.IsProcessingCanaries() && u.upgradeState.CurrentPhaseIsComplete() {
				return nil
			}
		}

		u.reportProgress()

		if u.upgradeState.CurrentPhaseIsComplete() {
			break
		}

		u.sleeper.Sleep(u.attemptInterval)
	}
	return u.checkStillBusyInstances()
}

func (u *Upgrader) registerInstancesAndCanaries() error {
	var canaryInstances []service.Instance

	allInstances, err := u.instanceLister.Instances()
	if err != nil {
		return fmt.Errorf("error listing service instances: %s", err)
	}

	if len(u.canarySelectionParams) > 0 {
		canaryInstances, err = u.instanceLister.FilteredInstances(u.canarySelectionParams)
		if err != nil {
			return fmt.Errorf("error listing service instances: %s", err)
		}
		if len(canaryInstances) == 0 && len(allInstances) > 0 {
			return fmt.Errorf("Failed to find a match to the canary selection criteria: %s. "+
				"Please ensure these selection criteria will match one or more service instances, "+
				"or remove `canary_selection_params` to disable selecting canaries from a specific org and space.", u.canarySelectionParams)
		}
		if len(canaryInstances) < u.canaries {
			u.canaries = len(canaryInstances)
		}
	} else {
		if u.canaries > 0 {
			canaryInstances = allInstances
		} else {
			canaryInstances = []service.Instance{}
		}
	}
	u.upgradeState, err = NewUpgradeState(canaryInstances, allInstances, u.canaries)
	if err != nil {
		return fmt.Errorf("error with canary instance listing: %s", err)
	}
	return nil
}

func (u *Upgrader) logRetryAttempt(attempt int) {
	if u.upgradeState.IsProcessingCanaries() {
		u.listener.RetryCanariesAttempt(attempt, u.attemptLimit, u.upgradeState.OutstandingCanaryCount())
	} else {
		u.listener.RetryAttempt(attempt, u.attemptLimit)
	}
}

func (u *Upgrader) upgradesToTriggerCount() int {
	inProg := u.upgradeState.CountInProgressInstances()
	needed := u.maxInFlight - inProg
	if u.upgradeState.IsProcessingCanaries() {
		outstandingCanaries := u.upgradeState.OutstandingCanaryCount()
		if needed > outstandingCanaries {
			needed = outstandingCanaries
		}
	}
	return needed
}

func (u *Upgrader) triggerUpgrades() {
	needed := u.upgradesToTriggerCount()
	if needed == 0 {
		return
	}

	totalInstances := u.upgradeState.CountInstancesInCurrentPhase()

	acceptedCount := 0
	for acceptedCount < needed {
		instance, err := u.upgradeState.NextPending()
		if err != nil {
			break
		}
		u.listener.InstanceOperationStarting(instance.GUID, u.upgradeState.GetUpgradeIndex(), totalInstances, u.upgradeState.IsProcessingCanaries())
		operation, err := u.triggerer.TriggerUpgrade(instance)
		if err != nil {
			u.upgradeState.SetState(instance.GUID, services.OperationFailed)
			u.failures = append(u.failures, instanceFailure{guid: instance.GUID, err: err})
			return
		}
		u.upgradeState.SetUpgradeOperation(instance.GUID, operation)
		u.upgradeState.SetState(instance.GUID, operation.Type)
		u.listener.InstanceOperationStartResult(instance.GUID, operation.Type)

		if operation.Type == services.OperationAccepted {
			u.listener.WaitingFor(instance.GUID, operation.Data.BoshTaskID)
			acceptedCount++
		}
	}
}

func (u *Upgrader) pollRunningTasks() {
	for _, inst := range u.upgradeState.InProgressInstances() {
		guid := inst.GUID
		state, err := u.stateChecker.Check(guid, u.upgradeState.GetUpgradeOperation(guid).Data)
		if err != nil {
			u.upgradeState.SetState(guid, services.OperationFailed)
			u.failures = append(u.failures, instanceFailure{guid: guid, err: err})
			continue
		}
		u.upgradeState.SetState(guid, state.Type)

		switch state.Type {
		case services.OperationSucceeded:
			u.listener.InstanceOperationFinished(guid, "success")
		case services.OperationFailed:
			u.listener.InstanceOperationFinished(guid, "failure")
			upgradeErr := fmt.Errorf("[%s] Operation failed: bosh task id %d: %s", guid, state.Data.BoshTaskID, state.Description)
			u.failures = append(u.failures, instanceFailure{guid: guid, err: upgradeErr})
		}
	}
}

func (u *Upgrader) reportProgress() {
	summary := u.upgradeState.Summary()
	u.listener.Progress(u.attemptInterval, summary.orphaned, summary.succeeded, summary.busy, summary.deleted)
}

func (u *Upgrader) printSummary() {
	summary := u.upgradeState.Summary()

	busyInstances := u.upgradeState.GetGUIDsInStates(services.OperationInProgress)
	failedList := u.failures
	var failedInstances []string
	for _, failure := range failedList {
		failedInstances = append(failedInstances, failure.guid)
	}

	u.listener.Finished(summary.orphaned, summary.succeeded, summary.deleted, busyInstances, failedInstances)
}

func (u *Upgrader) checkStillBusyInstances() error {
	busyInstances := u.upgradeState.GetGUIDsInStates(services.OperationInProgress)
	busyInstancesCount := len(busyInstances)

	if busyInstancesCount == 0 {
		return nil
	}

	if u.upgradeState.IsProcessingCanaries() {
		if !u.upgradeState.canariesCompleted() {
			return fmt.Errorf(
				"canaries didn't upgrade successfully: attempted to upgrade %d canaries, but only found %d instances not already in use by another BOSH task.",
				u.canaries,
				u.canaries-busyInstancesCount,
			)
		}
		return nil
	}
	return fmt.Errorf("The following instances could not be processed: %s", strings.Join(busyInstances, ", "))
}

func (u *Upgrader) formatError() error {
	err := u.errorFromList()
	if u.upgradeState.IsProcessingCanaries() {
		return errors.Wrap(err, "canaries didn't upgrade successfully")
	}
	return err
}

func (u *Upgrader) errorFromList() error {
	failureList := u.failures
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
