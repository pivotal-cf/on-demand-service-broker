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

	"github.com/coreos/go-semver/semver"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

const MinimunCFVersion string = "2.57.0"

func (b *Broker) startupChecks() error {
	logger := b.loggerFactory.New()
	if err := b.checkAPIVersions(logger); err != nil {
		return err
	}

	if err := b.checkAuthentication(logger); err != nil {
		return err
	}

	if err := b.verifyExistingInstancePlanIDsUnchanged(logger); err != nil {
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

func (b *Broker) verifyExistingInstancePlanIDsUnchanged(logger *log.Logger) error {
	instanceCountByPlanID, err := b.cfClient.CountInstancesOfServiceOffering(b.serviceOffering.ID, logger)
	if err != nil {
		return err
	}

	for plan, count := range instanceCountByPlanID {
		_, found := b.serviceOffering.Plans.FindByID(plan.ServicePlanEntity.UniqueID)

		if !found && count > 0 {
			return fmt.Errorf(
				"plan %s (%s) was expected but is now missing. You cannot remove or change the plan_id of a plan which has existing service instances",
				plan.ServicePlanEntity.Name,
				plan.ServicePlanEntity.UniqueID,
			)
		}
	}

	return nil
}

func (b *Broker) checkAPIVersions(logger *log.Logger) error {
	var apiErrorMessages []string

	if err := b.checkCFAPIVersion(logger); err != nil {
		apiErrorMessages = append(apiErrorMessages, "CF API error: "+err.Error())
	}

	if err := b.checkBoshDirectorVersion(logger); err != nil {
		apiErrorMessages = append(apiErrorMessages, "BOSH Director error: "+err.Error())
	}

	if len(apiErrorMessages) > 0 {
		return errors.New(strings.Join(apiErrorMessages, " "))
	}

	return nil
}

func (b *Broker) checkCFAPIVersion(logger *log.Logger) error {
	rawCFAPIVersion, err := b.cfClient.GetAPIVersion(logger)
	if err != nil {
		return fmt.Errorf("%s. ODB requires CF v238+.", err)
	}

	version, err := semver.NewVersion(rawCFAPIVersion)
	if err != nil {
		return fmt.Errorf("Cloud Foundry API version couldn't be parsed. Expected a semver, got: %s.", rawCFAPIVersion)
	}
	if version.LessThan(*semver.New(MinimunCFVersion)) {
		return errors.New("Cloud Foundry API version is insufficient, ODB requires CF v238+.")
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
