// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrader

import (
	"errors"
	"fmt"
	"time"

	"strings"

	"sync/atomic"

	"sync"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

//go:generate counterfeiter -o fakes/fake_listener.go . Listener
type Listener interface {
	Starting(maxInFlight int)
	RetryAttempt(num, limit int)
	InstancesToUpgrade(instances []service.Instance)
	InstanceUpgradeStarting(instance string, index int32, totalInstances int)
	InstanceUpgradeStartResult(instance string, status services.UpgradeOperationType)
	InstanceUpgraded(instance string, result string)
	WaitingFor(instance string, boshTaskId int)
	Progress(pollingInterval time.Duration, orphanCount, upgradedCount int32, upgradesLeftCount int, deletedCount int32)
	Finished(orphanCount, upgradedCount, deletedCount int32, couldNotStartCount int)
}

//go:generate counterfeiter -o fakes/fake_broker_services.go . BrokerServices
type BrokerServices interface {
	UpgradeInstance(instance service.Instance) (services.UpgradeOperation, error)
	LastOperation(instance string, operationData broker.OperationData) (brokerapi.LastOperation, error)
}

//go:generate counterfeiter -o fakes/fake_instance_lister.go . InstanceLister
type InstanceLister interface {
	Instances() ([]service.Instance, error)
}

//go:generate counterfeiter -o fakes/fake_sleeper.go . sleeper
type sleeper interface {
	Sleep(d time.Duration)
}

type Upgrader struct {
	brokerServices         BrokerServices
	instanceLister         InstanceLister
	pollingInterval        time.Duration
	attemptInterval        time.Duration
	attemptLimit           int
	maxInFlight            int
	canaries               int
	listener               Listener
	sleeper                sleeper
	instanceCountToUpgrade int
	mutex                  sync.Mutex
	instancesBeingUpgraded int
	upgradedTotal          int32
	orphansTotal           int32
	deletedTotal           int32
	startedUpgradeTotal    int32
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
		mutex:           sync.Mutex{},
	}
}

func (u *Upgrader) errorFromList(errorList chan error) error {
	switch l := len(errorList); {
	case l == 1:
		return <-errorList
	case l > 1:
		close(errorList)
		var err error
		for e := range errorList {
			err = multierror.Append(err, e)
		}
		return errors.New(err.Error())
	default:
		return nil
	}
}

func (u *Upgrader) upgrade(instancesToUpgrade chan service.Instance, stopWorkers chan interface{}, errorList chan error, maxInstancesToUpgrade int, maxInParallel int) (chan service.Instance, error) {
	attempt := 1
	remainingInstancesToUpgrade := maxInstancesToUpgrade
	for remainingInstancesToUpgrade > 0 {
		u.instanceCountToUpgrade = remainingInstancesToUpgrade
		if attempt > u.attemptLimit {
			break
		}

		u.startedUpgradeTotal = 0
		u.listener.RetryAttempt(attempt, u.attemptLimit)
		instancesToRetry := make(chan service.Instance, remainingInstancesToUpgrade)
		var wg sync.WaitGroup

		parallelWorkers := maxInParallel
		if parallelWorkers > remainingInstancesToUpgrade {
			parallelWorkers = remainingInstancesToUpgrade
		}

		wg.Add(parallelWorkers)
		for i := 0; i < parallelWorkers; i++ {
			go func() {
				u.upgradeInstances(instancesToUpgrade, instancesToRetry, stopWorkers, errorList)
				wg.Done()
			}()
		}
		wg.Wait()
		close(instancesToRetry)

		if err := u.errorFromList(errorList); err != nil {
			return nil, err
		}

		if int(u.upgradedTotal) != maxInstancesToUpgrade {
			remainingInstancesToUpgrade = len(instancesToRetry)
		} else {
			remainingInstancesToUpgrade = 0
		}

		u.listener.Progress(u.attemptInterval, u.orphansTotal, u.upgradedTotal, remainingInstancesToUpgrade, u.deletedTotal)

		if len(instancesToRetry) > 0 {
			u.sleeper.Sleep(u.attemptInterval)

			mergeChan := make(chan service.Instance, len(instancesToRetry)+len(instancesToUpgrade))

			for i := range instancesToUpgrade {
				mergeChan <- i
			}

			for i := range instancesToRetry {
				mergeChan <- i
			}

			close(mergeChan)
			instancesToUpgrade = mergeChan
		}
		attempt++
	}

	if u.upgradedTotal < int32(u.canaries) {
		return nil, fmt.Errorf("attempted to upgrade %d canaries, but only found %d instances not already in use by another BOSH task.\n", u.canaries, atomic.LoadInt32(&u.upgradedTotal))
	}

	return instancesToUpgrade, nil
}

func (u *Upgrader) upgradeAll(instancesToUpgrade chan service.Instance, stopWorkers chan interface{}, errorList chan error) ([]service.Instance, error) {
	instancesToUpgrade, err := u.upgrade(instancesToUpgrade, stopWorkers, errorList, len(instancesToUpgrade), u.maxInFlight)
	if err != nil {
		return nil, err
	}
	var instancesNotUpgraded []service.Instance
	for i := range instancesToUpgrade {
		instancesNotUpgraded = append(instancesNotUpgraded, i)
	}
	return instancesNotUpgraded, nil
}

