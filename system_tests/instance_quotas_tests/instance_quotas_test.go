// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package instance_quotas_tests

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"

	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/gbytes"
)

var _ = Describe("quotas", func() {
	const (
		planWithLimit1 = "plan-with-limit-1"
		planWithLimit5 = "plan-with-limit-5"
	)

	var (
		instanceA = fmt.Sprintf("instanceA-%s", uuid.New()[:7])
		instanceB = fmt.Sprintf("instanceB-%s", uuid.New()[:7])
	)

	Describe("Service Instance Limits", func() {
		const planQuotaTemplate = "plan instance limit exceeded for service ID: %s. Total instances: %d"

		AfterEach(func() {
			cf.AwaitInProgressOperations(instanceA)
			cf.AwaitInProgressOperations(instanceB)

			Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceA, "-f")).To(gexec.Exit())
			Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceB, "-f")).To(gexec.Exit())

			cf.AwaitServiceDeletion(instanceA)
			cf.AwaitServiceDeletion(instanceB)
		})

		It("correctly enforces the quota", func() {
			By("respecting the plan quotas", func() {
				By("allowing SIs to be created up to the plan limit", func() {
					session := cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithLimit1, instanceA)
					Expect(session).To(gexec.Exit(0), "Create failed for "+planWithLimit1)
					cf.AwaitServiceCreation(instanceA)
				})

				By("denying a create-service when the limit has been reached", func() {
					session := cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithLimit1, instanceB)
					Expect(session).To(gexec.Exit())
					Expect(session).To(gbytes.AnySay(fmt.Sprintf(planQuotaTemplate, brokerInfo.ServiceID, 1)))
				})

				By("allowing a create-service after deleting instances", func() {
					By("deleting an instance", func() {
						session := cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceA, "-f")
						Expect(session).To(gexec.Exit(0))
						cf.AwaitServiceDeletion(instanceA)
					})

					By("successfully provisioning another instance", func() {
						session := cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithLimit1, instanceA)
						Expect(session).To(gexec.Exit(0))
						cf.AwaitServiceCreation(instanceA)
					})
				})

				By("respecting quotas when switching plans", func() {
					By("creating a instance of a different plan", func() {
						session := cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithLimit5, instanceB)
						Expect(session).To(gexec.Exit(0))
						cf.AwaitServiceCreation(instanceB)
					})

					By("updating to a plan with quotas maxed", func() {
						session := cf.CfWithTimeout(cf.CfTimeout, "update-service", instanceB, "-p", planWithLimit1)
						Expect(session).To(gexec.Exit())
						Expect(session).To(gbytes.AnySay(fmt.Sprintf(planQuotaTemplate, brokerInfo.ServiceID, 1)))
					})

					By("deleting instance to free up quota", func() {
						session := cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceA, "-f")
						Expect(session).To(gexec.Exit(0))
						cf.AwaitServiceDeletion(instanceA)
					})

					By("updating to plan with newly freed quota", func() {
						session := cf.CfWithTimeout(cf.CfTimeout, "update-service", instanceB, "-p", planWithLimit1)
						Expect(session).To(gexec.Exit(0))
						cf.AwaitServiceUpdate(instanceB)
					})
				})
			})
		})
	})

	Describe("global quotas", func() {
		const (
			globalQuotaTemplate = "global instance limit exceeded for service ID: %s. Total instances: %d"
		)

		instanceC := fmt.Sprintf("instanceC-%s", uuid.New()[:7])

		Context("when the global limit is reached", func() {
			BeforeEach(func() {
				Expect(cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithLimit1, instanceA)).To(gexec.Exit(0))
				Expect(cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithLimit5, instanceB)).To(gexec.Exit(0))

				cf.AwaitServiceCreation(instanceA)
				cf.AwaitServiceCreation(instanceB)
			})

			AfterEach(func() {
				cf.AwaitInProgressOperations(instanceA)
				cf.AwaitInProgressOperations(instanceB)
				cf.AwaitInProgressOperations(instanceC)

				Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceA, "-f")).To(gexec.Exit())
				Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceB, "-f")).To(gexec.Exit())
				Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceC, "-f")).To(gexec.Exit())

				cf.AwaitServiceDeletion(instanceA)
				cf.AwaitServiceDeletion(instanceB)
				cf.AwaitServiceDeletion(instanceC)
			})

			It("respects global quotas", func() {
				By("creating a service when quota is maxed")
				session := cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithLimit5, instanceC)
				Expect(session).To(gexec.Exit())

				if session.ExitCode() == 0 {
					By("waiting for creation to finish, global quota was not enforced.")
					cf.AwaitServiceCreation(instanceC)
				}

				Expect(session).To(gbytes.AnySay(fmt.Sprintf(globalQuotaTemplate, brokerInfo.ServiceID, 2)))

				By("deleting instance to free up global quota")
				Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceB, "-f")).To(gexec.Exit(0))
				cf.AwaitServiceDeletion(instanceB)

				By("creating a service instance with newly freed quota")
				Expect(cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithLimit5, instanceC)).To(gexec.Exit(0))
				cf.AwaitServiceCreation(instanceC)
			})
		})
	})
})
