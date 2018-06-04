// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package main

import (
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/brokeraugmenter"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	"github.com/pivotal-cf/on-demand-service-broker/credstore"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/manifestsecrets"
	"github.com/pivotal-cf/on-demand-service-broker/noopservicescontroller"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/startupchecker"
	"github.com/pivotal-cf/on-demand-service-broker/task"
)

const componentName = "on-demand-service-broker"

func main() {
	loggerFactory := loggerfactory.New(os.Stdout, componentName, loggerfactory.Flags)

	logger := loggerFactory.New()
	logger.Println("Starting broker")

	configFilePath := flag.String("configFilePath", "", "path to config file")

	flag.Parse()
	if *configFilePath == "" {
		logger.Fatal("must supply -configFilePath")
	}

	conf, err := config.Parse(*configFilePath)
	if err != nil {
		logger.Fatalf("error parsing config: %s", err)
	}

	startBroker(conf, logger, loggerFactory)
}

func startBroker(conf config.Config, logger *log.Logger, loggerFactory *loggerfactory.LoggerFactory) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		logger.Fatalf("error getting a certificate pool to append our trusted cert to: %s", err)
	}

	l := boshlog.NewLogger(boshlog.LevelError)
	directorFactory := director.NewFactory(l)
	uaaFactory := boshuaa.NewFactory(l)

	boshClient, err := boshdirector.New(
		conf.Bosh.URL,
		[]byte(conf.Bosh.TrustedCert),
		certPool,
		directorFactory,
		uaaFactory,
		conf.Bosh.Authentication,
		logger)
	if err != nil {
		logger.Fatalf("error creating bosh client: %s", err)
	}

	cfAuthenticator, err := conf.CF.NewAuthHeaderBuilder(conf.Broker.DisableSSLCertVerification)
	if err != nil {
		logger.Fatalf("error creating CF authorization header builder: %s", err)
	}

	var cfClient broker.CloudFoundryClient

	var startupChecks []broker.StartupChecker

	if !conf.Broker.DisableCFStartupChecks {
		cfClient, err = cf.New(
			conf.CF.URL,
			cfAuthenticator,
			[]byte(conf.CF.TrustedCert),
			conf.Broker.DisableSSLCertVerification,
		)
		if err != nil {
			logger.Fatalf("error creating Cloud Foundry client: %s", err)
		}
		startupChecks = append(
			startupChecks,
			startupchecker.NewCFAPIVersionChecker(cfClient, broker.MinimumCFVersion, logger),
			startupchecker.NewCFPlanConsistencyChecker(cfClient, conf.ServiceCatalog, logger),
		)
	} else {
		cfClient = noopservicescontroller.New()
	}

	serviceAdapter := &serviceadapter.Client{
		ExternalBinPath: conf.ServiceAdapter.Path,
		CommandRunner:   serviceadapter.NewCommandRunner(),
		UsingStdin:      conf.Broker.UsingStdin,
	}

	manifestGenerator := task.NewManifestGenerator(
		serviceAdapter,
		conf.ServiceCatalog,
		conf.ServiceDeployment.Stemcell,
		conf.ServiceDeployment.Releases,
	)

	deploymentManager := task.NewDeployer(boshClient, manifestGenerator)

	startupChecks = append(startupChecks,
		startupchecker.NewBOSHDirectorVersionChecker(
			broker.MinimumMajorStemcellDirectorVersionForODB,
			broker.MinimumMajorSemverDirectorVersionForLifecycleErrands,
			boshClient.BoshInfo,
			conf.ServiceCatalog,
		),
		startupchecker.NewBOSHAuthChecker(boshClient, logger),
	)

	matcher := new(manifestsecrets.CredHubPathMatcher)
	var boshCredhubClient *credhub.CredHub
	if conf.Broker.ResolveManifestSecretsAtBind {
		boshCredhubClient, err = credhub.New(
			conf.BoshCredhub.URL,
			credhub.Auth(auth.UaaClientCredentials(
				conf.BoshCredhub.Authentication.UAA.ClientCredentials.ID,
				conf.BoshCredhub.Authentication.UAA.ClientCredentials.Secret,
			)),
			credhub.CaCerts(conf.BoshCredhub.RootCACert, conf.Bosh.TrustedCert),
		)
		if err != nil {
			logger.Fatalf("error starting broker: %s", err)
		}
	}
	bulkGetter := credstore.New(boshCredhubClient)
	manifestSecretResolver := manifestsecrets.NewResolver(conf.Broker.ResolveManifestSecretsAtBind, matcher, bulkGetter)

	onDemandBroker, err := broker.New(
		boshClient,
		cfClient,
		conf.ServiceCatalog,
		conf.Broker,
		startupChecks,
		serviceAdapter,
		deploymentManager,
		manifestSecretResolver,
		loggerFactory,
	)
	if err != nil {
		logger.Fatalf("error starting broker: %s", err)
	}

	if conf.Broker.StartUpBanner {
		fmt.Println(`
                  .//\
        \\      .+ssso/\     \\
      \---.\  .+ssssssso/.  \----\         ____     ______     ______
    .--------+ssssssssssso+--------\      / __ \   (_  __ \   (_   _ \
  .-------:+ssssssssssssssss+--------\   / /  \ \    ) ) \ \    ) (_) )
 -------./ssssssssssssssssssss:.------- ( ()  () )  ( (   ) )   \   _/
  \--------+ssssssssssssssso+--------/  ( ()  () )   ) )  ) )   /  _ \
    \-------.+osssssssssso/.-------/     \ \__/ /   / /__/ /   _) (_) )
      \---./  ./osssssso/   \.---/        \____/   (______/   (______/
        \/      \/osso/       \/
                  \/:/
								`)
	}

	credhubFactory := credhubbroker.CredhubFactory{Conf: conf}
	combinedBroker, err := brokeraugmenter.New(conf, onDemandBroker, credhubFactory, loggerFactory)
	if err != nil {
		logger.Fatalf("Error constructing the CredHub broker: %s", err)
	}
	server := apiserver.New(
		conf,
		combinedBroker,
		componentName,
		loggerFactory,
		logger,
	)

	stopServer := make(chan os.Signal, 1)
	apiserver.StartAndWait(conf, server, logger, stopServer)
}
