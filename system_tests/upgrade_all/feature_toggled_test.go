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
	"fmt"

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
	const standardOrg = "system"
	const standardSpace = "system"

	var (
		brokerInfo         bosh_helpers.BrokerInfo
		uniqueID           string
		nonCanariesDetails []appDetails
	)

	BeforeEach(func() {
		uniqueID = uuid.New()[:8]

		brokerInfo = bosh_helpers.DeployAndRegisterBroker(
			"feature-toggled-upgrade-"+uniqueID,
			bosh_helpers.BrokerDeploymentOptions{},
			service_helpers.Redis,
			[]string{
				"service_catalog_with_lifecycle.yml",
				"add_canary_filter.yml",
			},
		)

		cf_helpers.CreateOrg(canaryOrg)
		cf_helpers.CreateSpace(canaryOrg, canarySpace)

		cf_helpers.TargetOrgAndSpace(standardOrg, standardSpace)
	})

	AfterEach(func() {
		for _, appDtls := range nonCanariesDetails {
			cf_helpers.UnbindAndDeleteApp(appDtls.appName, appDtls.serviceName)
		}

		bosh_helpers.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)

		cf_helpers.DeleteSpace(canaryOrg, canarySpace)
		cf_helpers.DeleteOrg(canaryOrg)

		cf_helpers.TargetOrgAndSpace(standardOrg, standardSpace)
	})

	It("succeeds", func() {
		nonCanaryServices := 2
		appDtlsCh := make(chan appDetails, nonCanaryServices)
		appPath := cf_helpers.GetAppPath(service_helpers.Redis)

		performInParallel(func() {
			defer GinkgoRecover()

			appDtlsCh <- deployService(brokerInfo.ServiceOffering, appPath)
		}, nonCanaryServices)

		close(appDtlsCh)

		cf_helpers.TargetOrgAndSpace(canaryOrg, canarySpace)
		canaryDetails := deployService(brokerInfo.ServiceOffering, appPath)
		cf_helpers.TargetOrgAndSpace(standardOrg, standardSpace)

		for dtls := range appDtlsCh {
			nonCanariesDetails = append(nonCanariesDetails, dtls)
		}

		By("changing the name of instance group and disabling persistence", func() {
			brokerInfo = bosh_helpers.DeployAndRegisterBroker(
				"feature-toggled-upgrade-"+uniqueID,
				bosh_helpers.BrokerDeploymentOptions{},
				service_helpers.Redis,
				[]string{
					"service_catalog_with_lifecycle_updated.yml",
					"add_canary_filter.yml",
				})
		})

		By("logging stdout to the errand output")
		session := bosh_helpers.RunErrand(brokerInfo.DeploymentName, "upgrade-all-service-instances")
		Expect(session).To(gbytes.Say("STARTING OPERATION"))

		b := gbytes.NewBuffer()
		b.Write(session.Out.Contents())

		By("upgrading the canary instance first")
		Expect(b).To(SatisfyAll(
			gbytes.Say(fmt.Sprintf(`\[%s\] Starting to process service instance`, canaryDetails.serviceGUID)),
			gbytes.Say(fmt.Sprintf(`\[%s\] Result: Service Instance operation success`, canaryDetails.serviceGUID)),
		))

		By("expecting the remaining (less than maxInFlight) instances to start before any completion")
		Expect(b).To(SatisfyAll(
			gbytes.Say(fmt.Sprintf(`\[%s\] Starting to process service instance`, nonCanariesDetails[0].serviceGUID)),
			gbytes.Say(fmt.Sprintf(`\[%s\] Starting to process service instance`, nonCanariesDetails[1].serviceGUID)),
			gbytes.Say("Result: Service Instance operation success"),
		))

		Expect(string(session.Out.Contents())).To(SatisfyAll(
			ContainSubstring(fmt.Sprintf(`[%s] Result: Service Instance operation success`, nonCanariesDetails[0].serviceGUID)),
			ContainSubstring(fmt.Sprintf(`[%s] Result: Service Instance operation success`, nonCanariesDetails[1].serviceGUID)),
		))

		Expect(b).To(gbytes.Say("FINISHED PROCESSING Status: SUCCESS"))

		for _, appDtls := range append(nonCanariesDetails, canaryDetails) {
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

func deployService(serviceOffering, appPath string) appDetails {
	uuid := uuid.New()[:8]
	serviceName := "service-" + uuid
	appName := "app-" + uuid
	cf_helpers.CreateService(serviceOffering, "redis-with-post-deploy", serviceName, "")
	serviceGUID := cf_helpers.ServiceInstanceGUID(serviceName)
	appURL := cf_helpers.PushAndBindApp(appName, serviceName, appPath)
	cf_helpers.PutToTestApp(appURL, "uuid", uuid)

	return appDetails{
		uuid:                  uuid,
		appURL:                appURL,
		appName:               appName,
		serviceName:           serviceName,
		serviceGUID:           serviceGUID,
		serviceDeploymentName: "service-instance_" + serviceGUID,
	}
}
