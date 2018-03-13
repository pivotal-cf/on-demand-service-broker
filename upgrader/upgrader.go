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
	pendingInstances      []service.Instance
	busyInstances         []service.Instance
	inProgress            []service.Instance
	failures              []instanceFailure
	canaries              int
	outstandingCanaries   int
	processingCanaries    bool
	canarySelectionParams config.CanarySelectionParams

	states map[string]services.UpgradeOperation
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
	totalInstances  int
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
			states:                make(map[string]services.UpgradeOperation),
			canaries:              builder.Canaries,
			outstandingCanaries:   builder.Canaries,
			processingCanaries:    builder.Canaries > 0,
			canarySelectionParams: builder.CanarySelectionParams,
		},
	}
}

func (u *Upgrader) filteredInstances(unfilteredInstances []service.Instance) ([]service.Instance, error) {
	instances, err := u.instanceLister.FilteredInstances(u.controller.canarySelectionParams)
	if len(instances) == 0 {
		if len(unfilteredInstances) > 0 {
			return nil, fmt.Errorf("Upgrade failed to find a match to the canary selection criteria: %s Please ensure that this criterion will match 1 or more service instances, or remove the criteria to proceed without canaries", u.controller.canarySelectionParams)
		}
	}
	if u.controller.canaries == 0 || u.controller.canaries > len(instances) {
		u.controller.canaries = len(instances)
		u.controller.outstandingCanaries = u.controller.canaries
	}
	u.controller.processingCanaries = u.controller.canaries > 0
	return instances, err
}

func (u *Upgrader) UpgradeInstancesWithAttempts(instances []service.Instance, limit int) error {
	u.controller.pendingInstances = instances
	u.totalInstances = len(u.controller.pendingInstances)

	for attempt := 1; attempt <= u.attemptLimit; attempt++ {
		u.logRetryAttempt(attempt)

		for u.controller.hasInstancesToUpgrade() {
			u.triggerUpgrades()

			u.pollRunningTasks()

			if len(u.controller.findInstancesWithState(services.UpgradeAccepted)) > 0 {
				u.sleeper.Sleep(u.pollingInterval)
				continue
			}

			if len(u.controller.failures) > 0 {
				u.summary()
				return u.formatError()
			}

			if u.controller.processingCanaries && (u.controller.outstandingCanaries == 0 || u.upgradeCompleted(instances)) {
				u.controller.processingCanaries = false
				u.listener.CanariesFinished()
				return nil
			}
		}

		u.reportProgress()

		if u.upgradeCompleted(instances) {
			return nil
		}

		u.controller.pendingInstances = u.controller.busyInstances
		u.controller.busyInstances = nil

		u.sleeper.Sleep(u.attemptInterval)
	}
	return u.summary()
}

func (u *Upgrader) Upgrade() error {
	var instances []service.Instance
	var err error

	canaryInstances := []service.Instance{}

	u.listener.Starting(u.maxInFlight)

	allInstances, err := u.instanceLister.Instances()
	if err != nil {
		return fmt.Errorf("error listing service instances: %s", err)
	}

	if len(u.controller.canarySelectionParams) > 0 {
		canaryInstances, err = u.filteredInstances(allInstances)
		if err != nil {
			return fmt.Errorf("error listing service instances: %s", err)
		}
		instances = canaryInstances
	} else {
		instances = allInstances
	}

	u.listener.InstancesToUpgrade(allInstances)

	u.controller.pendingInstances = instances

	u.totalInstances = len(allInstances)

	if u.controller.processingCanaries {
		u.listener.CanariesStarting(u.controller.canaries, u.controller.canarySelectionParams)
		if err := u.UpgradeInstancesWithAttempts(instances, u.controller.canaries); err != nil {
			return err
		}
	}

	if err := u.UpgradeInstancesWithAttempts(allInstances, -1); err != nil {
		return err
	}

	return u.summary()
}

func (u *Upgrader) logRetryAttempt(attempt int) {
	if u.controller.processingCanaries {
		u.listener.RetryCanariesAttempt(attempt, u.attemptLimit, u.controller.outstandingCanaries)
	} else {
		u.listener.RetryAttempt(attempt, u.attemptLimit)
	}
}

func (u *Upgrader) upgradesToTriggerCount() int {
	var needed int
	if u.controller.processingCanaries {
		needed = u.controller.outstandingCanaries
		if needed > u.maxInFlight {
			needed = u.maxInFlight
		}
	} else {
		needed = u.maxInFlight - len(u.controller.findInstancesWithState(services.UpgradeAccepted))
	}
	return needed
}

func (u *Upgrader) triggerUpgrades() {
	needed := u.upgradesToTriggerCount()

	if needed > 0 && len(u.controller.failures) == 0 {
		for i := 0; i < needed; {
			instance := u.controller.nextInstance()
			if instance.GUID == "" {
				break
			}
			accepted, err := u.triggerUpgrade(instance)
			if accepted {
				i++
			}
			if err != nil {
				u.controller.failures = append(
					u.controller.failures,
					instanceFailure{
						guid: instance.GUID,
						err:  err,
					},
				)
			}
		}
	}
}

