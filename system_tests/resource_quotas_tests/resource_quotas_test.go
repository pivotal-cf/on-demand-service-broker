// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package resource_quotas_tests

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/gbytes"
)

var instanceA string
var instanceB string

var _ = Describe("quotas", func() {
	BeforeEach(func() {
		instanceA = fmt.Sprintf("instance-%s", uuid.New()[:7])
		instanceB = fmt.Sprintf("instance-%s", uuid.New()[:7])
	})

	Describe("global quotas", func() {
		const (
			dedicatedVMPlan        = "dedicated-vm" //Global quota: IPs 1; 1 instance uses 1 IP
			dedicatedHighMemVMPlan = "dedicated-high-memory-vm"
		)

		const (
			globalQuotaError = `global quotas \[ips: \(limit 1, used 1, requires 1\)\] would be exceeded by this deployment`
		)

		Context("when the global limit is reached during provision", func() {
			BeforeEach(func() {
				Eventually(cf.Cf("create-service", serviceOffering, dedicatedVMPlan, instanceA), cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceCreation(instanceA)
			})

			AfterEach(func() {
				cf.AwaitInProgressOperations(instanceA)
				cf.AwaitInProgressOperations(instanceB)

				Eventually(cf.Cf("delete-service", instanceA, "-f"), cf.CfTimeout).Should(gexec.Exit())
				Eventually(cf.Cf("delete-service", instanceB, "-f"), cf.CfTimeout).Should(gexec.Exit())

				cf.AwaitServiceDeletion(instanceA)
				cf.AwaitServiceDeletion(instanceB)
			})

			It("respects global quotas", func() {
				By("creating a service when quota is maxed")
				session := cf.Cf("create-service", serviceOffering, dedicatedVMPlan, instanceB)
				Eventually(session, cf.CfTimeout).Should(gexec.Exit())

				if session.ExitCode() == 0 {
					By("waiting for creation to finish, global quota was not enforced.")
					cf.AwaitServiceCreation(instanceB)
				}

				Expect(session).To(gbytes.AnySay(globalQuotaError))

				By("deleting instance to free up global quota")
				Eventually(cf.Cf("delete-service", instanceA, "-f"), cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceDeletion(instanceA)

				By("creating a service instance with newly freed quota")
				Eventually(cf.Cf("create-service", serviceOffering, dedicatedVMPlan, instanceB), cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceCreation(instanceB)
			})
		})

		Context("when the global limit is reached during update", func() {
			BeforeEach(func() {
				Eventually(cf.Cf("create-service", serviceOffering, dedicatedHighMemVMPlan, instanceA), cf.CfTimeout).Should(gexec.Exit(0))
				Eventually(cf.Cf("create-service", serviceOffering, dedicatedVMPlan, instanceB), cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceCreation(instanceA)
				cf.AwaitServiceCreation(instanceB)
			})

			AfterEach(func() {
				cf.AwaitInProgressOperations(instanceA)
				cf.AwaitInProgressOperations(instanceB)

				Eventually(cf.Cf("delete-service", instanceA, "-f"), cf.CfTimeout).Should(gexec.Exit())
				Eventually(cf.Cf("delete-service", instanceB, "-f"), cf.CfTimeout).Should(gexec.Exit())

				cf.AwaitServiceDeletion(instanceA)
				cf.AwaitServiceDeletion(instanceB)
			})

			It("respects plan quotas when updating", func() {
				By("update a service when quota is maxed")
				session := cf.Cf("update-service", instanceA, "-p", dedicatedVMPlan)
				Eventually(session, cf.CfTimeout).Should(gexec.Exit())

				if session.ExitCode() == 0 {
					By("waiting for creation to finish, plan quota was not enforced.")
					cf.AwaitServiceCreation(instanceB)
				}

				Expect(session).To(gbytes.AnySay(globalQuotaError))

				By("deleting instance to free up plan quota")
				Eventually(cf.Cf("delete-service", instanceB, "-f"), cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceDeletion(instanceB)

				By("updating a service instance with newly freed quota")
				Eventually(cf.Cf("update-service", instanceA, "-p", dedicatedVMPlan)).Should(gexec.Exit(0))
				cf.AwaitServiceUpdate(instanceA)
			})
		})
	})

	Describe("plan quotas", func() {
		const (
			planWithQuota    = "dedicated-high-memory-vm" //Quota: memory 50; 1 instance uses 40
			planWithoutQuota = "dedicated-vm"
		)

		const (
			planQuotaError = `plan quotas \[memory: \(limit 50, used 40, requires 40\)\] would be exceeded by this deployment`
		)

		Context("when the plan limit is reached during provision", func() {
			BeforeEach(func() {
				Eventually(cf.Cf("create-service", serviceOffering, planWithQuota, instanceA), cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceCreation(instanceA)
			})

			AfterEach(func() {
				cf.AwaitInProgressOperations(instanceA)
				cf.AwaitInProgressOperations(instanceB)

				Eventually(cf.Cf("delete-service", instanceA, "-f"), cf.CfTimeout).Should(gexec.Exit())
				Eventually(cf.Cf("delete-service", instanceB, "-f"), cf.CfTimeout).Should(gexec.Exit())

				cf.AwaitServiceDeletion(instanceA)
				cf.AwaitServiceDeletion(instanceB)
			})

			It("respects plan quotas", func() {
				By("creating a service when quota is maxed")
				session := cf.Cf("create-service", serviceOffering, planWithQuota, instanceB)
				Eventually(session, cf.CfTimeout).Should(gexec.Exit())

				if session.ExitCode() == 0 {
					By("waiting for creation to finish, plan quota was not enforced.")
					cf.AwaitServiceCreation(instanceB)
				}

				Expect(session).To(gbytes.AnySay(planQuotaError))

				By("deleting instance to free up plan quota")
				Eventually(cf.Cf("delete-service", instanceA, "-f"), cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceDeletion(instanceA)

				By("creating a service instance with newly freed quota")
				Eventually(cf.Cf("create-service", serviceOffering, planWithQuota, instanceB), cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceCreation(instanceB)
			})

		})

		Context("when the plan limit is reached during update", func() {
			BeforeEach(func() {
				Eventually(cf.Cf("create-service", serviceOffering, planWithoutQuota, instanceA), cf.CfTimeout).Should(gexec.Exit(0))
				Eventually(cf.Cf("create-service", serviceOffering, planWithQuota, instanceB), cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceCreation(instanceA)
				cf.AwaitServiceCreation(instanceB)
			})

			AfterEach(func() {
				cf.AwaitInProgressOperations(instanceA)
				cf.AwaitInProgressOperations(instanceB)

				Eventually(cf.Cf("delete-service", instanceA, "-f"), cf.CfTimeout).Should(gexec.Exit())
				Eventually(cf.Cf("delete-service", instanceB, "-f"), cf.CfTimeout).Should(gexec.Exit())

				cf.AwaitServiceDeletion(instanceA)
				cf.AwaitServiceDeletion(instanceB)
			})

			It("respects plan quotas when updating", func() {
				By("update a service when quota is maxed")
				session := cf.Cf("update-service", instanceA, "-p", planWithQuota)
				Eventually(session, cf.CfTimeout).Should(gexec.Exit())

				if session.ExitCode() == 0 {
					By("waiting for creation to finish, plan quota was not enforced.")
					cf.AwaitServiceCreation(instanceB)
				}

				Expect(session).To(gbytes.AnySay(planQuotaError))

				By("deleting instance to free up plan quota")
				Eventually(cf.Cf("delete-service", instanceB, "-f"), cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceDeletion(instanceB)

				By("updating a service instance with newly freed quota")
				Eventually(cf.Cf("update-service", instanceA, "-p", planWithQuota)).Should(gexec.Exit(0))
				cf.AwaitServiceUpdate(instanceA)
			})
		})
	})
})
