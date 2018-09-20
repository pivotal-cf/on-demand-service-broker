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

package canary_filter_siapi_test

import (
	"fmt"
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/on-demand-service-broker/system_tests/upgrade_all/shared"
)

var _ = Describe("parallel upgrade-all errand with canaries and SI API", func() {
	var (
		serviceInstances       []*TestService
		dataPersistenceEnabled bool
		canaryServiceInstances []*TestService
	)

	BeforeEach(func() {
		config.CurrentPlan = "dedicated-vm"
		dataPersistenceEnabled = false
		serviceInstances = []*TestService{}
		CfTargetSpace(config.CfSpace)
	})

	AfterEach(func() {
		CfTargetSpace(config.CfSpace)
		DeleteServiceInstances(serviceInstances, dataPersistenceEnabled)
		config.BoshClient.DeployODB(*config.OriginalBrokerManifest)
	})

	It("when canaries from an org and space are required, they upgrade before the rest", func() {
		var nonCanaryInstances []*TestService

		brokerManifest := config.BoshClient.GetManifest(config.BrokerBoshDeploymentName)
		serviceInstances = CreateServiceInstances(config, dataPersistenceEnabled)

		canaryServiceInstances = serviceInstances[len(serviceInstances)-1 : len(serviceInstances)]
		nonCanaryInstances = serviceInstances[:len(serviceInstances)-1]

		upgradeInstanceProperties := FindUpgradeAllServiceInstancesProperties(brokerManifest)
		filterParams := map[string]string{}
		for k, v := range upgradeInstanceProperties["canary_selection_params"].(map[interface{}]interface{}) {
			filterParams[k.(string)] = v.(string)
		}

		UpdateServiceInstancesAPI(brokerManifest, canaryServiceInstances, filterParams, config)

		By("logging stdout to the errand output")
		boshOutput := config.BoshClient.RunErrand(config.BrokerBoshDeploymentName, "upgrade-all-service-instances", []string{}, "")

		logMatcher := "(?s)STARTING CANARY UPGRADES(.*)FINISHED CANARY UPGRADES(.*)FINISHED UPGRADES"
		re := regexp.MustCompile(logMatcher)
		matches := re.FindStringSubmatch(boshOutput.StdOut)
		for _, instance := range canaryServiceInstances {
			Expect(matches[1]).To(ContainSubstring(instance.GUID), fmt.Sprintf("Canary instances %v not present in canary instances upgraded", canaryServiceInstances))
			Expect(matches[2]).NotTo(ContainSubstring(instance.GUID), fmt.Sprintf("Canary instances %v present in non-canary instances upgraded", canaryServiceInstances))
		}
		for _, instance := range nonCanaryInstances {
			Expect(matches[1]).NotTo(ContainSubstring(instance.GUID), fmt.Sprintf("Non-canary instances %v present in canary instances upgraded", nonCanaryInstances))
			Expect(matches[2]).To(ContainSubstring(instance.GUID), fmt.Sprintf("Non-canary instances %v not present in non-canary instances upgraded", nonCanaryInstances))
		}
	})
})
