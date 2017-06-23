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
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

var serviceInstances = []string{uuid.New(), uuid.New()}

var _ = Describe("upgrade-all-service-instances errand", func() {
	BeforeEach(func() {
		createServiceInstances()
	})

	AfterEach(func() {
		deleteServiceInstances()
		boshClient.DeployODB(*originalBrokerManifest)
	})

	It("exits 1 when the upgrader fails", func() {
		By("causing an upgrade error")
		brokerManifest := boshClient.GetManifest(brokerBoshDeploymentName)
		testPlan := extractPlanProperty(currentPlan, brokerManifest)

		redisServer := testPlan["instance_groups"].([]interface{})[0].(map[interface{}]interface{})
		redisServer["vm_type"] = "doesntexist"

		By("deploying the broken broker manifest")
		boshClient.DeployODB(*brokerManifest)
		boshOutput := boshClient.RunErrandWithoutCheckingSuccess(brokerBoshDeploymentName, "upgrade-all-service-instances", "")
		Expect(boshOutput.ExitCode).To(Equal(1))
		Expect(boshOutput.StdOut).To(ContainSubstring("Upgrade failed for service instance"))
	})

	It("upgrades all service instances", func() {
		By("causing pending changes for the service instance")
		brokerManifest := boshClient.GetManifest(brokerBoshDeploymentName)

		testPlan := extractPlanProperty(currentPlan, brokerManifest)
		testPlan["properties"] = map[interface{}]interface{}{"persistence": false}

		By("deploying the modified broker manifest")
		boshClient.DeployODB(*brokerManifest)

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

func createServiceInstances() {
	if boshSupportsLifecycleErrands {
		currentPlan = "lifecycle-post-deploy-plan"
	} else {
		currentPlan = "dedicated-vm"
	}
	for _, i := range serviceInstances {
		Eventually(cf.Cf("create-service", serviceOffering, currentPlan, i), cf_helpers.CfTimeout).Should(gexec.Exit(0))
	}
	for _, i := range serviceInstances {
		cf_helpers.AwaitServiceCreation(i)
	}
}

func deleteServiceInstances() {
	for _, i := range serviceInstances {
		Eventually(cf.Cf("delete-service", i, "-f"), cf_helpers.CfTimeout).Should(gexec.Exit(0))
	}
	for _, i := range serviceInstances {
		cf_helpers.AwaitServiceDeletion(i)
	}
}

func extractPlanProperty(planName string, manifest *bosh.BoshManifest) map[interface{}]interface{} {
	var testPlan map[interface{}]interface{}

	brokerJob := manifest.InstanceGroups[0].Jobs[0]
	serviceCatalog := brokerJob.Properties["service_catalog"].(map[interface{}]interface{})

	for _, plan := range serviceCatalog["plans"].([]interface{}) {
		if plan.(map[interface{}]interface{})["name"] == currentPlan {
			testPlan = plan.(map[interface{}]interface{})
		}
	}

	return testPlan
}

func getServiceDeploymentName(serviceInstanceName string) string {
	getInstanceDetailsCmd := cf.Cf("service", serviceInstanceName, "--guid")
	Eventually(getInstanceDetailsCmd, cf_helpers.CfTimeout).Should(gexec.Exit(0))
	serviceInstanceID := strings.TrimSpace(string(getInstanceDetailsCmd.Out.Contents()))
	return fmt.Sprintf("%s%s", "service-instance_", serviceInstanceID)
}
