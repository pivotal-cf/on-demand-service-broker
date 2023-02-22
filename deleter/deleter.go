// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package deleter

import (
	"fmt"
	"log"

	"time"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate -o fakes/fake_cloud_foundry_client.go . CloudFoundryClient
type CloudFoundryClient interface {
	GetServiceInstances(filter cf.GetInstancesFilter, logger *log.Logger) ([]cf.Instance, error)
	GetLastOperationForInstance(instanceGUID string, logger *log.Logger) (cf.LastOperation, error)
	GetBindingsForInstance(instanceGUID string, logger *log.Logger) ([]cf.Binding, error)
	DeleteBinding(binding cf.Binding, logger *log.Logger) error
	GetServiceKeysForInstance(instanceGUID string, logger *log.Logger) ([]cf.ServiceKey, error)
	DeleteServiceKey(serviceKey cf.ServiceKey, logger *log.Logger) error
	DeleteServiceInstance(instanceGUID string, logger *log.Logger) error
}

//counterfeiter:generate -o fakes/fake_sleeper.go . Sleeper
type Sleeper interface {
	Sleep(d time.Duration)
}

type Config struct {
	ServiceCatalog             ServiceCatalog `yaml:"service_catalog"`
	DisableSSLCertVerification bool           `yaml:"disable_ssl_cert_verification"` // TODO use the CF.disable_ssl_cert_verification field
	CF                         config.CF      `yaml:"cf"`
	PollingInterval            int            `yaml:"polling_interval"`
	PollingInitialOffset       int            `yaml:"polling_initial_offset"`
}

type ServiceCatalog struct {
	ID string `yaml:"id"`
}

type Deleter struct {
	logger               *log.Logger
	pollingInitialOffset time.Duration
	pollingInterval      time.Duration
	cfClient             CloudFoundryClient
	sleeper              Sleeper
}

func New(cfClient CloudFoundryClient, sleeper Sleeper, pollingInitialOffset int, pollingInterval int, logger *log.Logger) *Deleter {
	return &Deleter{
		logger:               logger,
		pollingInitialOffset: time.Duration(pollingInitialOffset) * time.Second,
		pollingInterval:      time.Duration(pollingInterval) * time.Second,
		cfClient:             cfClient,
		sleeper:              sleeper,
	}
}

func (d *Deleter) DeleteAllServiceInstances(serviceUniqueID string) error {
	d.logger.Printf("Deleter Configuration: polling_intial_offset: %v, polling_interval: %v.", d.pollingInitialOffset.Seconds(), d.pollingInterval.Seconds())
	instancesFilter := cf.GetInstancesFilter{ServiceOfferingID: serviceUniqueID}
	serviceInstances, err := d.cfClient.GetServiceInstances(instancesFilter, d.logger)
	if err != nil {
		return err
	}

	if len(serviceInstances) == 0 {
		d.logger.Println("No service instances found.")
		return nil
	}

	for _, instance := range serviceInstances {
		err = d.deleteBindings(instance.GUID)
		if err != nil {
			return err
		}

		err = d.deleteServiceKeys(instance.GUID)
		if err != nil {
			return err
		}

		deleteInProgress, err := d.deleteInProgress(instance.GUID)
		if err != nil {
			d.logger.Printf("could not retrieve information about service instance %s, will try to delete", instance.GUID)
		}
		if deleteInProgress {
			d.logger.Printf("service instance %s is being deleted, will skip sending the delete request", instance.GUID)
		} else {
			if err = d.deleteServiceInstance(instance.GUID); err != nil {
				return err
			}
		}

		d.logger.Printf("Waiting for service instance %s to be deleted", instance.GUID)

		err = d.pollInstanceDeleteStatus(instance.GUID)
		if err != nil {
			return err
		}
	}

	serviceInstances, err = d.cfClient.GetServiceInstances(instancesFilter, d.logger)
	if err != nil {
		return err
	}

	if len(serviceInstances) != 0 {
		return fmt.Errorf("expected 0 instances for service offering with unique ID: %s. Got %d instance(s).", serviceUniqueID, len(serviceInstances))
	}

	return nil
}

func (d Deleter) deleteBindings(instanceGUID string) error {
	bindings, err := d.cfClient.GetBindingsForInstance(instanceGUID, d.logger)
	switch err.(type) {
	case cf.ResourceNotFoundError:
		return nil
	case error:
		return err
	}

	for _, binding := range bindings {
		d.logger.Printf("Deleting binding %s of service instance %s to app %s\n", binding.GUID, instanceGUID, binding.AppGUID)
		err = d.cfClient.DeleteBinding(binding, d.logger)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d Deleter) deleteServiceKeys(instanceGUID string) error {
	serviceKeys, err := d.cfClient.GetServiceKeysForInstance(instanceGUID, d.logger)
	switch err.(type) {
	case cf.ResourceNotFoundError:
		return nil
	case error:
		return err
	}

	for _, serviceKey := range serviceKeys {
		d.logger.Printf("Deleting service key %s of service instance %s\n", serviceKey.GUID, instanceGUID)
		err = d.cfClient.DeleteServiceKey(serviceKey, d.logger)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d Deleter) deleteServiceInstance(instanceGUID string) error {
	d.logger.Printf("Deleting service instance %s\n", instanceGUID)
	return d.cfClient.DeleteServiceInstance(instanceGUID, d.logger)
}

func (d Deleter) pollInstanceDeleteStatus(instanceGUID string) error {
	d.sleeper.Sleep(d.pollingInitialOffset)

	for {
		d.sleeper.Sleep(d.pollingInterval)

		lastOperation, err := d.cfClient.GetLastOperationForInstance(instanceGUID, d.logger)
		switch err.(type) {
		case cf.ResourceNotFoundError:
			d.logger.Printf("Result: deleted service instance %s", instanceGUID)
			return nil
		case cf.UnauthorizedError,
			cf.ForbiddenError,
			cf.InvalidResponseError:
			return fmt.Errorf("Result: failed to delete service instance %s. Error: %s.", instanceGUID, err)
		case error:
			continue
		}

		if !lastOperation.IsDelete() {
			return fmt.Errorf(
				"Result: failed to delete service instance %s. Unexpected operation type: '%s'.",
				instanceGUID,
				lastOperation.Type,
			)
		}

		if lastOperation.OperationFailed() {
			return fmt.Errorf("Result: failed to delete service instance %s. Delete operation failed.", instanceGUID)
		}
	}
}

func (d Deleter) deleteInProgress(instanceGUID string) (bool, error) {
	lastOperation, err := d.cfClient.GetLastOperationForInstance(instanceGUID, d.logger)

	if err != nil {
		return false, err
	}

	return lastOperation.IsDelete(), nil
}
