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
	Finished(orphanCount, upgradedCount, deletedCount, couldNotStartCount int, failedInstances ...string)
	CanariesStarting(canaries int)
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
	LatestInstanceInfo(inst service.Instance) (service.Instance, error)
}

//go:generate counterfeiter -o fakes/fake_sleeper.go . sleeper
type sleeper interface {
	Sleep(d time.Duration)
}

type controller struct {
	pendingInstances    []service.Instance
	busyInstances       []service.Instance
	inProgress          []service.Instance
	failures            []instanceFailure
	outstandingCanaries int
	processingCanaries  bool

	states map[string]services.UpgradeOperation
}

type instanceFailure struct {
	instance service.Instance
	err      error
}

type Upgrader struct {
	brokerServices  BrokerServices
	instanceLister  InstanceLister
	pollingInterval time.Duration
	attemptInterval time.Duration
	attemptLimit    int
	maxInFlight     int
	canaries        int
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
		canaries:        builder.Canaries,
		listener:        builder.Listener,
		sleeper:         builder.Sleeper,
		controller: &controller{
			states:              make(map[string]services.UpgradeOperation),
			outstandingCanaries: builder.Canaries,
			processingCanaries:  builder.Canaries > 0,
		},
	}
}

func (u *Upgrader) Upgrade() error {
	u.listener.Starting(u.maxInFlight)
	instances, err := u.instanceLister.Instances()
	if err != nil {
		return fmt.Errorf("error listing service instances: %s", err)
	}

	u.listener.InstancesToUpgrade(instances)

	u.controller.pendingInstances = instances

	if u.controller.processingCanaries {
		u.listener.CanariesStarting(u.canaries)
	}

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

			if u.controller.processingCanaries && u.controller.outstandingCanaries == 0 {
				u.controller.processingCanaries = false
				u.listener.CanariesFinished()
				attempt = 0
			}
		}

		if attempt == 0 {
			continue
		}

		u.reportProgress()

		if u.upgradeCompleted(instances) {
			break
		}

		u.controller.pendingInstances = u.controller.busyInstances
		u.controller.busyInstances = nil

		u.sleeper.Sleep(u.attemptInterval)
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
	index := 1
	totalInstance := len(u.controller.pendingInstances)

	if needed > 0 && len(u.controller.failures) == 0 {
		for i := 0; i < needed; index++ {
			instance := u.controller.nextInstance()
			if instance.GUID == "" {
				break
			}
			accepted, err := u.triggerUpgrade(instance, index, totalInstance)
			if accepted {
				i++
			}
			if err != nil {
				u.controller.failures = append(
					u.controller.failures,
					instanceFailure{
						instance: instance,
						err:      err,
					},
				)
			}
		}
	}
}

func (u *Upgrader) pollRunningTasks() {
	for range u.controller.findInstancesWithState(services.UpgradeAccepted) {
		instance := u.controller.nextInProgressInstance()
		err := u.poll(instance)
		if err != nil {
			u.controller.failures = append(
				u.controller.failures,
				instanceFailure{
					instance: instance,
					err:      err,
				},
			)
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
		failedInstances = append(failedInstances, failure.instance.GUID)
	}
	pendingInstancesCount := len(pendingInstances)

	u.listener.Finished(orphaned, succeeded, deleted, pendingInstancesCount, failedInstances...)

	if pendingInstancesCount > 0 {
		if u.controller.processingCanaries {
			return fmt.Errorf(
				"canaries didn't upgrade successfully: attempted to upgrade %d canaries, but only found %d instances not already in use by another BOSH task.",
				u.canaries,
				u.canaries-pendingInstancesCount,
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

func (u *Upgrader) triggerUpgrade(instance service.Instance, index, totalInstances int) (bool, error) {
	u.listener.InstanceUpgradeStarting(instance.GUID, index, totalInstances, u.controller.processingCanaries)
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

func (u *Upgrader) poll(instance service.Instance) error {
	lastOperation, err := u.brokerServices.LastOperation(instance.GUID, u.controller.states[instance.GUID].Data)
	if err != nil {
		return fmt.Errorf("error getting last operation: %s\n", err)
	}

	switch lastOperation.State {
	case brokerapi.Failed:
		d := services.UpgradeOperation{Type: services.UpgradeFailed, Data: u.controller.states[instance.GUID].Data}
		u.controller.states[instance.GUID] = d
		u.listener.InstanceUpgraded(instance.GUID, "failure")
		return fmt.Errorf("[%s] Upgrade failed: bosh task id %d: %s",
			instance.GUID, u.controller.states[instance.GUID].Data.BoshTaskID, lastOperation.Description)
	case brokerapi.Succeeded:
		d := services.UpgradeOperation{Type: services.UpgradeSucceeded, Data: u.controller.states[instance.GUID].Data}
		u.controller.states[instance.GUID] = d
		u.listener.InstanceUpgraded(instance.GUID, "success")
	case brokerapi.InProgress:
		u.controller.isInProgress(instance)
		return nil
	default:
		return fmt.Errorf("not nice")
	}
	return nil
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
			out = append(out, "service-instance_"+guid)
		}
	}
	return out
}
