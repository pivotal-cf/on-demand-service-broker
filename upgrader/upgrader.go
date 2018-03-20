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
	InstancesToUpgrade(instances []service.Instance)
	InstanceUpgradeStarting(instance string, index int, totalInstances int, isCanary bool)
	InstanceUpgradeStartResult(instance string, status services.UpgradeOperationType)
	InstanceUpgraded(instance string, result string)
	WaitingFor(instance string, boshTaskId int)
	Progress(pollingInterval time.Duration, orphanCount, upgradedCount, upgradesLeftCount, deletedCount int)
	Finished(orphanCount, upgradedCount, deletedCount int, busyInstances, failedInstances []string)
	CanariesStarting(canaries int, filter config.CanarySelectionParams)
	CanariesFinished()
}

//go:generate counterfeiter -o fakes/fake_broker_services.go . BrokerServices
type BrokerServices interface {
	UpgradeInstance(instance service.Instance) (services.UpgradeOperation, error)
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

type controller struct {
	// pendingInstances []string
	failures []instanceFailure
	canaries int
	// outstandingCanaries   int
	canarySelectionParams config.CanarySelectionParams

	// states map[string]services.UpgradeOperation
	// plans  map[string]string

	upgradeState *upgradeState
}

type instanceFailure struct {
	guid string
	err  error
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

	controller *controller
}

func New(builder *Builder) *Upgrader {
	return &Upgrader{
		brokerServices:  builder.BrokerServices,
		instanceLister:  builder.ServiceInstanceLister,
		pollingInterval: builder.PollingInterval,
		attemptInterval: builder.AttemptInterval,
		attemptLimit:    builder.AttemptLimit,
		maxInFlight:     builder.MaxInFlight,
		listener:        builder.Listener,
		sleeper:         builder.Sleeper,
		controller: &controller{
			// states:                make(map[string]services.UpgradeOperation),
			// plans:                 make(map[string]string),
			canaries: builder.Canaries,
			// outstandingCanaries:   builder.Canaries,
			canarySelectionParams: builder.CanarySelectionParams,
		},
	}
}

func (u *Upgrader) UpgradeInstancesWithAttempts() error {
	for attempt := 1; attempt <= u.attemptLimit; attempt++ {
		u.controller.upgradeState.Retry()
		u.logRetryAttempt(attempt)

		for len(u.controller.upgradeState.GetInstancesInStates(services.UpgradePending, services.UpgradeAccepted)) > 0 {
			u.triggerUpgrades()

			u.pollRunningTasks()

			if len(u.controller.upgradeState.GetInstancesInStates(services.UpgradeAccepted)) > 0 {
				u.sleeper.Sleep(u.pollingInterval)
				continue
			}

			if len(u.controller.upgradeState.GetInstancesInStates(services.UpgradeFailed)) > 0 {
				return u.formatError()
			}

			if u.controller.upgradeState.IsProcessingCanaries() && u.controller.upgradeState.PhaseComplete() {
				return nil
			}
		}

		u.reportProgress()

		if u.controller.upgradeState.PhaseComplete() {
			break
		}

		u.sleeper.Sleep(u.attemptInterval)
	}
	return u.checkStillBusyInstances()
}

func (u *Upgrader) Upgrade() error {
	var canaryInstances []service.Instance
	var err error

	u.listener.Starting(u.maxInFlight)

	allInstances, err := u.instanceLister.Instances()
	if err != nil {
		return fmt.Errorf("error listing service instances: %s", err)
	}

	if len(u.controller.canarySelectionParams) > 0 {
		canaryInstances, err = u.instanceLister.FilteredInstances(u.controller.canarySelectionParams)
		if err != nil {
			return fmt.Errorf("error listing service instances: %s", err)
		}
		if len(canaryInstances) == 0 && len(allInstances) > 0 {
			return fmt.Errorf("Upgrade failed to find a match to the canary selection criteria: %s Please ensure that this criterion will match 1 or more service instances, or remove the criteria to proceed without canaries", u.controller.canarySelectionParams)
		}
		if len(canaryInstances) < u.controller.canaries {
			u.controller.canaries = len(canaryInstances)
		}
	} else {
		if u.controller.canaries > 0 {
			canaryInstances = allInstances
		} else {
			canaryInstances = []service.Instance{}
		}
	}
	u.controller.upgradeState, err = NewUpgradeState(canaryInstances, allInstances, u.controller.canaries)
	if err != nil {
		return fmt.Errorf("error with canary instance listing: %s", err)
	}

	u.listener.InstancesToUpgrade(allInstances)

	if u.controller.upgradeState.IsProcessingCanaries() {
		u.listener.CanariesStarting(u.controller.upgradeState.OutstandingCanaryCount(), u.controller.canarySelectionParams)
		if err := u.UpgradeInstancesWithAttempts(); err != nil {
			u.summary()
			return err
		}
		u.controller.upgradeState.MarkCanariesCompleted()
		u.listener.CanariesFinished()
	}

	if err := u.UpgradeInstancesWithAttempts(); err != nil {
		u.summary()
		return err
	}
	u.summary()
	return nil
}

func (u *Upgrader) logRetryAttempt(attempt int) {
	if u.controller.upgradeState.IsProcessingCanaries() {
		outstanding := u.controller.upgradeState.OutstandingCanaryCount()
		u.listener.RetryCanariesAttempt(attempt, u.attemptLimit, outstanding)
	} else {
		u.listener.RetryAttempt(attempt, u.attemptLimit)
	}
}

func (u *Upgrader) upgradesToTriggerCount() int {
	inProg := len(u.controller.upgradeState.GetInstancesInStates(services.UpgradeAccepted))
	needed := u.maxInFlight - inProg
	if u.controller.upgradeState.IsProcessingCanaries() {
		outstandingCanaries := u.controller.upgradeState.OutstandingCanaryCount()
		if needed > outstandingCanaries {
			needed = outstandingCanaries
		}
	}
	return needed
}

func (u *Upgrader) triggerUpgrades() {
	needed := u.upgradesToTriggerCount()
	totalInstances := u.controller.upgradeState.InstanceCountInPhase()

	if needed <= 0 || len(u.controller.upgradeState.GetInstancesInStates(services.UpgradeFailed)) > 0 {
		return
	}
	for i := 0; i < needed; {
		instance, err := u.controller.upgradeState.NextPending()
		if err != nil {
			break
		}
		u.listener.InstanceUpgradeStarting(instance.GUID, u.controller.calculateCurrentUpgradeIndex(), totalInstances, u.controller.upgradeState.IsProcessingCanaries())
		t := NewTriggerer(u.brokerServices, u.instanceLister, u.listener)
		operation, err := t.TriggerUpgrade(instance)
		if err != nil {
			u.controller.upgradeState.SetState(instance.GUID, services.UpgradeFailed)
			u.controller.failures = append(u.controller.failures, instanceFailure{guid: instance.GUID, err: err})
			return
		}
		u.controller.upgradeState.SetUpgradeOperation(instance.GUID, operation)
		u.controller.upgradeState.SetState(instance.GUID, operation.Type)
		u.listener.InstanceUpgradeStartResult(instance.GUID, operation.Type)

		if operation.Type == services.UpgradeAccepted {
			u.listener.WaitingFor(instance.GUID, operation.Data.BoshTaskID)
			i++
		}
	}
}

func (u *Upgrader) pollRunningTasks() {
	for _, inst := range u.controller.upgradeState.GetInstancesInStates(services.UpgradeAccepted) {
		guid := inst.GUID
		sc := NewStateChecker(u.brokerServices)
		state, err := sc.CheckState(guid, u.controller.upgradeState.GetUpgradeOperation(guid).Data)
		if err != nil {
			u.controller.failures = append(u.controller.failures, instanceFailure{guid: guid, err: err})
			continue
		}
		u.controller.upgradeState.SetState(guid, state.Type)

		switch state.Type {
		case services.UpgradeSucceeded:
			u.listener.InstanceUpgraded(guid, "success")
		case services.UpgradeFailed:
			u.listener.InstanceUpgraded(guid, "failure")
			upgradeErr := fmt.Errorf("[%s] Upgrade failed: bosh task id %d: %s", guid, state.Data.BoshTaskID, state.Description)
			u.controller.failures = append(u.controller.failures, instanceFailure{guid: guid, err: upgradeErr})
		}
	}
}

func (u *Upgrader) reportProgress() {
	orphaned := len(u.controller.findInstancesWithState(services.OrphanDeployment))
	succeeded := len(u.controller.findInstancesWithState(services.UpgradeSucceeded))
	busy := len(u.controller.findInstancesWithState(services.OperationInProgress))
	deleted := len(u.controller.findInstancesWithState(services.InstanceNotFound))
	u.listener.Progress(u.attemptInterval, orphaned, succeeded, busy, deleted)
}

func (u *Upgrader) summary() {
	busyInstances := u.controller.findInstancesWithState(services.OperationInProgress)
	orphaned := len(u.controller.findInstancesWithState(services.OrphanDeployment))
	succeeded := len(u.controller.findInstancesWithState(services.UpgradeSucceeded))
	deleted := len(u.controller.findInstancesWithState(services.InstanceNotFound))
	failedList := u.controller.failures
	var failedInstances []string
	for _, failure := range failedList {
		failedInstances = append(failedInstances, failure.guid)
	}
	u.listener.Finished(orphaned, succeeded, deleted, busyInstances, failedInstances)
}

func (u *Upgrader) checkStillBusyInstances() error {
	busyInstances := u.controller.findInstancesWithState(services.OperationInProgress)
	busyInstancesCount := len(busyInstances)

	if busyInstancesCount > 0 {
		if u.controller.upgradeState.IsProcessingCanaries() {
			if !u.controller.upgradeState.canariesCompleted() {
				return fmt.Errorf(
					"canaries didn't upgrade successfully: attempted to upgrade %d canaries, but only found %d instances not already in use by another BOSH task.",
					u.controller.canaries,
					u.controller.canaries-busyInstancesCount,
				)
			}
			return nil
		}
		return fmt.Errorf("The following instances could not be upgraded: %s", strings.Join(busyInstances, ", "))
	}
	return nil
}

func (u *Upgrader) formatError() error {
	err := u.errorFromList()
	if u.controller.upgradeState.IsProcessingCanaries() {
		return errors.Wrap(err, "canaries didn't upgrade successfully")
	}
	return err
}

func (u *Upgrader) errorFromList() error {
	failureList := u.controller.failures
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

func (c *controller) findInstancesWithState(state services.UpgradeOperationType) []string {
	out := []string{}
	for _, inst := range c.upgradeState.GetInstancesInStates(state) {
		out = append(out, inst.GUID)
	}
	return out
}

func (c *controller) calculateCurrentUpgradeIndex() int {
	return len(c.upgradeState.GetInstancesInStates(services.UpgradeSucceeded, services.UpgradeAccepted, services.InstanceNotFound, services.OrphanDeployment)) + 1
}
