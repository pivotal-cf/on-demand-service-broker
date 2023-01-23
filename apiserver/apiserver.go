// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package apiserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/gorilla/mux"
	brokerapi "github.com/pivotal-cf/brokerapi/v8"
	apiauth "github.com/pivotal-cf/brokerapi/v8/auth"
	"github.com/pivotal-cf/brokerapi/v8/domain"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
	"github.com/pkg/errors"
	"github.com/urfave/negroni"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate -o fakes/combined_broker.go . CombinedBroker
type CombinedBroker interface {
	mgmtapi.ManageableBroker
	domain.ServiceBroker
	SetUAAClient(uaaClient broker.UAAClient)
}

func New(
	conf config.Config,
	broker CombinedBroker,
	componentName string,
	mgmtapiLoggerFactory *loggerfactory.LoggerFactory,
	serverLogger *log.Logger,
) *http.Server {

	router := mux.NewRouter()
	registerManagementAPI(broker, conf, mgmtapiLoggerFactory, router)
	registerOSBAPI(broker, componentName, serverLogger, conf, router)

	server := negroni.New(
		negroni.NewRecovery(),
		createNegroniLogger(serverLogger),
		negroni.NewStatic(http.Dir("public")),
		negroni.Wrap(router),
	)

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", conf.Broker.Port),
		Handler: server,
	}
}

func registerOSBAPI(
	broker CombinedBroker,
	componentName string,
	serverLogger *log.Logger,
	conf config.Config,
	router *mux.Router,
) {
	apiBrokerHandler := brokerapi.New(
		broker,
		createBrokerAPILogger(componentName, serverLogger),
		brokerapi.BrokerCredentials{
			Username: conf.Broker.Username,
			Password: conf.Broker.Password,
		})

	router.PathPrefix("/v2").Handler(apiBrokerHandler)
}

func registerManagementAPI(
	broker CombinedBroker,
	conf config.Config,
	mgmtapiLoggerFactory *loggerfactory.LoggerFactory,
	router *mux.Router,
) {
	mgmtAPIRouter := mux.NewRouter()
	mgmtapi.AttachRoutes(mgmtAPIRouter, broker, conf.ServiceCatalog, mgmtapiLoggerFactory)
	authMiddleware := apiauth.NewWrapper(conf.Broker.Username, conf.Broker.Password).Wrap
	mgmtAPIRouter.Use(authMiddleware)

	router.PathPrefix("/mgmt").Handler(mgmtAPIRouter)
}

func StartAndWait(conf config.Config, server *http.Server, logger *log.Logger, stopServer chan os.Signal) error {
	stopped := make(chan struct{})
	signal.Notify(stopServer, os.Interrupt, syscall.SIGTERM)

	go handleBrokerTerminationSignal(stopServer, conf, logger, server, stopped)

	logger.Println("Listening on", server.Addr)
	var err error

	if conf.HasTLS() {
		if err = CheckCertExpiry(conf.Broker.TLS.CertFile); err != nil {
			return err
		}
		acceptableCipherSuites := []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		}
		tlsConfig := tls.Config{
			CipherSuites: acceptableCipherSuites,
			MinVersion:   tls.VersionTLS12,
		}
		server.TLSConfig = &tlsConfig
		err = server.ListenAndServeTLS(conf.Broker.TLS.CertFile, conf.Broker.TLS.KeyFile)
	} else {
		err = server.ListenAndServe()
	}

	if err != http.ErrServerClosed {
		return errors.Wrap(err, "error starting broker HTTP(s) server")
	}

	<-stopped
	return nil
}

func handleBrokerTerminationSignal(stopServer chan os.Signal, conf config.Config, logger *log.Logger, server *http.Server, stopped chan struct{}) {
	<-stopServer

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
}

func CheckCertExpiry(certFile string) error {
	certBytes, err := ioutil.ReadFile(certFile)
	if err != nil {
		return errors.Wrap(err, "can't read server certificate file "+certFile)
	}

	pemBlock, _ := pem.Decode(certBytes)
	if pemBlock == nil {
		return fmt.Errorf("failed to find any PEM data in certificate input from: %q", certFile)
	}

	cert, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return errors.Wrap(err, "can't parse server certificate file "+certFile)
	}

	if time.Now().After(cert.NotAfter) {
		return fmt.Errorf("server certificate expired on %v", cert.NotAfter)
	}
	return nil
}

func createNegroniLogger(serverLogger *log.Logger) *negroni.Logger {
	dateFormat := "2006/01/02 15:04:05.000000"
	logFormat := "Request {{.Method}} {{.Path}} Completed {{.Status}} in {{.Duration}} | Start Time: {{.StartTime}}"
	negroniLogger := negroni.NewLogger()
	negroniLogger.ALogger = serverLogger
	negroniLogger.SetFormat(logFormat)
	negroniLogger.SetDateFormat(dateFormat)
	return negroniLogger
}

func createBrokerAPILogger(componentName string, serverLogger *log.Logger) lager.Logger {
	brokerAPILogger := lager.NewLogger(componentName)
	brokerAPILogger.RegisterSink(lager.NewWriterSink(serverLogger.Writer(), lager.INFO))
	return brokerAPILogger
}
