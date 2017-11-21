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
	"os"

	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/network"
)

const (
	OrphanBoshDeploymentsDetectedMessage = "Orphan BOSH deployments detected with no corresponding service instance in Cloud Foundry. Before deleting any deployment it is recommended to verify the service instance no longer exists in Cloud Foundry and any data is safe to delete."
	OrphanDeploymentsDetectedExitCode    = 10
)

func main() {
	loggerFactory := loggerfactory.New(os.Stderr, "orphan-deployments", loggerfactory.Flags)
	logger := loggerFactory.New()

	brokerUsername := flag.String("brokerUsername", "", "username for the broker")
	brokerPassword := flag.String("brokerPassword", "", "password for the broker")
	brokerURL := flag.String("brokerUrl", "", "url of the broker")
	flag.Parse()

	httpClient := network.NewDefaultHTTPClient()
	authHeaderBuilder := authorizationheader.NewBasicAuthHeaderBuilder(*brokerUsername, *brokerPassword)
	brokerServices := services.NewBrokerServices(httpClient, authHeaderBuilder, *brokerURL, logger)

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
