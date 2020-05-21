// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrade_deployment_tests

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"gopkg.in/yaml.v2"
)

var _ = Describe("Upgrading deployment", func() {
	var (
		brokerInfo bosh_helpers.BrokerInfo
		appDtls    AppDetails

		prevODBReleaseName     string
		prevAdapterReleaseName string
	)

	BeforeEach(func() {
		uniqueID := uuid.New()[:6]

		prevODBReleaseName = envMustHave("PREVIOUS_ODB_RELEASE_NAME")
		prevAdapterReleaseName = envMustHave("PREVIOUS_ADAPTER_RELEASE_NAME")

		brokerInfo = bosh_helpers.DeployAndRegisterBroker(
			"-upgrade-odb-"+uniqueID,
			bosh_helpers.BrokerDeploymentOptions{
				BrokerTLS:          true,
				ODBReleaseName:     prevODBReleaseName,
				ODBVersion:         envMustHave("PREVIOUS_ODB_VERSION"),
				AdapterVersion:     envMustHave("PREVIOUS_ADAPTER_VERSION"),
				AdapterReleaseName: prevAdapterReleaseName,
			},
			service_helpers.Redis,
			[]string{
				"service_catalog.yml",
				"add_update_all_certificate.yml",
				"deprecated_uaa_auth.yml",
			},
		)

	})

	AfterEach(func() {
		By("deleting the app")
		session := cf_helpers.CfWithTimeout(cf_helpers.CfTimeout, "delete", appDtls.AppName, "-f", "-r")
		Expect(session).To(gexec.Exit(0))

		By("ensuring the service instance is deleted")
		session = cf_helpers.CfWithTimeout(cf_helpers.CfTimeout, "delete-service", appDtls.ServiceName, "-f")
		Expect(session).To(gexec.Exit())
		cf_helpers.AwaitServiceDeletion(appDtls.ServiceName)

		bosh_helpers.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
	})

	It("Upgrades from an older release", func() {
		appDtls = CreateServiceAndApp(brokerInfo.ServiceName, "dedicated-vm")

		odbReleaseName := "on-demand-service-broker-" + os.Getenv("DEV_ENV")
		adapterReleaseName := "redis-example-service-adapter-" + os.Getenv("DEV_ENV")
		latestODBVersion := bosh_helpers.GetLatestReleaseVersion(odbReleaseName)
		latestAdapterVersion := bosh_helpers.GetLatestReleaseVersion(adapterReleaseName)

		By("redeploying ODB with latest version", func() {
			manifestStr := bosh_helpers.GetManifestString(brokerInfo.DeploymentName)
			manifestStr = strings.ReplaceAll(manifestStr, prevODBReleaseName, odbReleaseName)
			manifestStr = strings.ReplaceAll(manifestStr, prevAdapterReleaseName, adapterReleaseName)

			var manifest bosh.BoshManifest
			Expect(yaml.Unmarshal([]byte(manifestStr), &manifest)).To(Succeed())

			changeReleaseVersion("on-demand-service-broker", &manifest, latestODBVersion)
			changeReleaseVersion("redis-example-service-adapter", &manifest, latestAdapterVersion)

			bosh_helpers.RedeployBroker(brokerInfo.DeploymentName, brokerInfo.URI, manifest)
		})

		By("ensuring the version in the manifest was updated", func() {
			deployedManifest := bosh_helpers.GetManifest(brokerInfo.DeploymentName)

			odbRelease := versionOfRelease(odbReleaseName, deployedManifest)
			Expect(odbRelease).To(Equal(latestODBVersion))

			adapterRelease := versionOfRelease(adapterReleaseName, deployedManifest)
			Expect(adapterRelease).To(Equal(latestAdapterVersion))
		})

		By("exercising the service instance", func() {
			cf_helpers.PutToTestApp(appDtls.AppURL, "foo", "bar")
		})

		By("running the upgrade errand", func() {
			session := bosh_helpers.RunErrand(brokerInfo.DeploymentName, "upgrade-all-service-instances")
			Expect(session.ExitCode()).To(Equal(0))
			Expect(session).To(gbytes.Say("FINISHED PROCESSING Status: SUCCESS"))
		})

		By("updating the service instance", func() {
			session := cf_helpers.CfWithTimeout(cf_helpers.CfTimeout, "update-service", appDtls.ServiceName, "-c", `{"maxclients": 60}`)
			Expect(session).To(gexec.Exit(0))
			cf_helpers.AwaitServiceUpdate(appDtls.ServiceName)
		})

		By("running the delete all errand", func() {
			taskOutput := bosh_helpers.RunErrand(brokerInfo.DeploymentName, "delete-all-service-instances")
			Expect(taskOutput.ExitCode()).To(Equal(0))
			cf_helpers.AwaitServiceDeletion(appDtls.ServiceName)
		})
	})
})

func changeReleaseVersion(releaseName string, manifest *bosh.BoshManifest, releaseVersion string) {
	for index, release := range manifest.Releases {
		if strings.Contains(release.Name, releaseName) {
			manifest.Releases[index].Version = releaseVersion
			return
		}
	}
	Fail("No release found for " + releaseName)
}

func versionOfRelease(releaseName string, manifest bosh.BoshManifest) string {
	for index, release := range manifest.Releases {
		if strings.Contains(release.Name, releaseName) {
			return manifest.Releases[index].Version
		}
	}

	Fail("No release found for " + releaseName)
	return "not-found"
}

type AppDetails struct {
	UUID                  string
	AppURL                string
	AppName               string
	ServiceName           string
	ServiceGUID           string
	ServiceDeploymentName string
}

// TODO move this somewhere universal and use in upgrade_all as well
// TODO rename ServiceName to ServiceInstanceName
func CreateServiceAndApp(serviceOffering, planName string) AppDetails {
	uniqueID := uuid.New()[:8]
	serviceName := "service-" + uniqueID
	appName := "app-" + uniqueID
	cf_helpers.CreateService(serviceOffering, planName, serviceName, "")
	serviceGUID := cf_helpers.ServiceInstanceGUID(serviceName)

	appPath := cf_helpers.GetAppPath(service_helpers.Redis)
	appURL := cf_helpers.PushAndBindApp(appName, serviceName, appPath)
	cf_helpers.PutToTestApp(appURL, "uuid", uniqueID)

	return AppDetails{
		UUID:                  uniqueID,
		AppURL:                appURL,
		AppName:               appName,
		ServiceName:           serviceName,
		ServiceGUID:           serviceGUID,
		ServiceDeploymentName: "service-instance_" + serviceGUID,
	}
}
