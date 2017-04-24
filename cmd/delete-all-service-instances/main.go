// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package main

import (
	"flag"
	"io/ioutil"

	"os"

	"github.com/pivotal-cf/on-demand-service-broker/cloud_foundry_client"
	"github.com/pivotal-cf/on-demand-service-broker/deleter"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"gopkg.in/yaml.v2"
)

func main() {
	loggerFactory := loggerfactory.New(os.Stdout, "delete-all-service-instances", loggerfactory.Flags)
	logger := loggerFactory.New()

	configFilePath := flag.String("configFilePath", "", "path to config file")
	flag.Parse()

	rawConfig, err := ioutil.ReadFile(*configFilePath)
	if err != nil {
		logger.Fatalf("Error reading config file: %s", err)
	}

	var deleterConfig deleter.Config
	err = yaml.Unmarshal(rawConfig, &deleterConfig)
	if err != nil {
		logger.Fatalf("Invalid config file: %s", err)
	}

	cfAuthenticator, err := deleterConfig.CF.NewAuthHeaderBuilder(deleterConfig.DisableSSLCertVerification)
	if err != nil {
		logger.Fatalf("error creating CF authorization header builder: %s", err)
	}

	cfClient, err := cloud_foundry_client.New(
		deleterConfig.CF.URL,
		cfAuthenticator,
		[]byte(deleterConfig.CF.TrustedCert),
		deleterConfig.DisableSSLCertVerification,
	)
	if err != nil {
		logger.Fatalf("error creating Cloud Foundry client: %s", err)
	}

	deleteTool := deleter.New(cfClient, deleterConfig.PollingInterval, logger)
	err = deleteTool.DeleteAllServiceInstances(deleterConfig.ServiceCatalog.ID)
	if err != nil {
		logger.Fatalln(err)
	}

	logger.Println("FINISHED DELETES")
}
