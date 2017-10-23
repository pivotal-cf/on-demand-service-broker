// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"errors"
	"strings"

	"github.com/pivotal-cf/on-demand-service-broker/startupchecker"
)

const (
	MinimumCFVersion                                     string = "2.57.0"
	MinimumMajorStemcellDirectorVersionForODB                   = 3262
	MinimumMajorSemverDirectorVersionForLifecycleErrands        = 261
)

func (b *Broker) startupChecks() error {
	logger := b.loggerFactory.New()

	startupErrors := []string{}

	cfAPIVersionChecker := startupchecker.NewCFAPIVersionChecker(b.cfClient, MinimumCFVersion, logger)
	cfPlanConsistencyChecker := startupchecker.NewCFPlanConsistencyChecker(b.cfClient, b.serviceOffering, logger)
	boshChecker := startupchecker.NewBOSHDirectorVersionChecker(
		MinimumMajorStemcellDirectorVersionForODB,
		MinimumMajorSemverDirectorVersionForLifecycleErrands,
		b.boshInfo,
		b.serviceOffering,
	)
	boshAuthChecker := startupchecker.NewBOSHAuthChecker(b.boshClient, logger)

	err := cfAPIVersionChecker.Check()
	if err != nil {
		startupErrors = append(startupErrors, err.Error())
	}

	if len(startupErrors) == 0 {
		err = cfPlanConsistencyChecker.Check()
		if err != nil {
			startupErrors = append(startupErrors, err.Error())
		}
	}

	err = boshChecker.Check()
	if err != nil {
		startupErrors = append(startupErrors, err.Error())
	}

	if len(startupErrors) > 0 {
		return errors.New(strings.Join(startupErrors, " "))
	}

	err = boshAuthChecker.Check()
	if err != nil {
		return err
	}

	return nil
}
