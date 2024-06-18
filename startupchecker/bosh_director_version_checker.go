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

package startupchecker

import (
	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

type BOSHDirectorVersionChecker struct {
	minimumMajorStemcellDirectorVersionForODB            int
	minimumMajorSemverDirectorVersionForLifecycleErrands int
	boshInfo                                             boshdirector.Info
	brokerConfig                                         config.Config
}

func NewBOSHDirectorVersionChecker(
	minimumMajorStemcellDirectorVersionForODB int,
	minimumMajorSemverDirectorVersionForLifecycleErrands int,
	boshInfo boshdirector.Info,
	config config.Config,
) *BOSHDirectorVersionChecker {
	return &BOSHDirectorVersionChecker{
		minimumMajorStemcellDirectorVersionForODB:            minimumMajorStemcellDirectorVersionForODB,
		minimumMajorSemverDirectorVersionForLifecycleErrands: minimumMajorSemverDirectorVersionForLifecycleErrands,
		boshInfo:     boshInfo,
		brokerConfig: config,
	}
}

func (c *BOSHDirectorVersionChecker) Check() error {
	errPrefix := "BOSH Director error: "
	directorVersion, err := c.boshInfo.GetDirectorVersion()
	if err != nil {
		return fmt.Errorf("%s%s. ODB requires BOSH v257+.", errPrefix, err)
	}
	if !c.directorVersionSufficientForODB(directorVersion) {
		return fmt.Errorf("%sAPI version is insufficient, ODB requires BOSH v257+.", errPrefix)
	}
	if c.brokerConfig.HasBindingWithDNSConfigured() && !c.directorVersionSufficientForBindingWithDNS(directorVersion) {
		return fmt.Errorf("%sAPI version for 'binding_with_dns' feature is insufficient. This feature requires BOSH v266.12+ / v267.6+ (got v%s)", errPrefix, directorVersion.Version)
	}
	if c.brokerConfig.ServiceCatalog.HasLifecycleErrands() && !c.directorVersionSufficientForLifecycleErrands(directorVersion) {
		return fmt.Errorf(
			"%sAPI version is insufficient, one or more plans are configured with lifecycle_errands which require BOSH v%d+.",
			errPrefix,
			c.minimumMajorSemverDirectorVersionForLifecycleErrands,
		)
	}

	return nil
}

func (c *BOSHDirectorVersionChecker) directorVersionSufficientForODB(directorVersion boshdirector.Version) bool {
	return directorVersion.Type == boshdirector.SemverDirectorVersionType ||
		int(directorVersion.Version.Major) >= c.minimumMajorStemcellDirectorVersionForODB
}

func (c *BOSHDirectorVersionChecker) directorVersionSufficientForBindingWithDNS(directorVersion boshdirector.Version) bool {
	if directorVersion.Type == boshdirector.SemverDirectorVersionType {
		if directorVersion.Version.Major < 266 {
			return false
		}

		if directorVersion.Version.Major == 266 && directorVersion.Version.Minor < 12 {
			return false
		}

		if directorVersion.Version.Major == 267 && directorVersion.Version.Minor < 6 {
			return false
		}

		return true
	}
	return false
}

func (c *BOSHDirectorVersionChecker) directorVersionSufficientForLifecycleErrands(directorVersion boshdirector.Version) bool {
	return directorVersion.Type == boshdirector.SemverDirectorVersionType &&
		int(directorVersion.Version.Major) >= c.minimumMajorSemverDirectorVersionForLifecycleErrands
}
