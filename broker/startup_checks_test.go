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
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("Initializing the broker", func() {
	Describe("check CF API version", func() {
		Context("when the CF API version is sufficient", func() {
			It("calls the CF API", func() {
				Expect(cfClient.GetAPIVersionCallCount()).To(Equal(1))
			})

			It("returns no error", func() {
				Expect(brokerCreationErr).NotTo(HaveOccurred())
			})
		})

		Context("when the CF API version is more than sufficient", func() {
			BeforeEach(func() {
				cfClient.GetAPIVersionReturns("3.0.0", nil)
			})

			It("calls the CF API", func() {
				Expect(cfClient.GetAPIVersionCallCount()).To(Equal(1))
			})

			It("returns no error", func() {
				Expect(brokerCreationErr).NotTo(HaveOccurred())
			})
		})

		Context("when the CF API version is insufficient", func() {
			BeforeEach(func() {
				cfClient.GetAPIVersionReturns("2.56.0", nil)
			})

			It("calls the CF API", func() {
				Expect(cfClient.GetAPIVersionCallCount()).To(Equal(1))
			})

			It("returns no error", func() {
				Expect(brokerCreationErr).To(HaveOccurred())
				Expect(brokerCreationErr.Error()).To(ContainSubstring("Cloud Foundry API version is insufficient, ODB requires CF v238+"))
			})
		})

		Context("when getting the CF API version fails", func() {
			BeforeEach(func() {
				cfClient.GetAPIVersionReturns("", fmt.Errorf("get api error"))
			})

			It("calls the CF API", func() {
				Expect(cfClient.GetAPIVersionCallCount()).To(Equal(1))
			})

			It("returns an error", func() {
				Expect(brokerCreationErr).To(HaveOccurred())
				Expect(brokerCreationErr).To(MatchError("CF API error: get api error. ODB requires CF v238+."))
			})
		})

		Context("when the CF API version cannot be parsed", func() {
			BeforeEach(func() {
				cfClient.GetAPIVersionReturns("1.invalid.0", nil)
			})

			It("calls the CF API", func() {
				Expect(cfClient.GetAPIVersionCallCount()).To(Equal(1))
			})

			It("returns an error", func() {
				Expect(brokerCreationErr).To(HaveOccurred())
				Expect(brokerCreationErr.Error()).To(ContainSubstring("Cloud Foundry API version couldn't be parsed. Expected a semver, got: 1.invalid.0"))
			})
		})
	})

	Describe("check BOSH director version", func() {
		Context("when the BOSH director version supports ODB and lifecycle errands are not configured", func() {
			BeforeEach(func() {
				boshDirectorVersion = boshdirector.NewVersion(boshdirector.MinimumMajorStemcellDirectorVersionForODB, boshdirector.StemcellDirectorVersionType)
				boshClient.GetDirectorVersionReturns(boshDirectorVersion, nil)
				serviceCatalog.Plans = config.Plans{
					existingPlan,
					secondPlan,
				}
			})

			It("calls the BOSH director", func() {
				Expect(boshClient.GetDirectorVersionCallCount()).To(Equal(1))
			})

			It("does not return an error", func() {
				Expect(brokerCreationErr).NotTo(HaveOccurred())
			})
		})

		Context("when the BOSH director version does not support ODB", func() {
			BeforeEach(func() {
				boshDirectorVersion = boshdirector.NewVersion(boshdirector.MinimumMajorStemcellDirectorVersionForODB-1, boshdirector.StemcellDirectorVersionType)
				boshClient.GetDirectorVersionReturns(boshDirectorVersion, nil)
			})

			It("calls the BOSH director", func() {
				Expect(boshClient.GetDirectorVersionCallCount()).To(Equal(1))
			})

			It("returns an error", func() {
				Expect(brokerCreationErr).To(HaveOccurred())
				Expect(brokerCreationErr).To(MatchError("BOSH Director error: API version is insufficient, ODB requires BOSH v257+."))
			})
		})

		Context("when the BOSH director version supports lifecycle errands", func() {
			BeforeEach(func() {
				boshDirectorVersion = boshdirector.NewVersion(boshdirector.MinimumMajorSemverDirectorVersionForLifecycleErrands, boshdirector.SemverDirectorVersionType)
				boshClient.GetDirectorVersionReturns(boshDirectorVersion, nil)
			})

			It("calls the BOSH director", func() {
				Expect(boshClient.GetDirectorVersionCallCount()).To(Equal(1))
			})

			It("does not return an error", func() {
				Expect(brokerCreationErr).NotTo(HaveOccurred())
			})
		})

		Context("when the BOSH director version does not support lifecycle errands and lifecycle errands are configured", func() {
			BeforeEach(func() {
				boshDirectorVersion = boshdirector.NewVersion(boshdirector.MinimumMajorStemcellDirectorVersionForODB, boshdirector.StemcellDirectorVersionType)
				boshClient.GetDirectorVersionReturns(boshDirectorVersion, nil)
			})

			It("calls the BOSH director", func() {
				Expect(boshClient.GetDirectorVersionCallCount()).To(Equal(1))
			})

			It("return an error", func() {
				Expect(brokerCreationErr).To(HaveOccurred())
				Expect(brokerCreationErr).To(MatchError("BOSH Director error: API version is insufficient, one or more plans are configured with lifecycle_errands which require BOSH v261+."))
			})
		})

		Context("when the BOSH director version does not support lifecycle errands and lifecycle errands are not configured", func() {
			BeforeEach(func() {
				boshDirectorVersion = boshdirector.NewVersion(boshdirector.MinimumMajorStemcellDirectorVersionForODB, boshdirector.StemcellDirectorVersionType)
				boshClient.GetDirectorVersionReturns(boshDirectorVersion, nil)

				emptyLifecycleErrandsPlan := config.Plan{
					ID: "empty-lifecycle-errands-plan-id",
					LifecycleErrands: &config.LifecycleErrands{
						PostDeploy: "",
						PreDelete:  "",
					},
					InstanceGroups: []serviceadapter.InstanceGroup{},
				}

				serviceCatalog.Plans = config.Plans{
					existingPlan,
					emptyLifecycleErrandsPlan,
				}
			})

			It("calls the BOSH director", func() {
				Expect(boshClient.GetDirectorVersionCallCount()).To(Equal(1))
			})

			It("should not return an error", func() {
				Expect(brokerCreationErr).NotTo(HaveOccurred())
			})
		})

		Context("when getting the BOSH director version fails", func() {
			BeforeEach(func() {
				boshClient.GetDirectorVersionReturns(boshdirector.Version{}, fmt.Errorf("bosh request error"))
			})

			It("calls the BOSH director", func() {
				Expect(boshClient.GetDirectorVersionCallCount()).To(Equal(1))
			})

			It("returns an error", func() {
				Expect(brokerCreationErr).To(HaveOccurred())
				Expect(brokerCreationErr).To(MatchError("BOSH Director error: bosh request error. ODB requires BOSH v257+."))
			})
		})

		Context("when the BOSH director version is unrecognised (e.g. bosh director 260.3)", func() {
			BeforeEach(func() {
				boshClient.GetDirectorVersionReturns(boshdirector.Version{}, fmt.Errorf(`unrecognised BOSH Director version: "0000 (00000000)"`))
			})

			It("calls the BOSH director", func() {
				Expect(boshClient.GetDirectorVersionCallCount()).To(Equal(1))
			})

			It("returns an error", func() {
				Expect(brokerCreationErr).To(HaveOccurred())
				Expect(brokerCreationErr).To(MatchError(`BOSH Director error: unrecognised BOSH Director version: "0000 (00000000)". ODB requires BOSH v257+.`))
			})
		})
	})

	Describe("check CF service instances", func() {
		Context("when there are no pre-existing instances", func() {
			BeforeEach(func() {
				cfClient.GetAPIVersionReturns("2.57.0", nil)
				cfClient.CountInstancesOfServiceOfferingReturns(map[string]int{}, nil)
			})

			It("returns no error", func() {
				Expect(brokerCreationErr).NotTo(HaveOccurred())
			})
		})

		Context("when there are no pre-existing instances of configured plans", func() {
			Context("service catalog contains the existing instance's plan", func() {
				BeforeEach(func() {
					cfClient.CountInstancesOfServiceOfferingReturns(map[string]int{existingPlanID: 1}, nil)
				})

				It("returns no error", func() {
					Expect(brokerCreationErr).NotTo(HaveOccurred())
				})
			})

			Context("service catalog does not contain a plan with zero instances", func() {
				BeforeEach(func() {
					cfClient.CountInstancesOfServiceOfferingReturns(map[string]int{existingPlanID: 1, "non-existing-plan": 0}, nil)
				})

				It("returns no error", func() {
					Expect(brokerCreationErr).NotTo(HaveOccurred())
				})
			})

			Context("service catalog does not contain the existing instance's plan", func() {
				BeforeEach(func() {
					cfClient.CountInstancesOfServiceOfferingReturns(map[string]int{"non-existent-plan": 1}, nil)
				})

				It("returns an error", func() {
					Expect(brokerCreationErr).To(MatchError(ContainSubstring("You cannot change the plan_id of a plan that has existing service instances")))
				})
			})

			Context("when instances cannot be retrieved", func() {
				BeforeEach(func() {
					cfClient.CountInstancesOfServiceOfferingReturns(nil, errors.New("error counting instances"))
				})

				It("returns an error", func() {
					Expect(brokerCreationErr).To(HaveOccurred())
				})
			})
		})
	})
})
