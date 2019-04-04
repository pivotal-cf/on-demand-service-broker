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

package upgrade_all_test

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

var _ = Describe("upgrade-all-service-instances errand using all the features available", func() {

	const canaryOrg = "canary_org"
	const canarySpace = "canary_space"

	var (
		brokerInfo     bosh_helpers.BrokerInfo
		uniqueID       string
		appDetailsList []appDetails
	)

	BeforeEach(func() {
		uniqueID = uuid.New()[:8]

		brokerInfo = bosh_helpers.DeployAndRegisterBroker(
			"feature-toggled-upgrade-"+uniqueID,
			bosh_helpers.BrokerDeploymentOptions{},
			service_helpers.Redis,
			[]string{
				"service_catalog_with_lifecycle.yml",
				// "add_canary_filter.yml",
			},
		)

		// cf_helpers.CreateOrg(canaryOrg)
		// cf_helpers.CreateSpace(canarySpace)
	})

	AfterEach(func() {
		for _, appDtls := range appDetailsList {
			cf_helpers.UnbindAndDeleteApp(appDtls.appName, appDtls.serviceName)
		}

		bosh_helpers.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)

		// cf_helpers.DeleteSpace(canarySpace)
		// cf_helpers.DeleteOrg(canaryOrg)
	})

	FIt("succeeds", func() {
		serviceNumber := 3
		appDtlsCh := make(chan appDetails, serviceNumber)
		appPath := cf_helpers.GetAppPath(service_helpers.Redis)

		// cf_helpers.TargetOrgAndSpace(canaryOrg, canarySpace)
		performInParallel(func() {
			defer GinkgoRecover()

			uuid := uuid.New()[:8]
			serviceName := "service-" + uuid
			appName := "app-" + uuid
			cf_helpers.CreateService(brokerInfo.ServiceOffering, "redis-with-post-deploy", serviceName, "")
			serviceGUID := cf_helpers.ServiceInstanceGUID(serviceName)
			appURL := cf_helpers.PushAndBindApp(appName, serviceName, appPath)
			cf_helpers.PutToTestApp(appURL, "uuid", uuid)

			appDtlsCh <- appDetails{
				uuid:                  uuid,
				appURL:                appURL,
				appName:               appName,
				serviceName:           serviceName,
				serviceDeploymentName: "service-instance_" + serviceGUID,
			}
		}, serviceNumber)
		close(appDtlsCh)

		for dtls := range appDtlsCh {
			appDetailsList = append(appDetailsList, dtls)
		}

		By("changing the name of instance group and disabling persistence", func() {
			brokerInfo = bosh_helpers.DeployAndRegisterBroker(
				"feature-toggled-upgrade-"+uniqueID,
				bosh_helpers.BrokerDeploymentOptions{},
				service_helpers.Redis,
				[]string{
					"service_catalog_with_lifecycle_updated.yml",
					// "add_canary_filter.yml",
				})
		})

		By("logging stdout to the errand output")
		session := bosh_helpers.RunErrand(brokerInfo.DeploymentName, "upgrade-all-service-instances")
		Expect(session).To(gbytes.Say("STARTING OPERATION"))

		instanceGUIDs := getInstanceGUIDs(session.Out.Contents())
		fmt.Printf("instanceGUIDs = %+v\n", instanceGUIDs)

		b := gbytes.NewBuffer()
		b.Write(session.Out.Contents())

		By("upgrading the canary instance first")
		Expect(b).To(SatisfyAll(
			gbytes.Say(fmt.Sprintf(`\[%s\] Starting to process service instance`, instanceGUIDs[0])),
			gbytes.Say(fmt.Sprintf(`\[%s\] Result: Service Instance operation success`, instanceGUIDs[0])),
		))

		By("expecting the remaining (less than maxInFlight) instances to start before any completion")
		Expect(b).To(SatisfyAll(
			gbytes.Say(fmt.Sprintf(`\[%s\] Starting to process service instance`, instanceGUIDs[1])),
			gbytes.Say(fmt.Sprintf(`\[%s\] Starting to process service instance`, instanceGUIDs[2])),
			gbytes.Say("Result: Service Instance operation success"),
		))

		Expect(string(session.Out.Contents())).To(SatisfyAll(
			ContainSubstring(fmt.Sprintf(`[%s] Result: Service Instance operation success`, instanceGUIDs[1])),
			ContainSubstring(fmt.Sprintf(`[%s] Result: Service Instance operation success`, instanceGUIDs[2])),
		))

		Expect(b).To(gbytes.Say("FINISHED PROCESSING Status: SUCCESS"))

		for _, appDtls := range appDetailsList {
			By("verifying the update changes were applied to the instance", func() {
				manifest := bosh_helpers.GetManifest(appDtls.serviceDeploymentName)
				instanceGroupProperties := bosh_helpers.FindInstanceGroupProperties(&manifest, "redis")
				Expect(instanceGroupProperties["redis"].(map[interface{}]interface{})["persistence"]).To(Equal("no"))
			})

			By("checking apps still have access to the data previously stored in their service", func() {
				Expect(cf_helpers.GetFromTestApp(appDtls.appURL, "uuid")).To(Equal(appDtls.uuid))
			})

			By("checking lifecycle errands executed as expected", func() {
				expectedBoshTasksOrder := []string{"create deployment", "run errand", "create deployment", "run errand", "create deployment", "run errand"}

				boshTasks := bosh_helpers.TasksForDeployment(appDtls.serviceDeploymentName)
				Expect(boshTasks).To(HaveLen(4))

				Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[0].Description).To(ContainSubstring(expectedBoshTasksOrder[3]))

				Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[1].Description).To(ContainSubstring(expectedBoshTasksOrder[2]))

				Expect(boshTasks[2].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[2].Description).To(ContainSubstring(expectedBoshTasksOrder[1]))

				Expect(boshTasks[3].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[3].Description).To(ContainSubstring(expectedBoshTasksOrder[0]))
			})
		}
	})
})

var _ = Describe("get instance guids", func() {
	It("works", func() {

		b := []byte(`
Task 1276 | 15:22:14 | Fetching logs for broker/75cf97ec-e27f-49db-821b-e92f72ef0163 (0): Finding and packing log files (00:00:01)
broker/75cf97ec-e27f-49db-821b-e92f72ef0163
0
[upgrade-all-service-instances] 2019/04/03 15:18:07.271933 [upgrade-all] STARTING OPERATION with 3 concurrent workers
[upgrade-all-service-instances] 2019/04/03 15:18:07.392318 [upgrade-all] Service Instances: c9d7ecf0-6727-40c7-8d3b-18100cdd2876 cfb6043c-2473-4481-ad0f-d3be07bb6bf3 bc2ba8f9-5aae-4fff-bc50-08f0d7e95ef6    
[upgrade-all-service-instances] 2019/04/03 15:18:07.392340 [upgrade-all] Total Service Instances found: 3
`)

		guids := getInstanceGUIDs(b)
		Expect(guids).To(HaveLen(3))
		fmt.Printf("guids = %+v\n", guids)
		Expect(guids[2]).To(Equal("bc2ba8f9-5aae-4fff-bc50-08f0d7e95ef6"))

	})

})

func getInstanceGUIDs(logOutput []byte) []string {
	scanner := bufio.NewScanner(bytes.NewReader(logOutput))

	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, "Service Instances: ")
		if idx < 0 {
			continue
		}
		regex := regexp.MustCompile(`\b[a-f0-9-]+\b`)
		matches := regex.FindAllString(line[idx:], -1)
		return matches
	}

	return nil
}
