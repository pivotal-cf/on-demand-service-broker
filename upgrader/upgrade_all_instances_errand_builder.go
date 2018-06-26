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

package upgrader

import (
	"log"
	"time"

	"crypto/x509"
	"errors"
	"fmt"

	"strings"

	"github.com/craigfurman/herottp"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/tools"
)

type Builder struct {
	BrokerServices        BrokerServices
	ServiceInstanceLister InstanceLister
	PollingInterval       time.Duration
	AttemptInterval       time.Duration
	AttemptLimit          int
	MaxInFlight           int
	Canaries              int
	Listener              Listener
	Sleeper               sleeper
	CanarySelectionParams config.CanarySelectionParams
}

func NewBuilder(
	conf config.UpgradeAllInstanceErrandConfig,
	logger *log.Logger,
) (*Builder, error) {

	brokerServices, err := brokerServices(conf, logger)
	if err != nil {
		return nil, err
	}

	instanceLister, err := serviceInstanceLister(conf, logger)
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

	listener := NewLoggingListener(logger)

	b := &Builder{
		BrokerServices:        brokerServices,
		ServiceInstanceLister: instanceLister,
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

func brokerServices(conf config.UpgradeAllInstanceErrandConfig, logger *log.Logger) (*services.BrokerServices, error) {
	if conf.BrokerAPI.Authentication.Basic.Username == "" ||
		conf.BrokerAPI.Authentication.Basic.Password == "" ||
		conf.BrokerAPI.URL == "" {
		return &services.BrokerServices{}, errors.New("the brokerUsername, brokerPassword and brokerUrl are required to function")
	}

	brokerBasicAuthHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(
		conf.BrokerAPI.Authentication.Basic.Username,
		conf.BrokerAPI.Authentication.Basic.Password,
	)

	return services.NewBrokerServices(
		herottp.New(herottp.Config{
			Timeout:    time.Duration(conf.RequestTimeout) * time.Second,
			MaxRetries: 5,
		}),
		brokerBasicAuthHeaderBuilder,
		conf.BrokerAPI.URL,
		logger,
	), nil
}

func serviceInstanceLister(conf config.UpgradeAllInstanceErrandConfig, logger *log.Logger) (*service.ServiceInstanceLister, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return &service.ServiceInstanceLister{},
			fmt.Errorf("error getting a certificate pool to append our trusted cert to: %s", err)
	}
	cert := conf.ServiceInstancesAPI.RootCACert
	certPool.AppendCertsFromPEM([]byte(cert))

	httpClient := herottp.New(herottp.Config{
		Timeout: 30 * time.Second,
		RootCAs: certPool,
	})

	manuallyConfigured := !strings.Contains(conf.ServiceInstancesAPI.URL, conf.BrokerAPI.URL)
	authHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(
		conf.ServiceInstancesAPI.Authentication.Basic.Username,
		conf.ServiceInstancesAPI.Authentication.Basic.Password,
	)
	return service.NewInstanceLister(
		httpClient,
		authHeaderBuilder,
		conf.ServiceInstancesAPI.URL,
		manuallyConfigured,
		logger,
	), nil
}

func pollingInterval(conf config.UpgradeAllInstanceErrandConfig) (time.Duration, error) {
	if conf.PollingInterval <= 0 {
		return 0, errors.New("the pollingInterval must be greater than zero")
	}
	return time.Duration(conf.PollingInterval) * time.Second, nil
}

func attemptInterval(conf config.UpgradeAllInstanceErrandConfig) (time.Duration, error) {
	if conf.AttemptInterval <= 0 {
		return 0, errors.New("the attemptInterval must be greater than zero")
	}
	return time.Duration(conf.AttemptInterval) * time.Second, nil
}

func attemptLimit(conf config.UpgradeAllInstanceErrandConfig) (int, error) {
	if conf.AttemptLimit <= 0 {
		return 0, errors.New("the attempt limit must be greater than zero")
	}
	return conf.AttemptLimit, nil
}

func maxInFlight(conf config.UpgradeAllInstanceErrandConfig) (int, error) {
	if conf.MaxInFlight <= 0 {
		return 0, errors.New("the max in flight must be greater than zero")
	}
	return conf.MaxInFlight, nil
}

func canaries(conf config.UpgradeAllInstanceErrandConfig) (int, error) {
	if conf.Canaries < 0 {
		return 0, errors.New("the number of canaries cannot be negative")
	}
	return conf.Canaries, nil
}

func canarySelectionParams(conf config.UpgradeAllInstanceErrandConfig) (config.CanarySelectionParams, error) {
	return conf.CanarySelectionParams, nil
}
