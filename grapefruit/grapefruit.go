// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package grapefruit

import (
	"context"
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
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
	"github.com/urfave/negroni"
)

type GrapefruitBroker interface {
	mgmtapi.ManageableBroker
	brokerapi.ServiceBroker
}

type Grapefruit struct {
	broker        GrapefruitBroker
	componentName string
}

func New(broker GrapefruitBroker, componentName string) *Grapefruit {
	return &Grapefruit{broker: broker, componentName: componentName}
}

func (g *Grapefruit) StartupServer(conf config.Config, loggerFactory *loggerfactory.LoggerFactory, logger *log.Logger) {

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

	server := setupServer(g.broker, conf, loggerFactory, logger, g.componentName)

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

	err := server.ListenAndServe()
	if err != http.ErrServerClosed {
		logger.Fatalf("Error listening and serving: %v\n", err)
	}

	<-stopped
}

func setupServer(
	broker GrapefruitBroker,
	conf config.Config,
	loggerFactory *loggerfactory.LoggerFactory,
	logger *log.Logger,
	componentName string,
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
