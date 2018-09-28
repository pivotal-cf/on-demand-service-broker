// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/craigfurman/herottp"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	yaml "gopkg.in/yaml.v2"
)

const (
	OrphanBoshDeploymentsDetectedMessage = "Orphan BOSH deployments detected with no corresponding service instance in Cloud Foundry. Before deleting any deployment it is recommended to verify the service instance no longer exists in Cloud Foundry and any data is safe to delete."
	OrphanDeploymentsDetectedExitCode    = 10
)

func main() {
	loggerFactory := loggerfactory.New(os.Stderr, "orphan-deployments", loggerfactory.Flags)
	logger := loggerFactory.New()

	var configPath string
	flag.StringVar(&configPath, "configPath", "", "path to orphan-deployment errand config")
	flag.Parse()

	if configPath == "" {
		logger.Fatalln("-configPath must be given as argument")
	}

	contents, err := ioutil.ReadFile(configPath)
	if err != nil {
		logger.Fatalln(err.Error())
	}

	var errandConfig config.OrphanDeploymentsErrandConfig
	if err := yaml.Unmarshal(contents, &errandConfig); err != nil {
		logger.Fatalf("failed to unmarshal errand config: %s\n", err.Error())
	}

	httpClient := herottp.New(herottp.Config{
		Timeout: 30 * time.Second,
	})

	brokerUsername := errandConfig.BrokerAPI.Authentication.Basic.Username
	brokerPassword := errandConfig.BrokerAPI.Authentication.Basic.Password

	authHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(brokerUsername, brokerPassword)
	brokerServices := services.NewBrokerServices(httpClient, authHeaderBuilder, errandConfig.BrokerAPI.URL, logger)

	orphans, err := brokerServices.OrphanDeployments()
	if err != nil {
		logger.Fatalf("error retrieving orphan deployments: %s", err)
	}

	rawJSON, err := json.Marshal(orphans)
	if err != nil {
		logger.Fatalf("error marshalling orphan deployments: %s", err)
	}

	fmt.Fprintln(os.Stdout, string(rawJSON))

	if len(orphans) > 0 {
		logger.Println(OrphanBoshDeploymentsDetectedMessage)
		os.Exit(OrphanDeploymentsDetectedExitCode)
	}
}
