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
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

var _ = Describe("broker registration errands", func() {
	BeforeEach(func() {
		cfLogInAsAdmin()
		Eventually(cf.Cf("delete-service-broker", brokerName, "-f")).Should(gexec.Exit(0))
	})

	Describe("register-broker", func() {
		BeforeEach(func() {
			boshClient.RunErrand(brokerBoshDeploymentName, "register-broker", []string{}, "")
		})

		AfterEach(func() {
			cfLogInAsAdmin()
			Eventually(cf.Cf("disable-service-access", serviceOffering)).Should(gexec.Exit(0))
			Eventually(cf.Cf("purge-service-offering", serviceOffering, "-f")).Should(gexec.Exit(0))
		})

		Context("when the broker is not registered", func() {
			Context("and the user is admin", func() {
				It("registers the broker with CF", func() {
					marketplaceSession := cf.Cf("marketplace", "-s", serviceOffering)

					By("confirming the registered offerings in the marketplace")
					Eventually(marketplaceSession).Should(gbytes.Say(serviceOffering))
					Eventually(marketplaceSession).Should(gbytes.Say("dedicated-vm"))
					Eventually(marketplaceSession).Should(gbytes.Say("dedicated-high-memory-vm"))

					Eventually(marketplaceSession).Should(gbytes.Say("inactive-plan"))
					Eventually(marketplaceSession).Should(gbytes.Say("manual-plan"))

					Eventually(marketplaceSession).Should(gexec.Exit(0))
				})
			})

			Context("and the user is a space developer", func() {
				It("registers the broker with CF", func() {
					cfLogInAsSpaceDev()
					marketplaceSession := cf.Cf("marketplace", "-s", serviceOffering)

					By("confirming the registered offerings in the marketplace")
					Eventually(marketplaceSession).Should(gbytes.Say(serviceOffering))
					Eventually(marketplaceSession).Should(gbytes.Say("dedicated-vm"))
					Eventually(marketplaceSession).Should(gbytes.Say("dedicated-high-memory-vm"))

					By("confirming disabled and manual plans are not visible in the marketplace")
					Eventually(marketplaceSession).ShouldNot(gbytes.Say("inactive-plan"))
					Eventually(marketplaceSession).ShouldNot(gbytes.Say("manual-plan"))

					Eventually(marketplaceSession).Should(gexec.Exit(0))
				})
			})
		})

		Context("when enabling cf access for a plan that is set to manual", func() {
			It("has to be enabled manually", func() {
				By("confirming manual plan is not visible to space devs in the marketplace")
				cfLogInAsSpaceDev()
				marketplaceSession := cf.Cf("marketplace", "-s", serviceOffering)
				Eventually(marketplaceSession).ShouldNot(gbytes.Say("manual-plan"))

				By("manually enabling service access to the service")
				cfLogInAsAdmin()
				Eventually(cf.Cf("enable-service-access", serviceOffering)).Should(gexec.Exit(0))

				By("confirming the manual plan is now visible to space devs in the marketplace")
				cfLogInAsSpaceDev()
				allEnabledMarketplaceSession := cf.Cf("marketplace", "-s", serviceOffering)
				Eventually(allEnabledMarketplaceSession).Should(gbytes.Say("manual-plan"))

				By("re-registering the broker")
				boshClient.RunErrand(brokerBoshDeploymentName, "register-broker", []string{}, "")

				By("confirming the manual plan is still visible to space devs in the marketplace")
				allButInactiveMarketplaceSession := cf.Cf("marketplace", "-s", serviceOffering)
				Eventually(allButInactiveMarketplaceSession).Should(gbytes.Say("manual-plan"))
			})
		})

		Context("when enabling cf access for a plan that is set to disable", func() {
			It("will revert to disabled when the broker is re-registered", func() {
				By("confirming inactive plan is not visible to space devs in the marketplace")
				cfLogInAsSpaceDev()
				marketplaceSession := cf.Cf("marketplace", "-s", serviceOffering)
				Eventually(marketplaceSession).ShouldNot(gbytes.Say("inactive-plan"))

				By("manually enabling service access to the service")
				cfLogInAsAdmin()
				Eventually(cf.Cf("enable-service-access", serviceOffering)).Should(gexec.Exit(0))

				By("confirming the inactive plan is now visible to space devs in the marketplace")
				cfLogInAsSpaceDev()
				allEnabledMarketplaceSession := cf.Cf("marketplace", "-s", serviceOffering)
				Eventually(allEnabledMarketplaceSession).Should(gbytes.Say("inactive-plan"))

				By("re-registering the broker")
				boshClient.RunErrand(brokerBoshDeploymentName, "register-broker", []string{}, "")

				By("confirming the inactive plan is no longer visible to space devs in the marketplace, while an active plan is")
				allButInactiveMarketplaceSession := cf.Cf("marketplace", "-s", serviceOffering)
				Eventually(allButInactiveMarketplaceSession).ShouldNot(gbytes.Say("inactive-plan"))
				Eventually(marketplaceSession).Should(gbytes.Say("dedicated-vm"))
			})
		})
	})

	Describe("deregister-broker", func() {
		It("removes the service from the CF", func() {
			boshClient.RunErrand(brokerBoshDeploymentName, "register-broker", []string{}, "")
			serviceBrokersSession := cf.Cf("service-brokers")
			Eventually(serviceBrokersSession).Should(gbytes.Say(brokerName))

			boshClient.RunErrand(brokerBoshDeploymentName, "deregister-broker", []string{}, "")
			serviceBrokersSession = cf.Cf("service-brokers")
			Eventually(serviceBrokersSession).ShouldNot(gbytes.Say(brokerName))
		})
	})
})
