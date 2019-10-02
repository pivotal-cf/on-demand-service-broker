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

package instanceiterator

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/craigfurman/herottp"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/network"
	"github.com/pivotal-cf/on-demand-service-broker/tools"
)

type Configurator struct {
	BrokerServices        BrokerServices
	PollingInterval       time.Duration
	AttemptInterval       time.Duration
	AttemptLimit          int
	MaxInFlight           int
	Canaries              int
	Listener              Listener
	Sleeper               sleeper
	Triggerer             Triggerer
	CanarySelectionParams config.CanarySelectionParams
}

func NewConfigurator(conf config.InstanceIteratorConfig, logger *log.Logger, logPrefix string) (*Configurator, error) {

	brokerServices, err := brokerServices(conf, logger)
	if err != nil {
		return nil, err
	}

	pollingInterval, err := pollingInterval(conf)
	if err != nil {
		return nil, err
	}

	attemptInterval, err := attemptInterval(conf)
	if err != nil {
		return nil, err
	}

	attemptLimit, err := attemptLimit(conf)
	if err != nil {
		return nil, err
	}

	maxInFlight, err := maxInFlight(conf)
	if err != nil {
		return nil, err
	}

	canaries, err := canaries(conf)
	if err != nil {
		return nil, err
	}

	canarySelectionParams, err := canarySelectionParams(conf)
	if err != nil {
		return nil, err
	}

	listener := NewLoggingListener(logger, logPrefix)

	b := &Configurator{
		BrokerServices:        brokerServices,
		PollingInterval:       pollingInterval,
		AttemptInterval:       attemptInterval,
		AttemptLimit:          attemptLimit,
		MaxInFlight:           maxInFlight,
		Canaries:              canaries,
		Listener:              listener,
		Sleeper:               &tools.RealSleeper{},
		CanarySelectionParams: canarySelectionParams,
	}

	return b, nil
}

func (b *Configurator) SetUpgradeTriggerer(cfClient CFClient, maintenanceInfoPresent bool, logger *log.Logger) error {
	if maintenanceInfoPresent && cfClient != nil && cfClient.CheckMinimumOSBAPIVersion("2.15", logger) {
		b.Listener.UpgradeStrategy("CF")
		b.Triggerer = NewCFTrigger(cfClient, logger)
		return nil
	}

	if b.BrokerServices == nil {
		return errors.New("unable to set triggerer, brokerServices must not be nil")
	}

	b.Listener.UpgradeStrategy("BOSH")
	b.Triggerer = NewBOSHUpgradeTriggerer(b.BrokerServices)
	return nil
}

func (b *Configurator) SetRecreateTriggerer() error {
	if b.BrokerServices == nil {
		return errors.New("unable to set triggerer, brokerServices must not be nil")
	}
	b.Triggerer = NewRecreateTriggerer(b.BrokerServices)
	return nil
}

func brokerServices(conf config.InstanceIteratorConfig, logger *log.Logger) (*services.BrokerServices, error) {
	if conf.BrokerAPI.Authentication.Basic.Username == "" ||
		conf.BrokerAPI.Authentication.Basic.Password == "" ||
		conf.BrokerAPI.URL == "" {
		return &services.BrokerServices{}, errors.New("the brokerUsername, brokerPassword and brokerUrl are required to function")
	}

	brokerBasicAuthHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(
		conf.BrokerAPI.Authentication.Basic.Username,
		conf.BrokerAPI.Authentication.Basic.Password,
	)

	certPool, err := network.AppendCertsFromPEM(conf.BrokerAPI.TLS.CACert)
	if err != nil {
		return &services.BrokerServices{},
			fmt.Errorf("error getting a certificate pool to append our trusted cert to: %s", err)
	}

	return services.NewBrokerServices(
		herottp.New(herottp.Config{
			Timeout:                           time.Duration(conf.RequestTimeout) * time.Second,
			RootCAs:                           certPool,
			DisableTLSCertificateVerification: conf.BrokerAPI.TLS.DisableSSLCertVerification,
			MaxRetries:                        5,
		}),
		brokerBasicAuthHeaderBuilder,
		conf.BrokerAPI.URL,
		logger,
	), nil
}

func pollingInterval(conf config.InstanceIteratorConfig) (time.Duration, error) {
	if conf.PollingInterval <= 0 {
		return 0, errors.New("the pollingInterval must be greater than zero")
	}
	return time.Duration(conf.PollingInterval) * time.Second, nil
}

func attemptInterval(conf config.InstanceIteratorConfig) (time.Duration, error) {
	if conf.AttemptInterval <= 0 {
		return 0, errors.New("the attemptInterval must be greater than zero")
	}
	return time.Duration(conf.AttemptInterval) * time.Second, nil
}

func attemptLimit(conf config.InstanceIteratorConfig) (int, error) {
	if conf.AttemptLimit <= 0 {
		return 0, errors.New("the attempt limit must be greater than zero")
	}
	return conf.AttemptLimit, nil
}

func maxInFlight(conf config.InstanceIteratorConfig) (int, error) {
	if conf.MaxInFlight <= 0 {
		return 0, errors.New("the max in flight must be greater than zero")
	}
	return conf.MaxInFlight, nil
}

func canaries(conf config.InstanceIteratorConfig) (int, error) {
	if conf.Canaries < 0 {
		return 0, errors.New("the number of canaries cannot be negative")
	}
	return conf.Canaries, nil
}

func canarySelectionParams(conf config.InstanceIteratorConfig) (config.CanarySelectionParams, error) {
	return conf.CanarySelectionParams, nil
}
