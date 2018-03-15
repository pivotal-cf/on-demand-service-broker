// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrader

import (
	"fmt"
	"sort"
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
	pendingInstances      []string
	failures              []instanceFailure
	canaries              int
	outstandingCanaries   int
	processingCanaries    bool
	canarySelectionParams config.CanarySelectionParams

	states map[string]services.UpgradeOperation
	plans  map[string]string
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
			plans:                 make(map[string]string),
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

func (u *Upgrader) UpgradeInstancesWithAttempts(instances []string, limit int) error {
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
				return nil
			}
		}

		u.reportProgress()

		if u.upgradeCompleted(instances) {
			return nil
		}

		u.controller.pendingInstances = u.controller.findInstancesWithState(services.OperationInProgress)

		u.sleeper.Sleep(u.attemptInterval)
	}
	return u.summary()
}

func (u *Upgrader) Upgrade() error {
	var instances []service.Instance
	var err error

	u.listener.Starting(u.maxInFlight)

	allInstances, err := u.instanceLister.Instances()
	if err != nil {
		return fmt.Errorf("error listing service instances: %s", err)
	}

	for _, si := range allInstances {
		u.controller.plans[si.GUID] = si.PlanUniqueID
	}

	if len(u.controller.canarySelectionParams) > 0 {
		canaryInstances, err := u.filteredInstances(allInstances)
		if err != nil {
			return fmt.Errorf("error listing service instances: %s", err)
		}
		instances = canaryInstances
	} else {
		instances = allInstances
	}

	u.listener.InstancesToUpgrade(allInstances)

	var guids []string
	for _, in := range instances {
		guids = append(guids, in.GUID)
	}

	var allGuids []string
	for _, in := range allInstances {
		allGuids = append(allGuids, in.GUID)
	}

	if u.controller.processingCanaries {
		u.listener.CanariesStarting(u.controller.canaries, u.controller.canarySelectionParams)
		if err := u.UpgradeInstancesWithAttempts(guids, u.controller.canaries); err != nil {
			return err
		}
		u.listener.CanariesFinished()
	}

	if err := u.UpgradeInstancesWithAttempts(allGuids, -1); err != nil {
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

	if needed <= 0 || len(u.controller.failures) > 0 {
		return
	}
	for i := 0; i < needed; {
		instance := u.controller.nextInstance()
		if instance.GUID == "" {
			break
		}
		u.listener.InstanceUpgradeStarting(instance.GUID, u.controller.calculateCurrentUpgradeIndex(), u.totalInstances, u.controller.processingCanaries)
		t := NewTriggerer(u.brokerServices, u.instanceLister, u.listener)
		operation, err := t.TriggerUpgrade(instance)
		if err != nil {
			u.controller.failures = append(u.controller.failures, instanceFailure{guid: instance.GUID, err: err})
		}
		u.controller.states[instance.GUID] = operation
		u.listener.InstanceUpgradeStartResult(instance.GUID, operation.Type)

		if operation.Type == services.UpgradeAccepted {
			u.listener.WaitingFor(instance.GUID, operation.Data.BoshTaskID)
			if u.controller.processingCanaries {
				u.controller.outstandingCanaries--
			}
			i++
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
	orphaned := len(u.controller.findInstancesWithState(services.OrphanDeployment))
	succeeded := len(u.controller.findInstancesWithState(services.UpgradeSucceeded))
	deleted := len(u.controller.findInstancesWithState(services.InstanceNotFound))
	failedList := u.controller.failures
	var failedInstances []string
	for _, failure := range failedList {
		failedInstances = append(failedInstances, failure.guid)
	}
	pendingInstancesCount := len(pendingInstances)

	u.listener.Finished(orphaned, succeeded, deleted, pendingInstances, failedInstances)

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

func (u *Upgrader) upgradeCompleted(guids []string) bool {
	for _, guid := range guids {
		s, ok := u.controller.states[guid]
		if !ok || s.Type == services.OperationInProgress {
			return false
		}
	}
	return true
}

func (c *controller) hasInstancesToUpgrade() bool {
	return len(c.pendingInstances) > 0 || len(c.findInstancesWithState(services.UpgradeAccepted)) > 0
}

func (c *controller) nextInstance() service.Instance {
	if len(c.pendingInstances) > 0 {
		guid := c.pendingInstances[0]
		c.pendingInstances = c.pendingInstances[1:len(c.pendingInstances)]
		state, found := c.states[guid]
		if found && state.Type != services.OperationInProgress {
			return c.nextInstance()
		}
		return service.Instance{GUID: guid, PlanUniqueID: c.plans[guid]}
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
	// TODO
	sort.Strings(out)
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
