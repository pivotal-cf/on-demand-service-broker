// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package quotas_tests

import (
	"fmt"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var _ = Describe("quotas", func() {
	const (
		planA = "dedicated-vm"             //Quota 1
		planB = "dedicated-high-memory-vm" //Quota 5
	)

	var (
		instanceA = fmt.Sprintf("instance-%s", uuid.New()[:7])
		instanceB = fmt.Sprintf("instance-%s", uuid.New()[:7])
	)

	Describe("plan quotas", func() {
		const planQuotaError = "The quota for this service plan has been exceeded. Please contact your Operator for help"

		Context("when the limit has been reached", func() {
			BeforeEach(func() {
				Eventually(cf.Cf("create-service", serviceOffering, planA, instanceA), cf_helpers.CfTimeout).Should(gexec.Exit(0))
				cf_helpers.AwaitServiceCreation(instanceA)
			})

			AfterEach(func() {
				Eventually(cf.Cf("delete-service", instanceA, "-f"), cf_helpers.CfTimeout).Should(gexec.Exit())
				Eventually(cf.Cf("delete-service", instanceB, "-f"), cf_helpers.CfTimeout).Should(gexec.Exit())

				cf_helpers.AwaitServiceDeletion(instanceA)
				cf_helpers.AwaitServiceDeletion(instanceB)
			})

			It("respects plan quotas", func() {
				By("denying a create-service request")
				session := cf.Cf("create-service", serviceOffering, planA, instanceB)
				Eventually(session, cf_helpers.CfTimeout).Should(gexec.Exit())
				Expect(session).To(gbytes.Say(planQuotaError))

				By("deleting an instance")
				Eventually(cf.Cf("delete-service", instanceA, "-f"), cf_helpers.CfTimeout).Should(gexec.Exit(0))
				cf_helpers.AwaitServiceDeletion(instanceA)

				By("successfully provisioning an instance")
				Eventually(cf.Cf("create-service", serviceOffering, planA, instanceB), cf_helpers.CfTimeout).Should(gexec.Exit(0))
				cf_helpers.AwaitServiceCreation(instanceB)
			})

			It("respects quotas when switching between plans", func() {
				By("creating a instance of a different plan")
				Eventually(cf.Cf("create-service", serviceOffering, planB, instanceB), cf_helpers.CfTimeout).Should(gexec.Exit(0))
				cf_helpers.AwaitServiceCreation(instanceB)

				By("updating to a plan with maxed quota")
				session := cf.Cf("update-service", instanceB, "-p", planA)
				Eventually(session, cf_helpers.CfTimeout).Should(gexec.Exit())
				Expect(session).To(gbytes.Say(planQuotaError))

				By("deleting instance to free up quota")
				Eventually(cf.Cf("delete-service", instanceA, "-f"), cf_helpers.CfTimeout).Should(gexec.Exit(0))
				cf_helpers.AwaitServiceDeletion(instanceA)

				By("updating to plan with newly freed quota")
				Eventually(cf.Cf("update-service", instanceB, "-p", planA), cf_helpers.CfTimeout).Should(gexec.Exit(0))
				cf_helpers.AwaitServiceUpdate(instanceB)
			})
		})
	})

	Describe("global quotas", func() {
		const (
			globalQuotaError = "The quota for this service has been exceeded. Please contact your Operator for help"
		)

		var instanceC = fmt.Sprintf("instance-%s", uuid.New()[:7])

		Context("when the global limit is reached", func() {
			BeforeEach(func() {
				Eventually(cf.Cf("create-service", serviceOffering, planA, instanceA), cf_helpers.CfTimeout).Should(gexec.Exit(0))
				Eventually(cf.Cf("create-service", serviceOffering, planB, instanceB), cf_helpers.CfTimeout).Should(gexec.Exit(0))

				cf_helpers.AwaitServiceCreation(instanceA)
				cf_helpers.AwaitServiceCreation(instanceB)
			})

			AfterEach(func() {
				Eventually(cf.Cf("delete-service", instanceA, "-f"), cf_helpers.CfTimeout).Should(gexec.Exit())
				Eventually(cf.Cf("delete-service", instanceB, "-f"), cf_helpers.CfTimeout).Should(gexec.Exit())
				Eventually(cf.Cf("delete-service", instanceC, "-f"), cf_helpers.CfTimeout).Should(gexec.Exit())

				cf_helpers.AwaitServiceDeletion(instanceA)
				cf_helpers.AwaitServiceDeletion(instanceB)
				cf_helpers.AwaitServiceDeletion(instanceC)
			})

			It("respects global quotas", func() {
				By("creating a service when quota is maxed")
				session := cf.Cf("create-service", serviceOffering, planB, instanceC)
				Eventually(session, cf_helpers.CfTimeout).Should(gexec.Exit())

				if session.ExitCode() == 0 {
					By("waiting for creation to finish, global quota was not enforced.")
					cf_helpers.AwaitServiceCreation(instanceC)
				}

				Expect(session).To(gbytes.Say(globalQuotaError))

				By("deleting instance to free up global quota")
				Eventually(cf.Cf("delete-service", instanceB, "-f"), cf_helpers.CfTimeout).Should(gexec.Exit(0))
				cf_helpers.AwaitServiceDeletion(instanceB)

				By("creating a service instance with newly freed quota")
				Eventually(cf.Cf("create-service", serviceOffering, planB, instanceC), cf_helpers.CfTimeout).Should(gexec.Exit(0))
				cf_helpers.AwaitServiceCreation(instanceC)

			})
		})
	})
})