func (u *Upgrader) pollRunningTasks() {
	for _, guid := range u.controller.findInstancesWithState(services.UpgradeAccepted) {
		sc := NewStateChecker(u.brokerServices)
		state, err := sc.CheckState(guid, u.controller.states[guid].Data)
		if err != nil {
			u.controller.failures = append(u.controller.failures, instanceFailure{guid: guid, err: err})
			continue
		}

		u.controller.states[guid] = state

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

func (u *Upgrader) summary() error {
	pendingInstances := u.controller.findInstancesWithState(services.OperationInProgress)
	busyInstances := u.controller.findInstancesWithState(services.OperationInProgress)
	orphaned := len(u.controller.findInstancesWithState(services.OrphanDeployment))
	succeeded := len(u.controller.findInstancesWithState(services.UpgradeSucceeded))
	deleted := len(u.controller.findInstancesWithState(services.InstanceNotFound))
	failedList := u.controller.failures
	var failedInstances []string
	for _, failure := range failedList {
		failedInstances = append(failedInstances, failure.guid)
	}
	pendingInstancesCount := len(pendingInstances)

	u.listener.Finished(orphaned, succeeded, deleted, busyInstances, failedInstances)

	if pendingInstancesCount > 0 {
		if u.controller.processingCanaries {
			return fmt.Errorf(
				"canaries didn't upgrade successfully: attempted to upgrade %d canaries, but only found %d instances not already in use by another BOSH task.",
				u.controller.canaries,
				u.controller.canaries-pendingInstancesCount,
			)
		}
		return fmt.Errorf("The following instances could not be upgraded: %s", strings.Join(pendingInstances, ", "))
	}
	return nil
}

func (u *Upgrader) formatError() error {
	err := u.errorFromList()
	if u.controller.processingCanaries {
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

func (u *Upgrader) triggerUpgrade(instance service.Instance) (bool, error) {
	u.listener.InstanceUpgradeStarting(instance.GUID, u.controller.calculateCurrentUpgradeIndex(), u.totalInstances, u.controller.processingCanaries)
	latestInstance, err := u.instanceLister.LatestInstanceInfo(instance)
	if err != nil {
		if err == service.InstanceNotFound {
			u.controller.states[instance.GUID] = services.UpgradeOperation{Type: services.InstanceNotFound}
			u.listener.InstanceUpgradeStartResult(latestInstance.GUID, services.InstanceNotFound)
			return false, nil
		} else {
			latestInstance = instance
			u.listener.FailedToRefreshInstanceInfo(instance.GUID)
		}
	}

	operation, err := u.brokerServices.UpgradeInstance(latestInstance)
	if err != nil {
		return false, fmt.Errorf(
			"Upgrade failed for service instance %s: %s\n", instance.GUID, err,
		)
	}

	u.listener.InstanceUpgradeStartResult(instance.GUID, operation.Type)
	u.controller.states[instance.GUID] = operation

	accepted := false
	switch operation.Type {
	case services.OrphanDeployment:
	case services.InstanceNotFound:
	case services.OperationInProgress:
		u.controller.isBusy(instance)
	case services.UpgradeAccepted:
		accepted = true
		u.controller.isInProgress(instance)
		u.listener.WaitingFor(instance.GUID, operation.Data.BoshTaskID)
		if u.controller.processingCanaries {
			u.controller.outstandingCanaries--
		}
	}
	return accepted, nil
}

func (u *Upgrader) upgradeCompleted(instances []service.Instance) bool {
	for _, instance := range instances {
		s, ok := u.controller.states[instance.GUID]
		if !ok || s.Type == services.OperationInProgress {
			return false
		}
	}
	return true
}

func (c *controller) hasInstancesToUpgrade() bool {
	return len(c.pendingInstances) > 0 || len(c.findInstancesWithState(services.UpgradeAccepted)) > 0
}

func (c *controller) isBusy(instance service.Instance) {
	c.busyInstances = append(c.busyInstances, instance)
}

func (c *controller) isInProgress(instance service.Instance) {
	c.inProgress = append(c.inProgress, instance)
}

func (c *controller) nextInstance() service.Instance {
	if len(c.pendingInstances) > 0 {
		instance := c.pendingInstances[0]
		c.pendingInstances = c.pendingInstances[1:len(c.pendingInstances)]
		state, found := c.states[instance.GUID]
		if found && state.Type != services.OperationInProgress {
			return c.nextInstance()
		}
		return instance
	} else {
		return service.Instance{}
	}
}

func (c *controller) nextInProgressInstance() service.Instance {
	if len(c.inProgress) > 0 {
		instance := c.inProgress[0]
		c.inProgress = c.inProgress[1:len(c.inProgress)]
		return instance
	} else {
		return service.Instance{}
	}
}

func (c *controller) findInstancesWithState(state services.UpgradeOperationType) []string {
	out := make([]string, 0)
	for guid, finalState := range c.states {
		if finalState.Type == state {
			out = append(out, guid)
		}
	}
	return out
}

func (c *controller) calculateCurrentUpgradeIndex() int {
	out := 1
	for _, finalState := range c.states {
		if (finalState.Type == services.UpgradeSucceeded) ||
			(finalState.Type == services.UpgradeAccepted) ||
			(finalState.Type == services.InstanceNotFound) ||
			(finalState.Type == services.OrphanDeployment) {
			out += 1
		}
	}
	return out
}
