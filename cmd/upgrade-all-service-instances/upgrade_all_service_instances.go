// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
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

	var errandConfig config.InstanceIteratorConfig
	configContents, err := ioutil.ReadFile(configPath)
	if err != nil {
		logger.Fatalln(err.Error())
	}

	if err := yaml.Unmarshal(configContents, &errandConfig); err != nil {
		logger.Fatalln(err.Error())
	}

	configurator, err := instanceiterator.NewConfigurator(errandConfig, logger, "upgrade-all")
	if err != nil {
		logger.Fatalln(err.Error())
	}

	cfClient := createCFClient(errandConfig, logger)

	if instanceiterator.CanUpgradeUsingCF(cfClient, errandConfig.MaintenanceInfoPresent, logger) {
		configurator.SetUpgradeTriggererToCF(cfClient, logger)

		if err := instanceiterator.New(configurator).Iterate(); err != nil {
			logger.Fatalln(err.Error())
		}
	}

	configurator.SetUpgradeTriggererToBOSH()

	if err := instanceiterator.New(configurator).Iterate(); err != nil {
		logger.Fatalln(err.Error())
	}
}

func createCFClient(errandConfig config.InstanceIteratorConfig, logger *log.Logger) instanceiterator.CFClient {
	if errandConfig.CF != (config.CF{}) {
		cfAuthenticator, err := errandConfig.CF.NewAuthHeaderBuilder(errandConfig.CF.DisableSSLCertVerification)
		if err != nil {
			logger.Printf("Error creating CF authorization header builder: %s", err)
			return nil
		}
		cfClient, err := cf.New(errandConfig.CF.URL, cfAuthenticator, []byte(errandConfig.CF.TrustedCert), errandConfig.CF.DisableSSLCertVerification, logger)
		if err != nil {
			logger.Printf("Error creating Cloud Foundry client: %s", err)
			return nil
		}
		return cfClient
	}
	return nil
}
