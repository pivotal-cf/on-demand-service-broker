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

package basic_test

import (
	"fmt"

	"github.com/coreos/go-semver/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/upgrade_all"
)

var _ = Describe("upgrade-all-service-instances errand, basic operation", func() {
	Context("BOSH upgrade", func() {
		const instancesToTest = 2

		var (
			brokerSuffix            string
			brokerInfo              bosh_helpers.BrokerInfo
			brokerDeploymentOptions bosh_helpers.BrokerDeploymentOptions
			uniqueID                string
		)

		BeforeEach(func() {
			uniqueID = uuid.New()[:8]

			brokerDeploymentOptions = bosh_helpers.BrokerDeploymentOptions{BrokerTLS: true}

			brokerSuffix = "-basic-upgrade-with-bosh-" + uniqueID

			brokerInfo = bosh_helpers.DeployAndRegisterBroker(
				brokerSuffix,
				brokerDeploymentOptions,
				service_helpers.Redis,
				[]string{
					"service_catalog.yml",
					"remove_parallel_upgrade.yml",
					"update_upgrade_all_job.yml",
				})
		})

		AfterEach(func() {
			bosh_helpers.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
		})

		It("runs successfully when there are no service instances provisioned", func() {
			By("logging stdout to the errand output")
			session := bosh_helpers.RunErrand(brokerInfo.DeploymentName, "upgrade-all-service-instances")
			Expect(session).To(gbytes.Say("STARTING OPERATION"))
			Expect(session).To(gbytes.Say("FINISHED PROCESSING Status: SUCCESS"))
		})

		Context("upgrading some instances in series", func() {
			var appDetailsList []upgrade_all.AppDetails

			AfterEach(func() {
				for _, appDtls := range appDetailsList {
					cf_helpers.UnbindAndDeleteApp(appDtls.AppName, appDtls.ServiceName)
				}
			})

			It("succeeds", func() {
				createTestServiceInstancesAndApps(instancesToTest, brokerInfo.ServiceName)

				By("changing the name of instance group and disabling persistence", func() {
					brokerInfo = bosh_helpers.DeployAndRegisterBroker(
						brokerSuffix,
						brokerDeploymentOptions,
						service_helpers.Redis,
						[]string{
							"service_catalog_updated.yml",
							"remove_parallel_upgrade.yml",
							"update_upgrade_all_job.yml",
						})
				})

				By("running the upgrade-all errand", func() {
					session := bosh_helpers.RunErrand(brokerInfo.DeploymentName, "upgrade-all-service-instances")
					Expect(session).To(SatisfyAll(
						gbytes.Say("Upgrading all instances via BOSH"),
						gbytes.Say("STARTING OPERATION"),
						gbytes.Say("FINISHED PROCESSING Status: SUCCESS"),
						gbytes.Say("Number of successful operations: %d", instancesToTest),
						gbytes.Say("Number of skipped operations: 0"),
					))
				})

				for _, appDtls := range appDetailsList {
					By("verifying the update changes were applied to the instance", func() {
						manifest := bosh_helpers.GetManifest(appDtls.ServiceDeploymentName)
						instanceGroupProperties := bosh_helpers.FindInstanceGroupProperties(&manifest, "redis")
						Expect(instanceGroupProperties["redis"].(map[interface{}]interface{})["persistence"]).To(Equal("no"))
					})

					By("checking apps still have access to the data previously stored in their service", func() {
						Expect(cf_helpers.GetFromTestApp(appDtls.AppURL, "uuid")).To(Equal(appDtls.UUID))
					})
				}
			})
		})
	})

	Context("CF upgrade", func() {
		const instancesToTest = 3

		var (
			brokerSuffix            string
			appDetails              []upgrade_all.AppDetails
			brokerInfo              bosh_helpers.BrokerInfo
			brokerDeploymentOptions bosh_helpers.BrokerDeploymentOptions
			uniqueID                string
		)

		BeforeEach(func() {
			if hasSufficientCAPIRelease, CAPISemver := checkCAPIVersion(); hasSufficientCAPIRelease {
				Skip(fmt.Sprintf(`Single instance upgrade not possible in this version. CAPI v%s`, CAPISemver))
			}

			uniqueID = uuid.New()[:8]

			brokerDeploymentOptions = bosh_helpers.BrokerDeploymentOptions{BrokerTLS: false}

			brokerSuffix = "-basic-upgrade-with-cf-" + uniqueID

			brokerInfo = bosh_helpers.DeployAndRegisterBroker(
				brokerSuffix,
				brokerDeploymentOptions,
				service_helpers.Redis,
				[]string{
					"service_catalog.yml",
					"remove_parallel_upgrade.yml",
					"update_upgrade_all_job.yml",
					"add_maintenance_info.yml",
				},
				"--var", "version=1.7.8",
			)

			appDetails = createTestServiceInstancesAndApps(instancesToTest, brokerInfo.ServiceName)
		})

		AfterEach(func() {
			for _, appDtls := range appDetails {
				cf_helpers.UnbindAndDeleteApp(appDtls.AppName, appDtls.ServiceName)
			}

			bosh_helpers.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
		})

		Context("upgrading service instances", func() {
			It("succeeds", func() {
				By("updating service offering, re-deploy and re-register broker", func() {
					brokerInfo = bosh_helpers.DeployAndRegisterBroker(
						brokerSuffix,
						brokerDeploymentOptions,
						service_helpers.Redis,
						[]string{
							"service_catalog_updated.yml",
							"remove_parallel_upgrade.yml",
							"update_upgrade_all_job.yml",
							"add_maintenance_info.yml",
						},
						"--var", "version=1.7.9",
					)
				})

				By("verifying that the service instance is out-dated", func() {
					for i, app := range appDetails {
						identifier := fmt.Sprintf("instance %d", i)
						session := cf_helpers.Cf("service", app.ServiceName)
						Eventually(session).Should(gexec.Exit(0), identifier)
						Expect(session).To(SatisfyAll(gbytes.Say("Showing available upgrade details for this service")), identifier)
					}
				})

				By("upgrading one of the instances", func() {
					app := appDetails[0]
					cf_helpers.UpdateServiceWithUpgrade(app.ServiceName)
				})

				By("running the upgrade-all errand upgrades only the ones that have upgrade available", func() {
					session := bosh_helpers.RunErrand(brokerInfo.DeploymentName, "upgrade-all-service-instances")
					Expect(session).To(SatisfyAll(
						gbytes.Say("Upgrading all instances via CF"),
						gbytes.Say("STARTING OPERATION"),
						gbytes.Say("instance already up to date - operation skipped"),
						gbytes.Say("FINISHED PROCESSING Status: SUCCESS"),
						gbytes.Say("Number of successful operations: 2"),
						gbytes.Say("Number of skipped operations: 1"),
					))
				})

				By("assuring that CF knows the service instance is updated", func() {
					for i, app := range appDetails {
						identifier := fmt.Sprintf("instance %d", i)
						session := cf_helpers.Cf("service", app.ServiceName)
						Eventually(session).Should(gexec.Exit(0), identifier)
						Expect(session).To(gbytes.Say("There is no upgrade available for this service."), identifier)
					}
				})

				By("running the upgrade-all errand again", func() {
					session := bosh_helpers.RunErrand(brokerInfo.DeploymentName, "upgrade-all-service-instances")
					Expect(session).To(SatisfyAll(
						gbytes.Say("STARTING OPERATION"),
						gbytes.Say("FINISHED PROCESSING Status: SUCCESS"),
						gbytes.Say("Number of successful operations: 0"),
						gbytes.Say("Number of skipped operations: %d", instancesToTest),
					))
				})
			})
		})
	})
})

