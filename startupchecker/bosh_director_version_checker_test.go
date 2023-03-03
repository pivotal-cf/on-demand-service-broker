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

package startupchecker_test

import (
	"fmt"
	"strconv"

	"github.com/pivotal-cf/on-demand-service-broker/config"
	. "github.com/pivotal-cf/on-demand-service-broker/startupchecker"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
)

var _ = Describe("BOSH Director Version Checker", func() {
	var (
		serviceCatalog       config.ServiceOffering
		postDeployErrandPlan config.Plan
	)

	BeforeEach(func() {
		serviceCatalog = config.ServiceOffering{}
		postDeployErrandPlan = config.Plan{
			ID: "post-deploy",
			LifecycleErrands: &serviceadapter.LifecycleErrands{
				PostDeploy: []serviceadapter.Errand{{
					Name: "health-check",
				}},
			},
			InstanceGroups: []serviceadapter.InstanceGroup{},
		}
	})

	It("returns no error when the BOSH director version supports ODB and lifecycle errands are not configured", func() {
		boshInfo := createBOSHInfoWithMajorVersion(
			broker.MinimumMajorStemcellDirectorVersionForODB,
			boshdirector.VersionType("stemcell"),
		)
		c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error when the BOSH director version does not support ODB", func() {
		boshInfo := createBOSHInfoWithMajorVersion(
			broker.MinimumMajorStemcellDirectorVersionForODB-1,
			boshdirector.VersionType("stemcell"),
		)
		c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
		err := c.Check()
		Expect(err).To(MatchError("BOSH Director error: API version is insufficient, ODB requires BOSH v257+."))
	})

	It("returns no error when BOSH director supports lifecycle errands", func() {
		boshInfo := createBOSHInfoWithMajorVersion(
			broker.MinimumMajorSemverDirectorVersionForLifecycleErrands,
			boshdirector.VersionType("semver"),
		)
		c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error when the BOSH director version does not support lifecycle errands and lifecycle errands are configured", func() {
		serviceCatalog.Plans = []config.Plan{postDeployErrandPlan}
		boshInfo := createBOSHInfoWithMajorVersion(
			broker.MinimumMajorStemcellDirectorVersionForODB,
			boshdirector.VersionType("stemcell"),
		)
		c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
		err := c.Check()
		Expect(err).To(MatchError("BOSH Director error: API version is insufficient, one or more plans are configured with lifecycle_errands which require BOSH v261+."))
	})

	It("returns no error when the BOSH director version does not support lifecycle errands and lifecycle errands are not configured", func() {
		emptyLifecycleErrandsPlan := config.Plan{
			ID: "empty-lifecycle-errands-plan-id",
			LifecycleErrands: &serviceadapter.LifecycleErrands{
				PostDeploy: []serviceadapter.Errand{},
				PreDelete:  []serviceadapter.Errand{},
			},
		}

		serviceCatalog.Plans = config.Plans{
			emptyLifecycleErrandsPlan,
		}
		boshInfo := createBOSHInfoWithMajorVersion(
			broker.MinimumMajorStemcellDirectorVersionForODB,
			boshdirector.VersionType("stemcell"),
		)
		c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
		err := c.Check()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("binding_with_dns", func() {
		When("a plan is configured with binding_with_dns", func() {
			BeforeEach(func() {
				serviceCatalog.Plans = []config.Plan{
					{
						Name:           "some-plan",
						ID:             "the-big-plan",
						InstanceGroups: []serviceadapter.InstanceGroup{},
						BindingWithDNS: []config.BindingDNS{
							{Name: "foo", LinkProvider: "bar", InstanceGroup: "some"},
						},
					},
				}
			})

			It("errors when the minimum BOSH version is not satisfied", func() {
				for _, v := range []string{"265.0.0", "266.11.0", "267.5.0"} {
					boshInfo := boshdirector.Info{Version: v}
					c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
					err := c.Check()
					Expect(err).To(MatchError(fmt.Sprintf("BOSH Director error: API version for 'binding_with_dns' feature is insufficient. This feature requires BOSH v266.12+ / v267.6+ (got v%s)", v)), fmt.Sprintf("Expected error for version %s", v))
				}
			})

			It("succeed when the minimum BOSH version is satisfied", func() {
				for _, v := range []string{"266.12.0", "267.6.0", "268.0.0"} {
					boshInfo := boshdirector.Info{Version: v}
					c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
					Expect(c.Check()).To(Succeed(), fmt.Sprintf("Expected version %s to succeed", v))
				}
			})
		})

		When("no plans are configured with binding_with_dns", func() {
			It("should not fail", func() {
				boshInfo := createBOSHInfoWithMajorVersion(
					262,
					boshdirector.VersionType("semver"),
				)
				c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
				Expect(c.Check()).To(Succeed())
			})
		})
	})

	It("returns an error when the BOSH director version in unrecognised", func() {
		boshInfo := boshdirector.Info{Version: "0000 (00000000)"}
		c := createBOSHDirectorVersionChecker(boshInfo, serviceCatalog)
		err := c.Check()
		Expect(err).To(MatchError(`BOSH Director error: unrecognised BOSH Director version: "0000 (00000000)". ODB requires BOSH v257+.`))
	})
})

func createBOSHInfoWithMajorVersion(majorVersion int, versionType boshdirector.VersionType) boshdirector.Info {
	var version string
	if versionType == "semver" {
		version = fmt.Sprintf("%s.1.0", strconv.Itoa(majorVersion))
	} else if versionType == "stemcell" {
		version = fmt.Sprintf("1.%s.0.0", strconv.Itoa(majorVersion))
	}
	return boshdirector.Info{Version: version}
}

func createBOSHDirectorVersionChecker(boshInfo boshdirector.Info, catalog config.ServiceOffering) *BOSHDirectorVersionChecker {
	return NewBOSHDirectorVersionChecker(
		broker.MinimumMajorStemcellDirectorVersionForODB,
		broker.MinimumMajorSemverDirectorVersionForLifecycleErrands,
		boshInfo,
		config.Config{ServiceCatalog: catalog},
	)
}
