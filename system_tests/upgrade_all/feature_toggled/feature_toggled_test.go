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

package feature_toggled_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/upgrade_all"
)

var _ = Describe("upgrade-all-service-instances errand using all the features available", func() {

	const canaryOrg = "canary_org"
	const canarySpace = "canary_space"

	var (
		brokerInfo         bosh_helpers.BrokerInfo
		uniqueID           string
		nonCanariesDetails []upgrade_all.AppDetails
		canaryDetails      upgrade_all.AppDetails
		standardOrg        string
		standardSpace      string
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

		standardOrg = os.Getenv("CF_ORG")
		standardSpace = os.Getenv("CF_SPACE")
		cf_helpers.TargetOrgAndSpace(standardOrg, standardSpace)
	})

	AfterEach(func() {
		cf_helpers.TargetOrgAndSpace(canaryOrg, canarySpace)
		cf_helpers.UnbindAndDeleteApp(canaryDetails.AppName, canaryDetails.ServiceName)
		cf_helpers.DeleteService(canaryDetails.ServiceName)

		cf_helpers.TargetOrgAndSpace(standardOrg, standardSpace)
		for _, appDtls := range nonCanariesDetails {
			cf_helpers.UnbindAndDeleteApp(appDtls.AppName, appDtls.ServiceName)
			cf_helpers.DeleteService(appDtls.ServiceName)
		}

		bosh_helpers.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)

		cf_helpers.DeleteSpace(canaryOrg, canarySpace)
		cf_helpers.DeleteOrg(canaryOrg)

		cf_helpers.TargetOrgAndSpace(standardOrg, standardSpace)
	})

	It("succeeds", func() {
		nonCanaryServices := 2
		planName := "redis-with-post-deploy"

		appDtlsCh := make(chan upgrade_all.AppDetails, nonCanaryServices)
		appPath := cf_helpers.GetAppPath(service_helpers.Redis)

		upgrade_all.PerformInParallel(func() {
			appDtlsCh <- upgrade_all.DeployService(brokerInfo.ServiceOffering, planName, appPath)
		}, nonCanaryServices)

		close(appDtlsCh)
		for dtls := range appDtlsCh {
			nonCanariesDetails = append(nonCanariesDetails, dtls)
		}

		cf_helpers.TargetOrgAndSpace(canaryOrg, canarySpace)
		canaryDetails = upgrade_all.DeployService(brokerInfo.ServiceOffering, planName, appPath)
		cf_helpers.TargetOrgAndSpace(standardOrg, standardSpace)

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

		By("running the upgrade-all errand")
		session := bosh_helpers.RunErrand(brokerInfo.DeploymentName, "upgrade-all-service-instances")

		By("upgrading the canary instance first, followed by the rest in parallel")
		Expect(session).To(SatisfyAll(
			gbytes.Say(`\[%s\] Starting to process service instance`, canaryDetails.ServiceGUID),
			gbytes.Say(`\[%s\] Result: Service Instance operation success`, canaryDetails.ServiceGUID),
			gbytes.Say("FINISHED CANARIES"),
			gbytes.Say(`Processing all instances`),
			gbytes.Say(`Starting to process service instance`),
			gbytes.Say(`Starting to process service instance`),
			gbytes.Say(`Result: Service Instance operation success`),
			gbytes.Say(`Result: Service Instance operation success`),
		))

		By("checking the other service instance upgrade completed", func() {
			Expect(string(session.Out.Contents())).To(
				ContainSubstring(`FINISHED PROCESSING Status: SUCCESS`),
			)
		})

		for _, appDtls := range append(nonCanariesDetails, canaryDetails) {
			By("verifying the update changes were applied to the instance", func() {
				manifest := bosh_helpers.GetManifest(appDtls.ServiceDeploymentName)
				instanceGroupProperties := bosh_helpers.FindInstanceGroupProperties(&manifest, "redis")
				Expect(instanceGroupProperties["redis"].(map[interface{}]interface{})["persistence"]).To(Equal("no"))
			})

			By("checking apps still have access to the data previously stored in their service", func() {
				Expect(cf_helpers.GetFromTestApp(appDtls.AppURL, "uuid")).To(Equal(appDtls.Uuid))
			})

			By("checking lifecycle errands executed as expected", func() {
				expectedBoshTasksOrder := []string{"create deployment", "run errand", "create deployment", "run errand", "create deployment", "run errand"}

				boshTasks := bosh_helpers.TasksForDeployment(appDtls.ServiceDeploymentName)
				Expect(boshTasks).To(HaveLen(4))

				for i, task := range boshTasks {
					Expect(task.State).To(Equal(boshdirector.TaskDone))
					Expect(task.Description).To(ContainSubstring(expectedBoshTasksOrder[3-i]))
				}
			})
		}
	})
})
