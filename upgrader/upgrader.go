// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrader

import (
	"fmt"
	"time"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/brokerclient"
)

//go:generate counterfeiter -o fakes/fake_listener.go . Listener
type Listener interface {
	Starting()
	InstancesToUpgrade(instances []string)
	InstanceUpgradeStarting(instance string, index, totalInstances int)
	InstanceUpgradeStartResult(status brokerclient.UpgradeOperationType)
	InstanceUpgraded(instance string, result string)
	WaitingFor(instance string, boshTaskId int)
	Progress(pollingInterval time.Duration, orphanCount, upgradedCount, upgradesLeftCount, deletedCount int)
	Finished(orphanCount, upgradedCount, deletedCount int)
}

//go:generate counterfeiter -o fakes/fake_broker_services.go . BrokerServices
type BrokerServices interface {
	Instances() ([]string, error)
	UpgradeInstance(instance string) (brokerclient.UpgradeOperation, error)
	LastOperation(instance string, operationData broker.OperationData) (brokerapi.LastOperation, error)
}

type upgrader struct {
	brokerServices  BrokerServices
	brokerUsername  string
	brokerPassword  string
	brokerUrl       string
	pollingInterval time.Duration
	listener        Listener
}

func New(brokerServices BrokerServices, pollingInterval int, listener Listener) upgrader {
	return upgrader{
		brokerServices:  brokerServices,
		pollingInterval: time.Duration(pollingInterval) * time.Second,
		listener:        listener,
	}
}

func (u upgrader) Upgrade() error {
	var upgradedTotal, orphansTotal, deletedTotal int

	u.listener.Starting()

	instanceGUIDsToUpgrade, err := u.brokerServices.Instances()
	if err != nil {
		return fmt.Errorf("error listing service instances: %s", err)
	}

	u.listener.InstancesToUpgrade(instanceGUIDsToUpgrade)

	for len(instanceGUIDsToUpgrade) > 0 {
		upgradedCount, orphanCount, deletedCount, retryInstanceGUIDs, err := u.upgradeInstances(instanceGUIDsToUpgrade)
		if err != nil {
			return err
		}

		upgradedTotal += upgradedCount
		orphansTotal += orphanCount
		deletedTotal += deletedCount

		instanceGUIDsToUpgrade = retryInstanceGUIDs
		retryCount := len(instanceGUIDsToUpgrade)

		u.listener.Progress(u.pollingInterval, orphansTotal, upgradedTotal, retryCount, deletedTotal)
		if retryCount > 0 {
			time.Sleep(u.pollingInterval)
		}
	}

	u.listener.Finished(orphansTotal, upgradedTotal, deletedTotal)

	return nil
}

func (u upgrader) upgradeInstances(instances []string) (int, int, int, []string, error) {
	var (
		upgradedCount, orphanCount, deletedCount int
		idsToRetry                               []string
	)

	instanceCount := len(instances)
	for i, instance := range instances {
		u.listener.InstanceUpgradeStarting(instance, i, instanceCount)
		operation, err := u.brokerServices.UpgradeInstance(instance)
		if err != nil {
			return 0, 0, 0, nil, fmt.Errorf(
				"Upgrade failed for service instance %s: %s\n", instance, err,
			)
		}

		u.listener.InstanceUpgradeStartResult(operation.Type)

		switch operation.Type {
		case brokerclient.ResultOrphan:
			orphanCount++
		case brokerclient.ResultNotFound:
			deletedCount++
		case brokerclient.ResultOperationInProgress:
			idsToRetry = append(idsToRetry, instance)
		case brokerclient.ResultAccepted:
			if err := u.pollLastOperation(instance, operation.Data); err != nil {
				u.listener.InstanceUpgraded(instance, "failure")
				return 0, 0, 0, nil, err
			}
			u.listener.InstanceUpgraded(instance, "success")
			upgradedCount++
		}
	}

	return upgradedCount, orphanCount, deletedCount, idsToRetry, nil
}

func (u upgrader) pollLastOperation(instance string, data broker.OperationData) error {
	u.listener.WaitingFor(instance, data.BoshTaskID)

	for {
		time.Sleep(u.pollingInterval)

		lastOperation, err := u.brokerServices.LastOperation(instance, data)
		if err != nil {
			return fmt.Errorf("error getting last operation: %s\n", err)
		}

		switch lastOperation.State {
		case brokerapi.Failed:
			return fmt.Errorf("Upgrade failed for service instance %s: bosh task id %d: %s",
				instance, data.BoshTaskID, lastOperation.Description)
		case brokerapi.Succeeded:
			return nil
		}
	}
}
