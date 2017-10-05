// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"time"

	"code.cloudfoundry.org/lager"
	"github.com/gorilla/mux"
	"github.com/pivotal-cf/brokerapi"
	apiauth "github.com/pivotal-cf/brokerapi/auth"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/task"
	"github.com/urfave/negroni"
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
	var (
		boshAuthenticator boshdirector.AuthHeaderBuilder
		err               error
	)

	noAuthHeaderBuilder := authorizationheader.NewNoAuthHeaderBuilder()
	unauthenticatedClient, err := boshdirector.New(conf.Bosh.URL, noAuthHeaderBuilder, conf.Broker.DisableSSLCertVerification, []byte(conf.Bosh.TrustedCert))
	boshInfo, err := unauthenticatedClient.GetInfo(logger)
	if err != nil {
		logger.Fatalf("error fetching BOSH director information: %s", err)
	}

	boshAuthenticator, err = conf.Bosh.NewAuthHeaderBuilder(boshInfo, conf.Broker.DisableSSLCertVerification)
	if err != nil {
		logger.Fatalf("error creating BOSH authorization header builder: %s", err)
	}

	boshClient, err := boshdirector.New(conf.Bosh.URL, boshAuthenticator, conf.Broker.DisableSSLCertVerification, []byte(conf.Bosh.TrustedCert))
	if err != nil {
		logger.Fatalf("error creating bosh client: %s", err)
	}

	cfAuthenticator, err := conf.CF.NewAuthHeaderBuilder(conf.Broker.DisableSSLCertVerification)
	if err != nil {
		logger.Fatalf("error creating CF authorization header builder: %s", err)
	}

	cfClient, err := cf.New(
		conf.CF.URL,
		cfAuthenticator,
		[]byte(conf.CF.TrustedCert),
		conf.Broker.DisableSSLCertVerification,
	)
	if err != nil {
		logger.Fatalf("error creating Cloud Foundry client: %s", err)
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

	onDemandBroker, err := broker.New(boshInfo, boshClient, cfClient, serviceAdapter, deploymentManager, conf.ServiceCatalog, conf.Broker.DisableCFStartupChecks, loggerFactory)
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

	server := setupServer(onDemandBroker, conf, loggerFactory, logger)

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

		if err := server.Shutdown(ctx); err != nil {
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

func setupServer(
	broker *broker.Broker,
	conf config.Config,
	loggerFactory *loggerfactory.LoggerFactory,
	logger *log.Logger,
) *http.Server {

	brokerRouter := mux.NewRouter()
	mgmtapi.AttachRoutes(brokerRouter, broker, conf.ServiceCatalog, loggerFactory)
	brokerapi.AttachRoutes(brokerRouter, broker, lager.NewLogger(componentName))
	authProtectedBrokerAPI := apiauth.
		NewWrapper(conf.Broker.Username, conf.Broker.Password).
		Wrap(brokerRouter)

	negroniLogger := &negroni.Logger{ALogger: logger}
	server := negroni.New(
		negroni.NewRecovery(),
		negroniLogger,
		negroni.NewStatic(http.Dir("public")),
	)

	server.UseHandler(authProtectedBrokerAPI)
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", conf.Broker.Port),
		Handler: server,
	}
}
