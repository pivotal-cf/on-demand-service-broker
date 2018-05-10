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

package brokeraugmenter

import (
	"strings"
	"time"

	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

func New(conf config.Config,
	baseBroker apiserver.CombinedBroker,
	credhubFactory credhubbroker.CredentialStoreFactory,
	loggerFactory *loggerfactory.LoggerFactory) (apiserver.CombinedBroker, error) {

	if !conf.HasCredHub() {
		return baseBroker, nil
	}

	var credhubStore credhubbroker.CredentialStore
	var err error

	waitMillis := 16
	retryLimit := 10

	// if consul hasn't started yet, wait until internal DNS begins to work
	for retries := 0; retries < retryLimit; retries++ {
		credhubStore, err = credhubFactory.New()
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "no such host") {
			time.Sleep(time.Duration(waitMillis) * time.Millisecond)
			waitMillis *= 2
		} else {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	return credhubbroker.New(baseBroker, credhubStore, conf.ServiceCatalog.Name, loggerFactory), nil
}
