// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_registration_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

var _ = Describe("broker registration errands", func() {
	Describe("Register-broker errand", func() {
		When("user is logged in as admin", func() {
			It("can see the service and all plans in the marketplace regardless of cf_service_access", func() {
				cfLogInAsAdmin()

				marketplaceSession := cf.Cf("marketplace", "-e", "--show-unavailable", brokerInfo.ServiceName)

				By("confirming the registered offerings in the marketplace")
				Expect(marketplaceSession).To(gbytes.Say(brokerInfo.ServiceName))
				Expect(marketplaceSession).To(gbytes.Say("default-plan"))
				Expect(marketplaceSession).To(gbytes.Say("enabled-plan"))
				Expect(marketplaceSession).To(gbytes.Say("disabled-plan"))
				Expect(marketplaceSession).To(gbytes.Say("org-restricted-plan"))
				Expect(marketplaceSession).To(gbytes.Say("manual-plan"))

				Expect(marketplaceSession).To(gexec.Exit(0))
			})
		})

		When("user is logged in as space dev", func() {
			BeforeEach(func() {
				cfLogInAsSpaceDev()
			})

			When("cf_service_access is not set for one of the plans", func() {
				It("should be visible in the marketplace, as it's enabled by default", func() {
					marketplaceSession := cf.Cf("marketplace", "-e", brokerInfo.ServiceName)
					Expect(marketplaceSession).To(gbytes.Say(brokerInfo.ServiceName))
					Expect(marketplaceSession).To(gbytes.Say("default-plan"))
				})
			})

			When("cf_service_access is set to enable for one of the plans", func() {
				It("should be visible in the marketplace", func() {
					marketplaceSession := cf.Cf("marketplace", "-e", brokerInfo.ServiceName)
					Expect(marketplaceSession).To(gbytes.Say(brokerInfo.ServiceName))
					Expect(marketplaceSession).To(gbytes.Say("enabled-plan"))
				})
			})

			When("cf_service_access is set to disable for one of the plans", func() {
				It("will disable access when the broker is re-registered", func() {
					By("should not be visible in the marketplace")
					marketplaceSession := cf.Cf("marketplace", "-e", brokerInfo.ServiceName)
					Expect(marketplaceSession).To(gbytes.Say(brokerInfo.ServiceName))
					Expect(marketplaceSession).ToNot(gbytes.Say("disabled-plan"))

					By("manually enabling service access to the service")
					cfLogInAsAdmin()
					session := cf.Cf("enable-service-access", brokerInfo.ServiceName, "-p", "disabled-plan")
					Expect(session).
						To(gexec.Exit(0))

					By("confirming the plan is now visible to space devs in the marketplace")
					cfLogInAsSpaceDev()
					allEnabledMarketplaceSession := cf.Cf("marketplace", "-e", brokerInfo.ServiceName)
					Expect(allEnabledMarketplaceSession).To(gbytes.Say("disabled-plan"))

					By("re-registering the broker")
					bosh_helpers.RunErrand(brokerInfo.DeploymentName, "register-broker")

					By("confirming the disabled-plan is now disabled again")
					allButInactiveMarketplaceSession := cf.Cf("marketplace", "-e", brokerInfo.ServiceName)
					Expect(allButInactiveMarketplaceSession).ToNot(gbytes.Say("disabled-plan"))
				})
			})

			When("cf_service_access is set to manual for one of the plans", func() {
				AfterEach(func() {
					cfLogInAsAdmin()
					session := cf.Cf("disable-service-access", brokerInfo.ServiceName, "-p", "manual-plan")
					Expect(session).To(gexec.Exit(0))
				})

				It("has to be enabled manually", func() {
					By("should not be visible in the marketplace")
					marketplaceSession := cf.Cf("marketplace", "-e", brokerInfo.ServiceName)
					Expect(marketplaceSession).To(gbytes.Say(brokerInfo.ServiceName))
					Expect(marketplaceSession).ToNot(gbytes.Say("manual-plan"))

					By("manually enabling service access to the service")
					cfLogInAsAdmin()
					session := cf.Cf("enable-service-access", brokerInfo.ServiceName, "-p", "manual-plan")
					Expect(session).To(gexec.Exit(0))

					By("confirming the manual plan is now visible to space devs in the marketplace")
					cfLogInAsSpaceDev()
					allEnabledMarketplaceSession := cf.Cf("marketplace", "-e", brokerInfo.ServiceName)
					Expect(allEnabledMarketplaceSession).To(gbytes.Say("manual-plan"))

					By("re-registering the broker")
					bosh_helpers.RunErrand(brokerInfo.DeploymentName, "register-broker")

					By("confirming the manual plan is still visible to space devs in the marketplace")
					allButInactiveMarketplaceSession := cf.Cf("marketplace", "-e", brokerInfo.ServiceName)
					Expect(allButInactiveMarketplaceSession).To(gbytes.Say("manual-plan"))
				})
			})

			When("cf_service_access is set to org-restricted for one of the plans", func() {
				It("should be visible when logged in user belongs to the correct org", func() {
					cfLogInAsDefaultSpaceDev()

					marketplaceSession := cf.Cf("marketplace", "-e", brokerInfo.ServiceName)
					Expect(marketplaceSession).To(gexec.Exit(0))
					Expect(string(marketplaceSession.Buffer().Contents())).To(SatisfyAll(
						ContainSubstring(brokerInfo.ServiceName),
						ContainSubstring("org-restricted-plan-2 "),
						ContainSubstring("org-restricted-plan "),
					))
				})

				It("should not be visible when logged in user belongs to different org", func() {
					cfLogInAsSpaceDev()

					marketplaceSession := cf.Cf("marketplace", "-e", brokerInfo.ServiceName)
					Expect(marketplaceSession).To(gbytes.Say(brokerInfo.ServiceName))
					Expect(marketplaceSession).ToNot(gbytes.Say("org-restricted-plan"))
				})

				It("should be restricted after previously being enabled", func() {
					By("manually enabling service access to the plan to all", func() {
						cfLogInAsAdmin()
						session := cf.Cf("enable-service-access", brokerInfo.ServiceName, "-p", "org-restricted-plan-2")
						Expect(session).To(gexec.Exit(0))
					})

					By("making sure that the space dev can see the about-to-be-limited plan", func() {
						cfLogInAsSpaceDev()
						marketplaceSession := cf.Cf("marketplace", "-e", brokerInfo.ServiceName)
						Expect(marketplaceSession).To(gexec.Exit(0))
						Expect(marketplaceSession).To(gbytes.Say("org-restricted-plan-2"))
					})

					By("re-registering the broker with limited plan-2", func() {
						bosh_helpers.RunErrand(brokerInfo.DeploymentName, "register-broker")
					})

					By("making sure that the space dev now cannot see that", func() {
						cfLogInAsSpaceDev()
						marketplaceSession := cf.Cf("marketplace", "-e", brokerInfo.ServiceName)
						Expect(marketplaceSession).To(gexec.Exit(0))
						Expect(marketplaceSession).ToNot(gbytes.Say("org-restricted-plan-2"))
					})

					By("making sure that the authorised space dev now can see that", func() {
						cfLogInAsDefaultSpaceDev()
						marketplaceSession := cf.Cf("marketplace", "-e", brokerInfo.ServiceName)
						Expect(marketplaceSession).To(gexec.Exit(0))
						Expect(marketplaceSession).To(gbytes.Say("org-restricted-plan-2"))
					})
				})

				It("should be re-enabled after disabled", func() {
					By("manually disabling service access to the plan")
					cfLogInAsAdmin()
					session := cf.Cf("disable-service-access", brokerInfo.ServiceName, "-p", "org-restricted-plan")
					Expect(session).To(gexec.Exit(0))

					By("re-registering the broker")
					bosh_helpers.RunErrand(brokerInfo.DeploymentName, "register-broker")

					cfLogInAsDefaultSpaceDev()
					marketplaceSession := cf.Cf("marketplace", "-e", brokerInfo.ServiceName)
					Expect(marketplaceSession).To(gexec.Exit(0))

					Expect(string(marketplaceSession.Buffer().Contents())).To(SatisfyAll(
						ContainSubstring(brokerInfo.ServiceName),
						ContainSubstring("org-restricted-plan-2 "),
						ContainSubstring("org-restricted-plan "),
					))
				})
			})
		})
	})

	Describe("deregister-broker", func() {
		BeforeEach(func() {
			cfLogInAsAdmin()
			bosh_helpers.RunErrand(brokerInfo.DeploymentName, "register-broker")
			serviceBrokersSession := cf.Cf("service-brokers")
			Expect(serviceBrokersSession).To(gbytes.Say(brokerInfo.BrokerName))
		})

		AfterEach(func() {
			bosh_helpers.RunErrand(brokerInfo.DeploymentName, "register-broker")
		})

		It("removes the service from the CF", func() {
			bosh_helpers.RunErrand(brokerInfo.DeploymentName, "deregister-broker")
			serviceBrokersSession := cf.Cf("service-brokers")
			Expect(serviceBrokersSession).ToNot(gbytes.Say(brokerInfo.BrokerName))
		})
	})
})
