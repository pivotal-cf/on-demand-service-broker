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
	"os"

	"gopkg.in/yaml.v2"

	"flag"

	"io/ioutil"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/deregistrar"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

func main() {
	loggerFactory := loggerfactory.New(os.Stdout, "deregister-broker", loggerfactory.Flags)
	logger := loggerFactory.New()

	brokerName := flag.String("brokerName", "", "broker name")
	configFilePath := flag.String("configFilePath", "", "path to config file")
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

	var config deregistrar.Config
	err = yaml.Unmarshal(rawConfig, &config)
	if err != nil {
		logger.Fatalf("Invalid config file: %s", err)
	}

	cfAuthenticator, err := config.CF.NewAuthHeaderBuilder(config.DisableSSLCertVerification)
	if err != nil {
		logger.Fatalf("Error creating CF authorization header builder: %s", err)
	}

	cfClient, err := cf.New(
		config.CF.URL,
		cfAuthenticator,
		[]byte(config.CF.TrustedCert),
		config.DisableSSLCertVerification,
	)
	if err != nil {
		logger.Fatalf("Error creating Cloud Foundry client: %s", err)
	}

	deregistrarTool := deregistrar.New(cfClient, logger)
	err = deregistrarTool.Deregister(*brokerName)
	if err != nil {
		logger.Fatal(err.Error())
	}

	logger.Println("FINISHED DEREGISTER BROKER")
}
