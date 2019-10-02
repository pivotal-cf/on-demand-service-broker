// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package main

import (
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/craigfurman/herottp"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/runtimechecker"
	"gopkg.in/yaml.v2"
)

func main() {
	loggerFactory := loggerfactory.New(os.Stdout, "recreate-all-service-instances", loggerfactory.Flags)
	logger := loggerFactory.New()

	var configPath string
	flag.StringVar(&configPath, "configPath", "", "path to recreate-all-service-instances config")
	flag.Parse()

	if configPath == "" {
		logger.Fatalln("-configPath must be given as argument")
	}

	var conf config.InstanceIteratorConfig
	configContents, err := ioutil.ReadFile(configPath)
	if err != nil {
		logger.Fatalln(err.Error())
	}

	err = yaml.Unmarshal(configContents, &conf)
	if err != nil {
		logger.Fatalln(err.Error())
	}

	err = checkBoshVersion(conf)
	if err != nil {
		log.Fatal(err)
	}

	configurator, err := instanceiterator.NewConfigurator(conf, logger, "recreate-all")
	if err != nil {
		logger.Fatalln(err.Error())
	}
	configurator.SetRecreateTriggerer()

	recreateTool := instanceiterator.New(configurator)
	err = recreateTool.Iterate()
	if err != nil {
		logger.Fatalln(err.Error())
	}
}

func checkBoshVersion(conf config.InstanceIteratorConfig) error {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("error getting a certificate pool to append our trusted cert to: %s", err)
	}
	cert := conf.Bosh.TrustedCert
	certPool.AppendCertsFromPEM([]byte(cert))

	boshClient := herottp.New(herottp.Config{
		Timeout: 30 * time.Second,
		RootCAs: certPool,
	})

	var infoResp boshdirector.Info
	resp, err := boshClient.Get(conf.Bosh.URL + "/info")
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("an error occurred while talking to the BOSH director: got HTTP %d", resp.StatusCode)
	}

	err = json.NewDecoder(resp.Body).Decode(&infoResp)
	if err != nil {
		return fmt.Errorf("an error occurred while talking to the BOSH director: %s", err)
	}

	runtimeChecker := runtimechecker.RecreateRuntimeChecker{BoshInfo: infoResp}
	return runtimeChecker.Check()
}
