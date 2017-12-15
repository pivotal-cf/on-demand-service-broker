// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package main

import (
	"context"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/craigfurman/herottp"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/brokeraugmenter"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
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
		conf.Broker.DisableSSLCertVerification,
		[]byte(conf.Bosh.TrustedCert),
		herottp.New(herottp.Config{
			NoFollowRedirect:                  true,
			DisableTLSCertificateVerification: conf.Broker.DisableSSLCertVerification,
			RootCAs: certPool,
			Timeout: 30 * time.Second,
		}),
		conf.Bosh,
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

	onDemandBroker, err := broker.New(boshClient, cfClient, conf.ServiceCatalog, startupChecks, serviceAdapter, deploymentManager, loggerFactory)
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
	broker, err := brokeraugmenter.New(conf, onDemandBroker, credhubFactory, loggerFactory)
	if err != nil {
		logger.Fatalf("Error constructing the CredHub broker: %s", err)
	}
	server := apiserver.New(
		conf,
		broker,
		componentName,
		loggerFactory,
		logger,
	)

	stopped := make(chan struct{})
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop

		timeoutSecs := conf.Broker.ShutdownTimeoutSecs
		logger.Printf("Broker shutting down on signal (timeout %d secs)...\n", timeoutSecs)

		ctx, cancel := context.WithTimeout(
			context.Background(),
			time.Second*time.Duration(timeoutSecs),
		)
		defer cancel()

		if err = server.Shutdown(ctx); err != nil {
			logger.Printf("Error gracefully shutting down server: %v\n", err)
		} else {
			logger.Println("Server gracefully shut down")
		}

		close(stopped)
	}()

	logger.Println("Listening on", server.Addr)

	err = server.ListenAndServe()
	if err != http.ErrServerClosed {
		logger.Fatalf("Error listening and serving: %v\n", err)
	}

	<-stopped
}
