// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package main

import (
	"crypto/x509"
	"flag"
	"io/ioutil"
	"os"
	"time"

	"github.com/craigfurman/herottp"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/network"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader"
	yaml "gopkg.in/yaml.v2"
)

func main() {
	loggerFactory := loggerfactory.New(os.Stdout, "upgrade-all-service-instances", loggerfactory.Flags)
	logger := loggerFactory.New()

	var configPath string
	flag.StringVar(&configPath, "configPath", "", "path to upgrade-all-service-instances config")
	flag.Parse()

	if configPath == "" {
		logger.Fatalln("-configPath must be given as argument")
	}

	var conf config.UpgradeAllInstanceErrandConfig
	configContents, err := ioutil.ReadFile(configPath)
	if err != nil {
		logger.Fatalln(err.Error())
	}

	err = yaml.Unmarshal(configContents, &conf)
	if err != nil {
		logger.Fatalln(err.Error())
	}

	if conf.BrokerAPI.Authentication.Basic.Username == "" ||
		conf.BrokerAPI.Authentication.Basic.Password == "" ||
		conf.BrokerAPI.URL == "" {
		logger.Fatalln("the brokerUsername, brokerPassword and brokerUrl are required to function")
	}

	if conf.PollingInterval <= 0 {
		logger.Fatalln("the pollingInterval must be greater than zero")
	}

	certPool, err := x509.SystemCertPool()
	if err != nil {
		logger.Fatalf("error getting a certificate pool to append our trusted cert to: %s", err)
	}
	cert := conf.ServiceInstancesAPI.RootCACert
	certPool.AppendCertsFromPEM([]byte(cert))

	httpClient := herottp.New(herottp.Config{
		Timeout: 30 * time.Second,
		RootCAs: certPool,
	})

	brokerBasicAuthHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(
		conf.BrokerAPI.Authentication.Basic.Username,
		conf.BrokerAPI.Authentication.Basic.Password,
	)

	brokerServices := services.NewBrokerServices(httpClient, brokerBasicAuthHeaderBuilder, conf.BrokerAPI.URL, logger)
	listener := upgrader.NewLoggingListener(logger)

	serviceInstancesAPIBasicAuthClient := network.NewBasicAuthHTTPClient(
		httpClient,
		conf.ServiceInstancesAPI.Authentication.Basic.Username,
		conf.ServiceInstancesAPI.Authentication.Basic.Password,
		conf.ServiceInstancesAPI.URL,
	)

	serviceInstancesServices := service.NewInstanceLister(serviceInstancesAPIBasicAuthClient)
	upgradeTool := upgrader.New(brokerServices, serviceInstancesServices, conf.PollingInterval, listener)

	err = upgradeTool.Upgrade()
	if err != nil {
		logger.Fatalln(err.Error())
	}
}
