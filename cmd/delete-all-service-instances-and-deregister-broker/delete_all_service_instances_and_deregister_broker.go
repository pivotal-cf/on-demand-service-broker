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

package main

import (
	"flag"
	"io/ioutil"
	"os"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/deleter"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/purger"
	"github.com/pivotal-cf/on-demand-service-broker/registrar"
	"github.com/pivotal-cf/on-demand-service-broker/tools"
	"gopkg.in/yaml.v2"
)

func main() {
	loggerFactory := loggerfactory.New(os.Stdout, "delete-all-service-instances-and-deregister-broker", loggerfactory.Flags)
	logger := loggerFactory.New()

	configFilePath := flag.String("configFilePath", "", "path to config file")
	brokerName := flag.String("brokerName", "", "broker name")
	flag.Parse()

	if *brokerName == "" {
		logger.Fatal("Missing argument -brokerName")
	}

	if *configFilePath == "" {
		logger.Fatal("Missing argument -configFilePath")
	}

	rawConfig, err := ioutil.ReadFile(*configFilePath)
	if err != nil {
		logger.Fatalf("Error reading config file: %s", err)
	}

	var config deleter.Config
	err = yaml.Unmarshal(rawConfig, &config)
	if err != nil {
		logger.Fatalf("Invalid config file: %s", err)
	}

	cfAuthenticator, err := config.CF.NewAuthHeaderBuilder(config.DisableSSLCertVerification)
	if err != nil {
		logger.Fatalf("Error creating CF authorization header builder: %s", err)
	}

	cfClient, err := cf.New(config.CF.URL, cfAuthenticator, []byte(config.CF.TrustedCert), config.DisableSSLCertVerification, logger)
	if err != nil {
		logger.Fatalf("Error creating Cloud Foundry client: %s", err)
	}

	clock := tools.RealSleeper{}

	deleteTool := deleter.New(cfClient, clock, config.PollingInitialOffset, config.PollingInterval, logger)

	registrarTool := registrar.New(cfClient, logger)

	purgerTool := purger.New(deleteTool, registrarTool, cfClient, logger)

	err = purgerTool.DeleteInstancesAndDeregister(config.ServiceCatalog.ID, *brokerName)
	if err != nil {
		logger.Fatalf(err.Error())
	}
	logger.Println("FINISHED PURGE INSTANCES AND DEREGISTER BROKER")
}