func (u *Upgrader) upgradeCanaries(instancesToUpgrade chan service.Instance, stopWorkers chan interface{}, errorList chan error) (chan service.Instance, error) {
	maxInParallel := u.canaries
	if u.canaries > u.maxInFlight {
		maxInParallel = u.maxInFlight
	}
	return u.upgrade(instancesToUpgrade, stopWorkers, errorList, u.canaries, maxInParallel)
}

func (u *Upgrader) Upgrade() error {
	u.listener.Starting(u.maxInFlight)

	instances, err := u.instanceLister.Instances()
	if err != nil {
		return fmt.Errorf("error listing service instances: %s", err)
	}

	u.listener.InstancesToUpgrade(instances)
	instancesToUpgrade := make(chan service.Instance, len(instances))

	stopWorkers := make(chan interface{})
	errorList := make(chan error, len(instances))

	for _, instance := range instances {
		instancesToUpgrade <- instance
	}
	close(instancesToUpgrade)

	u.instanceCountToUpgrade = u.canaries
	instancesToUpgrade, err = u.upgradeCanaries(instancesToUpgrade, stopWorkers, errorList)
	if err != nil {
		return fmt.Errorf("canaries didn't upgrade successfully: %s", err)
	}

	u.instanceCountToUpgrade = len(instancesToUpgrade)

	instancesNotUpgraded, err := u.upgradeAll(instancesToUpgrade, stopWorkers, errorList)
	if err != nil {
		return err
	}

	u.instanceCountToUpgrade = len(instancesNotUpgraded)
	u.listener.Finished(u.orphansTotal, u.upgradedTotal, u.deletedTotal, u.instanceCountToUpgrade)

	var instanceDeploymentNames []string
	for _, inst := range instancesNotUpgraded {
		instanceDeploymentNames = append(instanceDeploymentNames, fmt.Sprintf("service-instance_%s", inst.GUID))
	}
	if len(instanceDeploymentNames) > 0 {
		return fmt.Errorf("The following instances could not be upgraded: %s", strings.Join(instanceDeploymentNames, ", "))
	}

	return nil
}

func (u *Upgrader) upgradeInstances(instances, retries chan service.Instance, stop chan interface{}, errors chan error) {
	for {
		select {
		case <-stop:
			return
		default:
			u.mutex.Lock()
			if u.instancesBeingUpgraded >= u.instanceCountToUpgrade {
				u.mutex.Unlock()
				return
			}

			u.instancesBeingUpgraded++
			u.mutex.Unlock()

			defer func() {
				u.mutex.Lock()
				u.instancesBeingUpgraded--
				u.mutex.Unlock()
			}()

			instance, ok := <-instances
			if !ok {
				return
			}
			err := u.performInstanceUpgrade(instance, retries)
			if err != nil {
				errors <- err
				ensureChannelClosed(stop)
				return
			}
		}
	}
}

func ensureChannelClosed(ch chan interface{}) {
	select {
	case _, ok := <-ch:
		if ok {
			close(ch)
		}
	default:
		close(ch)
	}
}

func (u *Upgrader) performInstanceUpgrade(instance service.Instance, retryChan chan service.Instance) error {
	currentStartedUpgradeCount := atomic.AddInt32(&u.startedUpgradeTotal, 1)
	u.listener.InstanceUpgradeStarting(instance.GUID, currentStartedUpgradeCount-1, u.instanceCountToUpgrade)
	operation, err := u.brokerServices.UpgradeInstance(instance)
	if err != nil {
		return fmt.Errorf(
			"Upgrade failed for service instance %s: %s\n", instance.GUID, err,
		)
	}

	u.listener.InstanceUpgradeStartResult(instance.GUID, operation.Type)

	switch operation.Type {
	case services.OrphanDeployment:
		atomic.AddInt32(&u.orphansTotal, 1)
	case services.InstanceNotFound:
		atomic.AddInt32(&u.deletedTotal, 1)
	case services.OperationInProgress:
		retryChan <- instance
	case services.UpgradeAccepted:
		if err := u.pollLastOperation(instance.GUID, operation.Data); err != nil {
			u.listener.InstanceUpgraded(instance.GUID, "failure")
			return err
		}
		u.listener.InstanceUpgraded(instance.GUID, "success")
		atomic.AddInt32(&u.upgradedTotal, 1)
	}
	return nil
}

func (u *Upgrader) pollLastOperation(instance string, data broker.OperationData) error {
	u.listener.WaitingFor(instance, data.BoshTaskID)

	for {
		u.sleeper.Sleep(u.pollingInterval)

		lastOperation, err := u.brokerServices.LastOperation(instance, data)
		if err != nil {
			return fmt.Errorf("error getting last operation: %s\n", err)
		}

		switch lastOperation.State {
		case brokerapi.Failed:
			return fmt.Errorf("[%s] Upgrade failed: bosh task id %d: %s",
				instance, data.BoshTaskID, lastOperation.Description)
		case brokerapi.Succeeded:
			return nil
		}
	}
}
