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

	"net/url"

	"crypto/x509"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

//go:generate counterfeiter -o fakes/fake_listener.go . Listener
type Listener interface {
	Starting()
	InstancesToUpgrade(instances []service.Instance)
	InstanceUpgradeStarting(instance string, index, totalInstances int)
	InstanceUpgradeStartResult(status services.UpgradeOperationType)
	InstanceUpgraded(instance string, result string)
	WaitingFor(instance string, boshTaskId int)
	Progress(pollingInterval time.Duration, orphanCount, upgradedCount, upgradesLeftCount, deletedCount int)
	Finished(orphanCount, upgradedCount, deletedCount int)
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

type upgrader struct {
	brokerServices  BrokerServices
	instanceLister  InstanceLister
	pollingInterval time.Duration
	listener        Listener
}

func New(brokerServices BrokerServices, instanceLister InstanceLister, pollingInterval int, listener Listener) upgrader {
	return upgrader{
		brokerServices:  brokerServices,
		instanceLister:  instanceLister,
		pollingInterval: time.Duration(pollingInterval) * time.Second,
		listener:        listener,
	}
}

func (u upgrader) Upgrade() error {
	var upgradedTotal, orphansTotal, deletedTotal int

	u.listener.Starting()

	instances, err := u.instanceLister.Instances()
	if err != nil {
		urlError, ok := err.(*url.Error)
		if ok {
			if urlError.Err == nil {
				return errors.New("error listing service instances: unknown url.Error")
			}
			_, ok := urlError.Err.(x509.UnknownAuthorityError)
			if ok {
				return fmt.Errorf(
					"SSL validation error for `service_instances_api.url`: %s. Please configure a `service_instances_api.root_ca_cert` and use a valid SSL certificate",
					urlError.URL)
			}
		}
		return fmt.Errorf("error listing service instances: %s", err)
	}

	u.listener.InstancesToUpgrade(instances)

	for len(instances) > 0 {
		upgradedCount, orphanCount, deletedCount, retryInstances, err := u.upgradeInstances(instances)
		if err != nil {
			return err
		}

		upgradedTotal += upgradedCount
		orphansTotal += orphanCount
		deletedTotal += deletedCount

		instances = retryInstances
		retryCount := len(instances)

		u.listener.Progress(u.pollingInterval, orphansTotal, upgradedTotal, retryCount, deletedTotal)
		if retryCount > 0 {
			time.Sleep(u.pollingInterval)
		}
	}

	u.listener.Finished(orphansTotal, upgradedTotal, deletedTotal)

	return nil
}

func (u upgrader) upgradeInstances(instances []service.Instance) (int, int, int, []service.Instance, error) {
	var (
		upgradedCount, orphanCount, deletedCount int
		instancesToRetry                         []service.Instance
	)

	instanceCount := len(instances)
	for i, instance := range instances {
		u.listener.InstanceUpgradeStarting(instance.GUID, i, instanceCount)
		operation, err := u.brokerServices.UpgradeInstance(instance)
		if err != nil {
			return 0, 0, 0, nil, fmt.Errorf(
				"Upgrade failed for service instance %s: %s\n", instance.GUID, err,
			)
		}

		u.listener.InstanceUpgradeStartResult(operation.Type)

		switch operation.Type {
		case services.OrphanDeployment:
			orphanCount++
		case services.InstanceNotFound:
			deletedCount++
		case services.OperationInProgress:
			instancesToRetry = append(instancesToRetry, instance)
		case services.UpgradeAccepted:
			if err := u.pollLastOperation(instance.GUID, operation.Data); err != nil {
				u.listener.InstanceUpgraded(instance.GUID, "failure")
				return 0, 0, 0, nil, err
			}
			u.listener.InstanceUpgraded(instance.GUID, "success")
			upgradedCount++
		}
	}

	return upgradedCount, orphanCount, deletedCount, instancesToRetry, nil
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
