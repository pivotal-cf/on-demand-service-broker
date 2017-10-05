// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

var _ = Describe("Initializing the broker", func() {
	Describe("check CF API version", func() {
		var boshInfo *boshdirector.Info

		BeforeEach(func() {
			boshInfo = createBOSHInfoWithMajorVersion(
				boshdirector.MinimumMajorSemverDirectorVersionForLifecycleErrands,
				boshdirector.VersionType("semver"),
			)
		})

		It("returns with no error if CF API version is sufficient", func() {
			_, brokerCreationErr := createBroker(boshInfo)
			Expect(brokerCreationErr).NotTo(HaveOccurred())
			Expect(cfClient.GetAPIVersionCallCount()).To(Equal(1))
		})

		It("returns with no error if the CF API version is more than sufficient", func() {
			cfClient.GetAPIVersionReturns("3.0.0", nil)
			_, brokerCreationErr = createBroker(boshInfo)
			Expect(cfClient.GetAPIVersionCallCount()).To(Equal(1))
			Expect(brokerCreationErr).NotTo(HaveOccurred())
		})

		It("returns with error if the CF API version is insufficient", func() {
			cfClient.GetAPIVersionReturns("2.56.0", nil)
			_, brokerCreationErr = createBroker(boshInfo)
			Expect(cfClient.GetAPIVersionCallCount()).To(Equal(1))
			Expect(brokerCreationErr).To(HaveOccurred())
			Expect(brokerCreationErr.Error()).To(ContainSubstring("Cloud Foundry API version is insufficient, ODB requires CF v238+"))
		})

		It("returns with an error if the CF API request failed", func() {
			cfClient.GetAPIVersionReturns("", fmt.Errorf("get api error"))
			_, brokerCreationErr = createBroker(boshInfo)
			Expect(cfClient.GetAPIVersionCallCount()).To(Equal(1))
			Expect(brokerCreationErr).To(HaveOccurred())
			Expect(brokerCreationErr).To(MatchError("CF API error: get api error. ODB requires CF v238+."))
		})

		It("returns with an error if the CF API version cannot be parsed", func() {
			cfClient.GetAPIVersionReturns("1.invalid.0", nil)
			_, brokerCreationErr = createBroker(boshInfo)
			Expect(cfClient.GetAPIVersionCallCount()).To(Equal(1))
			Expect(brokerCreationErr).To(HaveOccurred())
			Expect(brokerCreationErr.Error()).To(ContainSubstring("Cloud Foundry API version couldn't be parsed. Expected a semver, got: 1.invalid.0"))
		})

		It("does not verify CF API version when CF startup checks are disabled", func() {
			_, brokerCreationErr := broker.New(
				boshInfo,
				boshClient,
				cfClient,
				serviceAdapter,
				fakeDeployer,
				serviceCatalog,
				true,
				loggerFactory,
			)
			Expect(brokerCreationErr).NotTo(HaveOccurred())
			Expect(cfClient.GetAPIVersionCallCount()).To(Equal(0))
		})
	})

	Describe("check BOSH director version", func() {
		It("returns no error when the BOSH director version supports ODB and lifecycle errands are not configured", func() {
			serviceCatalog.Plans = config.Plans{
				existingPlan,
				secondPlan,
			}
			boshInfo := createBOSHInfoWithMajorVersion(
				boshdirector.MinimumMajorStemcellDirectorVersionForODB,
				boshdirector.VersionType("stemcell"),
			)
			_, brokerCreationErr = createBroker(boshInfo)
			Expect(brokerCreationErr).NotTo(HaveOccurred())
		})

		It("returns an error when the BOSH director version does not support ODB", func() {
			boshInfo := createBOSHInfoWithMajorVersion(
				boshdirector.MinimumMajorStemcellDirectorVersionForODB-1,
				boshdirector.VersionType("stemcell"),
			)
			_, brokerCreationErr = createBroker(boshInfo)
			Expect(brokerCreationErr).To(HaveOccurred())
			Expect(brokerCreationErr).To(MatchError("BOSH Director error: API version is insufficient, ODB requires BOSH v257+."))
		})

		It("returns no error when BOSH director supports lifecycle errands", func() {
			boshInfo := createBOSHInfoWithMajorVersion(
				boshdirector.MinimumMajorSemverDirectorVersionForLifecycleErrands,
				boshdirector.VersionType("semver"),
			)
			_, brokerCreationErr = createBroker(boshInfo)
			Expect(brokerCreationErr).NotTo(HaveOccurred())
		})

		It("returns an error when the BOSH director version does not support lifecycle errands and lifecycle errands are configured", func() {
			boshInfo := createBOSHInfoWithMajorVersion(
				boshdirector.MinimumMajorStemcellDirectorVersionForODB,
				boshdirector.VersionType("stemcell"),
			)
			_, brokerCreationErr = createBroker(boshInfo)
			Expect(brokerCreationErr).To(HaveOccurred())
			Expect(brokerCreationErr).To(MatchError("BOSH Director error: API version is insufficient, one or more plans are configured with lifecycle_errands which require BOSH v261+."))
		})

		It("returns no error when the BOSH director version does not support lifecycle errands and lifecycle errands are not configured", func() {
			emptyLifecycleErrandsPlan := config.Plan{
				ID: "empty-lifecycle-errands-plan-id",
				LifecycleErrands: &config.LifecycleErrands{
					PostDeploy: "",
					PreDelete:  "",
				},
			}

			serviceCatalog.Plans = config.Plans{
				existingPlan,
				emptyLifecycleErrandsPlan,
			}
			boshInfo := createBOSHInfoWithMajorVersion(
				boshdirector.MinimumMajorStemcellDirectorVersionForODB,
				boshdirector.VersionType("stemcell"),
			)
			_, brokerCreationErr = createBroker(boshInfo)
			Expect(brokerCreationErr).NotTo(HaveOccurred())
		})

		It("returns an error when the BOSH director version in unrecognised", func() {
			boshInfo := &boshdirector.Info{Version: "0000 (00000000)"}
			_, brokerCreationErr = createBroker(boshInfo)
			Expect(brokerCreationErr).To(HaveOccurred())
			Expect(brokerCreationErr).To(MatchError(`BOSH Director error: unrecognised BOSH Director version: "0000 (00000000)". ODB requires BOSH v257+.`))
		})

		It("still verifies BOSH version when CF startup checks are disabled", func() {
			boshInfo := &boshdirector.Info{Version: "0000 (00000000)"}

			_, brokerCreationErr := broker.New(
				boshInfo,
				boshClient,
				cfClient,
				serviceAdapter,
				fakeDeployer,
				serviceCatalog,
				true,
				loggerFactory,
			)
			Expect(brokerCreationErr).To(HaveOccurred())
		})
	})

	Describe("check CF service instances", func() {
		var boshInfo *boshdirector.Info

		BeforeEach(func() {
			boshInfo = createBOSHInfoWithMajorVersion(
				boshdirector.MinimumMajorSemverDirectorVersionForLifecycleErrands,
				boshdirector.VersionType("semver"),
			)
		})

		It("returns no error when there are no pre-existing instances", func() {
			cfClient.GetAPIVersionReturns("2.57.0", nil)
			cfClient.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{}, nil)

			_, brokerCreationErr = createBroker(boshInfo)
			Expect(brokerCreationErr).NotTo(HaveOccurred())
		})

		It("returns no error when there are no pre-existing instances of configured plans and service catalog contains the existing instance's plan", func() {
			cfClient.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{
				cfServicePlan("1234", existingPlanID, "url", existingPlanName): 1,
			}, nil)
			_, brokerCreationErr = createBroker(boshInfo)
			Expect(brokerCreationErr).NotTo(HaveOccurred())
		})

		It("returns no error when the service catalog does not contain a plan with zero instances", func() {
			cfClient.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{
				cfServicePlan("1234", existingPlanID, "url", existingPlanName):            1,
				cfServicePlan("1234", "non-existent-plan-id", "url", "non-existent-plan"): 0,
			}, nil)
			_, brokerCreationErr = createBroker(boshInfo)
			Expect(brokerCreationErr).NotTo(HaveOccurred())
		})

		It("returns an error if service catalog does not contain the existing instance's plan", func() {
			cfClient.CountInstancesOfServiceOfferingReturns(map[cf.ServicePlan]int{
				cfServicePlan("1234", "non-existent-plan-id", "url", "non-existent-plan"): 1,
			}, nil)
			_, brokerCreationErr = createBroker(boshInfo)
			Expect(brokerCreationErr).To(HaveOccurred())
			Expect(brokerCreationErr).To(MatchError(ContainSubstring(
				fmt.Sprintf("plan non-existent-plan (non-existent-plan-id) was expected but is now missing. You cannot remove or change the plan_id of a plan which has existing service instances"),
			)))
		})

		It("returns an error when instances cannot be retrieved", func() {
			cfClient.CountInstancesOfServiceOfferingReturns(nil, errors.New("error counting instances"))
			_, brokerCreationErr = createBroker(boshInfo)
			Expect(brokerCreationErr).To(HaveOccurred())
		})

		It("does not check preexisting versions when CF startup checks are disabled", func() {
			_, brokerCreationErr := broker.New(
				boshInfo,
				boshClient,
				cfClient,
				serviceAdapter,
				fakeDeployer,
				serviceCatalog,
				true,
				loggerFactory,
			)
			Expect(brokerCreationErr).NotTo(HaveOccurred())
			Expect(cfClient.CountInstancesOfServiceOfferingCallCount()).To(Equal(0))
		})

	})
})
