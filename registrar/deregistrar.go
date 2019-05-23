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

package registrar

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/config"
)

type Deregistrar struct {
	cfClient CloudFoundryClient
	logger   *log.Logger
}

type Config struct {
	DisableSSLCertVerification bool `yaml:"disable_ssl_cert_verification"`
	CF                         config.CF
}

//go:generate counterfeiter -o fakes/fake_cloud_foundry_client.go . CloudFoundryClient
type CloudFoundryClient interface {
	GetServiceOfferingGUID(string, *log.Logger) (string, error)
	DeregisterBroker(string, *log.Logger) error
}

func New(client CloudFoundryClient, logger *log.Logger) *Deregistrar {
	return &Deregistrar{
		cfClient: client,
		logger:   logger,
	}
}

func (r *Deregistrar) Deregister(brokerName string) error {
	var brokerGUID string

	brokerGUID, err := r.cfClient.GetServiceOfferingGUID(brokerName, r.logger)
	if err != nil {
		return err
	}

	if brokerGUID != "" {
		err = r.cfClient.DeregisterBroker(brokerGUID, r.logger)
		if err != nil {
			return fmt.Errorf("Failed to deregister broker with %s with guid %s, err: %s", brokerName, brokerGUID, err.Error())
		}
	}
	return nil
}
