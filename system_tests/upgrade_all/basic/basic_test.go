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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
	. "github.com/pivotal-cf/on-demand-service-broker/system_tests/upgrade_all/shared"
)

var serviceInstances []*TestService
var canaryServiceInstances []*TestService

var dataPersistenceEnabled bool

var _ = Describe("upgrade-all-service-instances errand", func() {
	var (
		spaceName string
	)

	BeforeEach(func() {
		spaceName = ""
		config.CurrentPlan = "dedicated-vm"
		dataPersistenceEnabled = true
		serviceInstances = []*TestService{}
		CfTargetSpace(config.CfSpace)
	})

	AfterEach(func() {
		CfTargetSpace(config.CfSpace)
		DeleteServiceInstances(serviceInstances, dataPersistenceEnabled)
		if spaceName != "" {
			CfTargetSpace(spaceName)
			DeleteServiceInstances(canaryServiceInstances, dataPersistenceEnabled)
			CfDeleteSpace(spaceName)
		}
		config.BoshClient.DeployODB(*config.OriginalBrokerManifest)
	})

	It("exits 1 when the upgrader fails", func() {
		brokerManifest := config.BoshClient.GetManifest(config.BrokerBoshDeploymentName)
		serviceInstances = CreateServiceInstances(config, dataPersistenceEnabled)

		By("causing an upgrade error")
		testPlan := ExtractPlanProperty(config.CurrentPlan, brokerManifest)

		redisServer := testPlan["instance_groups"].([]interface{})[0].(map[interface{}]interface{})
		redisServer["vm_type"] = "doesntexist"

		By("deploying the broken broker manifest")
		config.BoshClient.DeployODB(*brokerManifest)

		boshOutput := config.BoshClient.RunErrandWithoutCheckingSuccess(config.BrokerBoshDeploymentName, "upgrade-all-service-instances", []string{}, "")
		Expect(boshOutput.ExitCode).To(Equal(1))
		Expect(boshOutput.StdOut).To(ContainSubstring("Operation failed"))
	})

	It("when there are no service instances provisioned, upgrade-all-service-instances runs successfully", func() {
		brokerManifest := config.BoshClient.GetManifest(config.BrokerBoshDeploymentName)

		UpdatePlanProperties(brokerManifest, config)
		MigrateJobProperty(brokerManifest, config)

		By("deploying the modified broker manifest")
		config.BoshClient.DeployODB(*brokerManifest)

		By("logging stdout to the errand output")
		boshOutput := config.BoshClient.RunErrand(config.BrokerBoshDeploymentName, "upgrade-all-service-instances", []string{}, "")
		Expect(boshOutput.ExitCode).To(Equal(0))
		Expect(boshOutput.StdOut).To(ContainSubstring("STARTING OPERATION"))
	})

	It("when there are multiple service instances provisioned, upgrade-all-service-instances runs successfully", func() {
		brokerManifest := config.BoshClient.GetManifest(config.BrokerBoshDeploymentName)
		serviceInstances = CreateServiceInstances(config, dataPersistenceEnabled)

		UpdatePlanProperties(brokerManifest, config)
		MigrateJobProperty(brokerManifest, config)

		By("deploying the modified broker manifest")
		config.BoshClient.DeployODB(*brokerManifest)

		By("logging stdout to the errand output")
		boshOutput := config.BoshClient.RunErrand(config.BrokerBoshDeploymentName, "upgrade-all-service-instances", []string{}, "")
		Expect(boshOutput.StdOut).To(ContainSubstring("STARTING OPERATION"))

		for _, service := range serviceInstances {
			deploymentName := GetServiceDeploymentName(service.Name)
			manifest := config.BoshClient.GetManifest(deploymentName)

			By("ensuring data still exists", func() {
				Expect(cf.GetFromTestApp(service.AppURL, "foo")).To(Equal("bar"))
			})

			By(fmt.Sprintf("upgrading instance '%s'", service.Name))
			instanceGroupProperties := bosh_helpers.FindInstanceGroupProperties(manifest, "redis")
			Expect(instanceGroupProperties["redis"].(map[interface{}]interface{})["persistence"]).To(Equal("no"))
		}
	})
})
