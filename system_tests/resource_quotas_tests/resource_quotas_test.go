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
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var _ = Describe("quotas", func() {
	const (
		planA = "dedicated-vm" //Quota: IPs 1; 1 instance uses 1 IP
	)

	var (
		instanceA = fmt.Sprintf("instance-%s", uuid.New()[:7])
	)

	Describe("global quotas", func() {
		const (
			globalQuotaError = "global quota of ips: 1 would be exceeded by this deployment"
		)
		var instanceB = fmt.Sprintf("instance-%s", uuid.New()[:7])

		Context("when the global limit is reached", func() {
			BeforeEach(func() {
				Eventually(cf.Cf("create-service", serviceOffering, planA, instanceA), cf.CfTimeout).Should(gexec.Exit(0))
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
				session := cf.Cf("create-service", serviceOffering, planA, instanceB)
				Eventually(session, cf.CfTimeout).Should(gexec.Exit())

				if session.ExitCode() == 0 {
					By("waiting for creation to finish, global quota was not enforced.")
					cf.AwaitServiceCreation(instanceB)
				}

				Expect(session).To(gbytes.Say(globalQuotaError))

				By("deleting instance to free up global quota")
				Eventually(cf.Cf("delete-service", instanceA, "-f"), cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceDeletion(instanceA)

				By("creating a service instance with newly freed quota")
				Eventually(cf.Cf("create-service", serviceOffering, planA, instanceB), cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceCreation(instanceB)
			})
		})
	})
})
