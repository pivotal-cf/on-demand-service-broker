// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/startupchecker"
)

const MinimumCFVersion string = "2.57.0"

func (b *Broker) startupChecks() error {
	logger := b.loggerFactory.New()

	startupErrors := []string{}

	cfChecker := startupchecker.NewCFChecker(b.cfClient, MinimumCFVersion, b.serviceOffering, logger)
	err := cfChecker.Check()
	if err != nil {
		startupErrors = append(startupErrors, err.Error())
	}

	if err = b.checkBoshDirectorVersion(logger); err != nil {
		startupErrors = append(startupErrors, "BOSH Director error: "+err.Error())
	}

	if len(startupErrors) > 0 {
		return errors.New(strings.Join(startupErrors, " "))
	}

	if err := b.checkAuthentication(logger); err != nil {
		return err
	}

	return nil
}

func (b *Broker) checkAuthentication(logger *log.Logger) error {
	if err := b.boshClient.VerifyAuth(logger); err != nil {
		return errors.New("BOSH Director error: " + err.Error())
	}
	return nil
}

func (b *Broker) checkBoshDirectorVersion(logger *log.Logger) error {
	directorVersion, err := b.boshInfo.GetDirectorVersion(logger)
	if err != nil {
		return fmt.Errorf("%s. ODB requires BOSH v257+.", err)
	}

	if !directorVersion.SupportsODB() {
		return errors.New("API version is insufficient, ODB requires BOSH v257+.")
	}

	if b.serviceOffering.HasLifecycleErrands() && !directorVersion.SupportsLifecycleErrands() {
		errMsg := fmt.Sprintf("API version is insufficient, one or more plans are configured with lifecycle_errands which require BOSH v%d+.", boshdirector.MinimumMajorSemverDirectorVersionForLifecycleErrands)
		return errors.New(errMsg)
	}

	return nil
}
