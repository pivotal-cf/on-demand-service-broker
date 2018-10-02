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

package orphan_deployments_tests

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/siapi_helpers"
)

var _ = Describe("orphan deployments errand", func() {
	Context("when there are two deployments and one is an orphan", func() {
		var (
			orphanInstanceName           string
			anotherInstanceName          string
			orphanInstanceDeploymentName string
		)

		BeforeEach(func() {
			orphanInstanceName = uuid.New()[:7]
			anotherInstanceName = uuid.New()[:7]

			Eventually(cf.Cf("create-service", serviceOffering, "dedicated-vm", orphanInstanceName), cf.CfTimeout).Should(gexec.Exit(0))
			Eventually(cf.Cf("create-service", serviceOffering, "dedicated-vm", anotherInstanceName), cf.CfTimeout).Should(gexec.Exit(0))
			cf.AwaitServiceCreation(orphanInstanceName)
			cf.AwaitServiceCreation(anotherInstanceName)

			By("getting the service instances' GUIDs")
		})

		AfterEach(func() {
			Eventually(cf.Cf("delete-service", orphanInstanceName, "-f"), cf.CfTimeout).Should(gexec.Exit(0))
			Eventually(cf.Cf("delete-service", anotherInstanceName, "-f"), cf.CfTimeout).Should(gexec.Exit(0))
			cf.AwaitServiceDeletion(orphanInstanceName)
			cf.AwaitServiceDeletion(anotherInstanceName)
		})

		It("lists the orphan deployment", func() {
			notOrphanInstanceGUID := cf.ServiceInstanceGUID(anotherInstanceName)
			orphanInstanceGUID := cf.ServiceInstanceGUID(orphanInstanceName)
			orphanInstanceDeploymentName = fmt.Sprintf("service-instance_%s", orphanInstanceGUID)

			instancesToReturn := []service.Instance{
				{GUID: notOrphanInstanceGUID},
			}

			By("setting up SI api to report only one instance")
			err := siapi_helpers.UpdateServiceInstancesAPI(siapiConfig, instancesToReturn)
			Expect(err).ToNot(HaveOccurred())

			By("running the orphan-deployments errand")
			taskOutput := boshClient.RunErrandWithoutCheckingSuccess(brokerBoshDeploymentName, "orphan-deployments", []string{}, "")

			By("checking the errand task output")
			Expect(taskOutput.ExitCode).To(Equal(10), "expected to have exit code 10: some orphans found")
			Expect(taskOutput.StdOut).To(MatchJSON(fmt.Sprintf(`[{"deployment_name":"%s"}]`, orphanInstanceDeploymentName)))
			Expect(taskOutput.StdOut).NotTo(ContainSubstring(notOrphanInstanceGUID))
		})
	})
})
