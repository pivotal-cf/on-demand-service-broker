package canary_filter_test

import (
	"fmt"
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/on-demand-service-broker/system_tests/upgrade_all/shared"
)

var _ = Describe("parallel upgrade-all errand with canaries", func() {
	var (
		filterParams           map[string]string
		spaceName              string
		serviceInstances       []*TestService
		dataPersistenceEnabled bool
		canaryServiceInstances []*TestService
	)

	BeforeEach(func() {
		spaceName = ""
		config.CurrentPlan = "dedicated-vm"
		dataPersistenceEnabled = false
		serviceInstances = []*TestService{}
		filterParams = map[string]string{}
		CfTargetSpace(config.CfSpace)
	})

	AfterEach(func() {
		CfTargetSpace(config.CfSpace)
		DeleteServiceInstances(serviceInstances, dataPersistenceEnabled)
		CfTargetSpace(spaceName)
		DeleteServiceInstances(canaryServiceInstances, dataPersistenceEnabled)
		CfDeleteSpace(spaceName)
		config.BoshClient.DeployODB(*config.OriginalBrokerManifest)
	})

	It("when canaries from an org and space are required, they upgrade before the rest", func() {
		brokerManifest := config.BoshClient.GetManifest(config.BrokerBoshDeploymentName)
		upgradeInstanceProperties := FindUpgradeAllServiceInstancesProperties(brokerManifest)

		serviceInstances = CreateServiceInstances(config, dataPersistenceEnabled)

		filterParams := map[string]string{}
		for k, v := range upgradeInstanceProperties["canary_selection_params"].(map[interface{}]interface{}) {
			filterParams[k.(string)] = v.(string)
		}

		spaceName = filterParams["cf_space"]
		CfCreateSpace(spaceName)
		CfTargetSpace(spaceName)

		canaryServiceInstances = CreateServiceInstances(config, dataPersistenceEnabled)

		var nonCanaryInstances []*TestService
		nonCanaryInstances = serviceInstances

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
