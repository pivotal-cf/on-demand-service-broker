// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrade_instances_errand_tests

import (
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

type testService struct {
	Name    string
	AppName string
	AppURL  string
}

var serviceInstances = []*testService{
	{Name: uuid.New(), AppName: uuid.New()},
	{Name: uuid.New(), AppName: uuid.New()},
}

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

		brokerJobs := brokerManifest.InstanceGroups[0].Jobs
		serviceAdapterJob := extractServiceAdapterJob(brokerJobs)
		Expect(serviceAdapterJob).ToNot(BeNil(), "Couldn't find service adapter job in existing manifest")

		newRedisServerName := "redis"
		serviceAdapterJob.Properties["redis_instance_group_name"] = newRedisServerName

		testPlanInstanceGroup := testPlan["instance_groups"].([]interface{})[0].(map[interface{}]interface{})

		oldRedisServerName := testPlanInstanceGroup["name"]

		testPlanInstanceGroup["name"] = newRedisServerName
		testPlanInstanceGroup["migrated_from"] = []map[interface{}]interface{}{
			{"name": oldRedisServerName},
		}

		By("deploying the modified broker manifest")
		boshClient.DeployODB(*brokerManifest)

		By("logging stdout to the errand output")
		boshOutput := boshClient.RunErrand(brokerBoshDeploymentName, "upgrade-all-service-instances", "")
		Expect(boshOutput.StdOut).To(ContainSubstring("STARTING UPGRADES"))

		for _, service := range serviceInstances {
			deploymentName := getServiceDeploymentName(service.Name)
			manifest := boshClient.GetManifest(deploymentName)

			By("ensuring data still exists", func() {
				Expect(cf_helpers.GetFromTestApp(service.AppURL, "foo")).To(Equal("bar"))
			})

			By(fmt.Sprintf("upgrading instance '%s'", service.Name))
			Expect(manifest.InstanceGroups[0].Properties["redis"].(map[interface{}]interface{})["persistence"]).To(Equal("no"))

			if boshSupportsLifecycleErrands {
				By(fmt.Sprintf("running the post-deploy errand for instance '%s'", service.Name))
				boshTasks := boshClient.GetTasksForDeployment(getServiceDeploymentName(service.Name))
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

	var wg sync.WaitGroup

	for _, service := range serviceInstances {
		wg.Add(1)

		go func(ts *testService, wg *sync.WaitGroup) {
			Eventually(cf.Cf("create-service", serviceOffering, currentPlan, ts.Name), cf_helpers.CfTimeout).Should(
				gexec.Exit(0),
			)
			cf_helpers.AwaitServiceCreation(ts.Name)

			By("pushing an app and binding to it")
			ts.AppURL = cf_helpers.PushAndBindApp(
				ts.AppName,
				ts.Name,
				path.Join(ciRootPath, exampleAppDirName),
			)

			By("adding data to the service instance")
			cf_helpers.PutToTestApp(ts.AppURL, "foo", "bar")

			wg.Done()
		}(service, &wg)
	}

	wg.Wait()
}

func deleteServiceInstances() {
	By("running the delete all errand")
	taskOutput := boshClient.RunErrand(brokerBoshDeploymentName, "delete-all-service-instances", "")
	Expect(taskOutput.ExitCode).To(Equal(0))

	var wg sync.WaitGroup

	for _, service := range serviceInstances {
		wg.Add(1)
		go func(ts *testService, wg *sync.WaitGroup) {
			By("deleting the corresponding app")
			Eventually(cf.Cf("delete", ts.AppName, "-f", "-r"), cf_helpers.CfTimeout).Should(gexec.Exit(0))

			By("ensuring the service instance is deleted")
			cf_helpers.AwaitServiceDeletion(service.Name)

			wg.Done()
		}(service, &wg)
	}

	wg.Wait()
}

func extractPlanProperty(planName string, manifest *bosh.BoshManifest) map[interface{}]interface{} {
	var testPlan map[interface{}]interface{}

	brokerJob := manifest.InstanceGroups[0].Jobs[0]
	serviceCatalog := brokerJob.Properties["service_catalog"].(map[interface{}]interface{})

	for _, plan := range serviceCatalog["plans"].([]interface{}) {
		if plan.(map[interface{}]interface{})["name"] == planName {
			testPlan = plan.(map[interface{}]interface{})
		}
	}

	return testPlan
}

func extractServiceAdapterJob(jobs []bosh.Job) bosh.Job {
	for _, j := range jobs {
		if j.Name == "service-adapter" {
			return j
		}
	}

	return bosh.Job{}
}

func getServiceDeploymentName(serviceInstanceName string) string {
	getInstanceDetailsCmd := cf.Cf("service", serviceInstanceName, "--guid")
	Eventually(getInstanceDetailsCmd, cf_helpers.CfTimeout).Should(gexec.Exit(0))
	serviceInstanceID := strings.TrimSpace(string(getInstanceDetailsCmd.Out.Contents()))
	return fmt.Sprintf("%s%s", "service-instance_", serviceInstanceID)
}
