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

var _ = Describe("quotas", func() {
	var instanceA string
	var instanceB string

	BeforeEach(func() {
		instanceA = fmt.Sprintf("instance-%s", uuid.New()[:7])
		instanceB = fmt.Sprintf("instance-%s", uuid.New()[:7])
	})

	Describe("global quotas", func() {
		const (
			planWithGlobalQuota = "plan-with-global-quota"
			globalQuotaError    = `global quotas \[limited_resource: \(limit 1, used 1, requires 1\)\] would be exceeded by this deployment`
		)

		Context("when the global limit is reached during provision", func() {
			BeforeEach(func() {
				Expect(cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithGlobalQuota, instanceA)).To(gexec.Exit(0))
				cf.AwaitServiceCreation(instanceA)
			})

			AfterEach(func() {
				cf.AwaitInProgressOperations(instanceA)
				cf.AwaitInProgressOperations(instanceB)

				Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceA, "-f")).To(gexec.Exit())
				Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceB, "-f")).To(gexec.Exit())

				cf.AwaitServiceDeletion(instanceA)
				cf.AwaitServiceDeletion(instanceB)
			})

			It("respects global quotas", func() {
				By("creating a service when quota is maxed", func() {
					session := cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithGlobalQuota, instanceB)
					Expect(session).To(gexec.Exit())
					if session.ExitCode() == 0 {
						cf.AwaitServiceCreation(instanceB)
						Fail("global quota was not enforced")
					}
					Expect(session).To(gbytes.AnySay(globalQuotaError))
				})

				By("deleting instance to free up global quota", func() {
					Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceA, "-f")).To(gexec.Exit(0))
					cf.AwaitServiceDeletion(instanceA)
				})

				By("creating a service instance with newly freed quota", func() {
					Expect(cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithGlobalQuota, instanceB)).To(gexec.Exit(0))
					cf.AwaitServiceCreation(instanceB)
				})
			})
		})

		Context("when the global limit is reached during update", func() {
			const (
				planWithNoCost = "plan-with-no-cost"
			)

			BeforeEach(func() {
				Expect(cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithNoCost, instanceA)).To(gexec.Exit(0))
				Expect(cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithGlobalQuota, instanceB)).To(gexec.Exit(0))
				cf.AwaitServiceCreation(instanceA)
				cf.AwaitServiceCreation(instanceB)
			})

			AfterEach(func() {
				cf.AwaitInProgressOperations(instanceA)
				cf.AwaitInProgressOperations(instanceB)

				Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceA, "-f")).To(gexec.Exit())
				Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceB, "-f")).To(gexec.Exit())

				cf.AwaitServiceDeletion(instanceA)
				cf.AwaitServiceDeletion(instanceB)
			})

			It("respects plan quotas when updating", func() {
				By("updating a service when quota is maxed", func() {
					session := cf.CfWithTimeout(cf.CfTimeout, "update-service", instanceA, "-p", planWithGlobalQuota)
					Expect(session).To(gexec.Exit())

					if session.ExitCode() == 0 {
						cf.AwaitServiceCreation(instanceB)
						Fail("global quota was not enforced")
					}

					Expect(session).To(gbytes.AnySay(globalQuotaError))
				})

				By("deleting instance to free up plan quota", func() {
					Expect(cf.Cf("delete-service", instanceB, "-f")).To(gexec.Exit(0))
					cf.AwaitServiceDeletion(instanceB)
				})

				By("updating a service instance with newly freed quota", func() {
					Expect(cf.Cf("update-service", instanceA, "-p", planWithGlobalQuota)).To(gexec.Exit(0))
					cf.AwaitServiceUpdate(instanceA)
				})
			})
		})
	})

	Describe("plan quotas", func() {
		const (
			planWithQuota    = "plan-with-quota"
			planWithoutQuota = "plan-with-no-cost"
		)

		const (
			planQuotaError = `plan quotas \[plan_specific_limited_resource: \(limit 5, used 3, requires 3\)\] would be exceeded by this deployment`
		)

		Context("when the plan limit is reached during provision", func() {
			BeforeEach(func() {
				Expect(cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithQuota, instanceA)).To(gexec.Exit(0))
				cf.AwaitServiceCreation(instanceA)
			})

			AfterEach(func() {
				cf.AwaitInProgressOperations(instanceA)
				cf.AwaitInProgressOperations(instanceB)

				Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceA, "-f")).To(gexec.Exit())
				Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceB, "-f")).To(gexec.Exit())

				cf.AwaitServiceDeletion(instanceA)
				cf.AwaitServiceDeletion(instanceB)
			})

			It("respects plan quotas", func() {
				By("creating a service when quota is maxed", func() {
					session := cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithQuota, instanceB)
					Expect(session).To(gexec.Exit())

					if session.ExitCode() == 0 {
						cf.AwaitServiceCreation(instanceB)
						Fail("plan quota was not enforced")
					}

					Expect(session).To(gbytes.AnySay(planQuotaError))
				})

				By("deleting instance to free up plan quota", func() {
					Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceA, "-f")).To(gexec.Exit(0))
					cf.AwaitServiceDeletion(instanceA)
				})

				By("creating a service instance with newly freed quota", func() {
					Expect(cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithQuota, instanceB)).To(gexec.Exit(0))
					cf.AwaitServiceCreation(instanceB)
				})
			})

		})

		Context("when the plan limit is reached during update", func() {
			BeforeEach(func() {
				Expect(cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithoutQuota, instanceA)).To(gexec.Exit(0))
				Expect(cf.CfWithTimeout(cf.CfTimeout, "create-service", brokerInfo.ServiceName, planWithQuota, instanceB)).To(gexec.Exit(0))
				cf.AwaitServiceCreation(instanceA)
				cf.AwaitServiceCreation(instanceB)
			})

			AfterEach(func() {
				cf.AwaitInProgressOperations(instanceA)
				cf.AwaitInProgressOperations(instanceB)

				Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceA, "-f")).To(gexec.Exit())
				Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceB, "-f")).To(gexec.Exit())

				cf.AwaitServiceDeletion(instanceA)
				cf.AwaitServiceDeletion(instanceB)
			})

			It("respects plan quotas when updating", func() {
				By("update a service when quota is maxed", func() {
					session := cf.CfWithTimeout(cf.CfTimeout, "update-service", instanceA, "-p", planWithQuota)
					Expect(session).To(gexec.Exit())

					if session.ExitCode() == 0 {
						cf.AwaitServiceCreation(instanceB)
						Fail("Plan quota was not enforced")
					}

					Expect(session).To(gbytes.AnySay(planQuotaError))
				})

				By("deleting instance to free up plan quota", func() {
					Expect(cf.CfWithTimeout(cf.CfTimeout, "delete-service", instanceB, "-f")).To(gexec.Exit(0))
					cf.AwaitServiceDeletion(instanceB)
				})

				By("updating a service instance with newly freed quota", func() {
					Expect(cf.Cf("update-service", instanceA, "-p", planWithQuota)).To(gexec.Exit(0))
					cf.AwaitServiceUpdate(instanceA)
				})
			})
		})
	})
})