func createTestServiceInstancesAndApps(count int, serviceName string) (appDetailsList []upgrade_all.AppDetails) {
	appDtlsCh := make(chan upgrade_all.AppDetails, count)
	upgrade_all.PerformInParallel(func(planName string) {
		appDtls := upgrade_all.CreateServiceAndApp(serviceName, planName)
		appDtlsCh <- appDtls

		By("verifying that the persistence property starts as 'yes'", func() {
			manifest := bosh_helpers.GetManifest(appDtls.ServiceDeploymentName)
			instanceGroupProperties := bosh_helpers.FindInstanceGroupProperties(&manifest, "redis-server")
			Expect(instanceGroupProperties["redis"].(map[interface{}]interface{})["persistence"]).To(Equal("yes"))
		})
	}, count, upgrade_all.PlanNamesForParallelCreate{
		Items: []upgrade_all.PlanName{
			{Index: 0, Value: "dedicated-vm"},
			{Index: 1, Value: "dedicated-vm-with-post-deploy"},
			{Index: 2, Value: "dedicated-vm"},
		}})
	close(appDtlsCh)

	for dtls := range appDtlsCh {
		appDetailsList = append(appDetailsList, dtls)
	}

	return appDetailsList
}

func checkCAPIVersion() (bool, *semver.Version) {
	capiVersion := bosh_helpers.GetLatestReleaseVersion("capi")
	capiSemver := semver.New(capiVersion)
	return capiSemver.LessThan(*semver.New("1.84.0")), capiSemver
}
