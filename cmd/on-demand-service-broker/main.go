// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package main

import (
	"crypto/x509"
	"flag"
	"log"
	"os"

	"github.com/cloudfoundry/bosh-cli/v7/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/v7/uaa"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshlinks"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/brokerinitiator"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/noopservicescontroller"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

func main() {
	loggerFactory := loggerfactory.New(os.Stdout, broker.ComponentName, loggerfactory.Flags)
	logger := loggerFactory.New()
	logger.Println("Starting broker")

	config := configParser(logger)
	boshClient := createBoshClient(logger, config)
	commandRunner := serviceadapter.NewCommandRunner()
	stopServer := make(chan os.Signal, 1)
	cfClient := createCfClient(config, logger)

	brokerinitiator.Initiate(config, boshClient, boshClient, cfClient, commandRunner, stopServer, loggerFactory)
}

func configParser(logger *log.Logger) config.Config {
	configFilePath := flag.String("configFilePath", "", "path to config file")
	flag.Parse()
	if *configFilePath == "" {
		logger.Fatal("must supply -configFilePath")
	}
	config, err := config.Parse(*configFilePath)
	if err != nil {
		logger.Fatalf("error parsing config: %s", err)
	}
	return config
}

func createCfClient(conf config.Config, logger *log.Logger) broker.CloudFoundryClient {
	var cfClient broker.CloudFoundryClient
	if !conf.Broker.DisableCFStartupChecks {
		cfClient = createRealCfClient(conf, logger, cfClient)
	} else {
		cfClient = noopservicescontroller.New()
	}
	return cfClient
}

func createRealCfClient(conf config.Config, logger *log.Logger, cfClient broker.CloudFoundryClient) broker.CloudFoundryClient {
	cfAuthenticator, err := conf.CF.NewAuthHeaderBuilder(conf.Broker.DisableSSLCertVerification)
	if err != nil {
		logger.Fatalf("error creating CF authorization header builder: %s", err)
	}
	cfClient, err = cf.New(conf.CF.URL, cfAuthenticator, []byte(conf.CF.TrustedCert), conf.Broker.DisableSSLCertVerification, logger)
	if err != nil {
		logger.Fatalf("error creating Cloud Foundry client: %s", err)
	}
	return cfClient
}

func createBoshClient(logger *log.Logger, conf config.Config) *boshdirector.Client {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		logger.Fatalf("error getting a certificate pool to append our trusted cert to: %s", err)
	}
	boshLogger := boshlog.NewLogger(boshlog.LevelError)
	directorFactory := director.NewFactory(boshLogger)
	uaaFactory := boshuaa.NewFactory(boshLogger)
	boshClient, err := boshdirector.New(
		conf.Bosh.URL,
		[]byte(conf.Bosh.TrustedCert),
		certPool,
		directorFactory,
		uaaFactory,
		conf.Bosh.Authentication,
		boshlinks.NewDNSRetriever,
		boshdirector.NewBoshHTTP,
		logger)
	if err != nil {
		logger.Fatalf("error creating bosh client: %s", err)
	}
	return boshClient
}
