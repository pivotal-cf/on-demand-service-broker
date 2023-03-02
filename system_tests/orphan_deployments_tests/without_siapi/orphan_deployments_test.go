// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package orphan_deployments_tests

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

var _ = Describe("orphan deployments errand", func() {
	Context("when there are two deployments and one is an orphan", func() {
		var (
			orphanInstanceName  string
			anotherInstanceName string
		)

		BeforeEach(func() {
			orphanInstanceName = "instance-to-purge-" + uuid.New()[:7]
			anotherInstanceName = "instance-to-keep-" + uuid.New()[:7]

			cf.CreateService(brokerInfo.ServiceName, "redis-orphan-without-siapi", orphanInstanceName, "")
			cf.CreateService(brokerInfo.ServiceName, "redis-orphan-without-siapi", anotherInstanceName, "")
		})

		AfterEach(func() {
			cf.DeleteService(orphanInstanceName)
			cf.DeleteService(anotherInstanceName)
		})

		It("lists the orphan deployment", func() {
			By("getting the service instances' GUIDs")
			orphanInstanceGUID := cf.ServiceInstanceGUID(orphanInstanceName)
			orphanInstanceDeploymentName := fmt.Sprintf("service-instance_%s", orphanInstanceGUID)
			anotherInstanceGUID := cf.ServiceInstanceGUID(anotherInstanceName)
			anotherInstanceDeploymentName := fmt.Sprintf("service-instance_%s", anotherInstanceGUID)

			By("purging one service instance")
			Expect(cf.CfWithTimeout(cf.CfTimeout, "purge-service-instance", orphanInstanceName, "-f")).To(gexec.Exit(0))

			By("running the orphan-deployments errand")
			session := bosh.RunErrand(brokerInfo.DeploymentName, "orphan-deployments", gexec.Exit(1))

			By("checking the errand task output")
			Expect(session.ExitCode()).To(Equal(1))
			Expect(string(session.Buffer().Contents())).To(SatisfyAll(
				ContainSubstring("Orphan BOSH deployments detected"),
				ContainSubstring(`{"deployment_name":"%s"}`, orphanInstanceDeploymentName),
				Not(ContainSubstring(anotherInstanceDeploymentName)),
			))

		})
	})
})
