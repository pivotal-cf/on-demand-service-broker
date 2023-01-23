// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package purger

import (
	"fmt"
	"log"
)

const errorMessageTemplate = "Purger Failed: %s"

type Purger struct {
	deleter     Deleter
	deregistrar Deregistrar
	cfClient    CloudFoundryClient
	logger      *log.Logger
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate -o fakes/fake_deleter.go . Deleter
type Deleter interface {
	DeleteAllServiceInstances(string) error
}

//counterfeiter:generate -o fakes/fake_registrar.go . Deregistrar
type Deregistrar interface {
	Deregister(string) error
}

//counterfeiter:generate -o fakes/fake_cloud_foundry_client.go . CloudFoundryClient
type CloudFoundryClient interface {
	DisableServiceAccessForAllPlans(serviceOfferingID string, logger *log.Logger) error
}

func New(d Deleter, r Deregistrar, cfClient CloudFoundryClient, logger *log.Logger) *Purger {
	return &Purger{
		deleter:     d,
		deregistrar: r,
		cfClient:    cfClient,
		logger:      logger,
	}
}

func (p Purger) DeleteInstancesAndDeregister(serviceCatalogID, brokerName string) error {
	p.logger.Println("Disabling service access for all plans")
	err := p.cfClient.DisableServiceAccessForAllPlans(serviceCatalogID, p.logger)
	if err != nil {
		return fmt.Errorf(errorMessageTemplate, err.Error())
	}

	p.logger.Println("Deleting all service instances")
	err = p.deleter.DeleteAllServiceInstances(serviceCatalogID)
	if err != nil {
		return fmt.Errorf(errorMessageTemplate, err.Error())
	}

	p.logger.Println("Deregistering service brokers")
	err = p.deregistrar.Deregister(brokerName)
	if err != nil {
		return fmt.Errorf(errorMessageTemplate, err.Error())
	}
	return nil
}
