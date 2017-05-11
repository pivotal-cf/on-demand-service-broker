// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrade_instances_errand_tests

import (
	"fmt"
	"strings"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var _ = Describe("upgrade-all-service-instances errand", func() {
	It("upgrades all service instances", func() {
		By("logging stdout to the errand output")
		boshOutput := boshClient.RunErrand(brokerBoshDeploymentName, "upgrade-all-service-instances", "")
		Expect(boshOutput.StdOut).To(ContainSubstring("STARTING UPGRADES"))

		for _, instanceName := range serviceInstances {
			deploymentName := getServiceDeploymentName(instanceName)
			manifest := boshClient.GetManifest(deploymentName)

			By(fmt.Sprintf("upgrading instance '%s'", instanceName))
			Expect(manifest.InstanceGroups[0].Properties["redis"].(map[interface{}]interface{})["persistence"]).To(Equal("no"))

			if boshSupportsLifecycleErrands {
				By(fmt.Sprintf("running the post-deploy errand for instance '%s'", instanceName))
				boshTasks := boshClient.GetTasksForDeployment(getServiceDeploymentName(instanceName))
				Expect(boshTasks).To(HaveLen(4))

				Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[0].Description).To(ContainSubstring("run errand"))

				Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[1].Description).To(ContainSubstring("create deployment"))

				Expect(boshTasks[2].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[2].Description).To(ContainSubstring("run errand"))

				Expect(boshTasks[3].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[3].Description).To(ContainSubstring("create deployment"))
			}
		}
	})
})

func getServiceDeploymentName(serviceInstanceName string) string {
	getInstanceDetailsCmd := cf.Cf("service", serviceInstanceName, "--guid")
	Eventually(getInstanceDetailsCmd, cf_helpers.CfTimeout).Should(gexec.Exit(0))
	serviceInstanceID := strings.TrimSpace(string(getInstanceDetailsCmd.Out.Contents()))
	return fmt.Sprintf("%s%s", "service-instance_", serviceInstanceID)
}
