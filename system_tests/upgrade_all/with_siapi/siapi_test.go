package parallel_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
	. "github.com/pivotal-cf/on-demand-service-broker/system_tests/upgrade_all/shared"
)

var _ = Describe("parallel upgrade-all errand and SIAPI", func() {
	var (
		filterParams           map[string]string
		spaceName              string
		serviceInstances       []*TestService
		dataPersistenceEnabled bool
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
		config.BoshClient.DeployODB(*config.OriginalBrokerManifest)
	})

	It("upgrade-all-service-instances runs successfully", func() {
		brokerManifest := config.BoshClient.GetManifest(config.BrokerBoshDeploymentName)
		serviceInstances = CreateServiceInstances(config, dataPersistenceEnabled)

		UpdateServiceInstancesAPI(brokerManifest, serviceInstances, filterParams, config)
		UpdatePlanProperties(brokerManifest, config)
		MigrateJobProperty(brokerManifest, config)

		By("deploying the modified broker manifest")
		config.BoshClient.DeployODB(*brokerManifest)

		By("logging stdout to the errand output")
		boshOutput := config.BoshClient.RunErrand(config.BrokerBoshDeploymentName, "upgrade-all-service-instances", []string{}, "")
		Expect(boshOutput.StdOut).To(ContainSubstring("STARTING UPGRADES"))

		instanceGUIDs := getInstanceGUIDs(boshOutput.StdOut)

		b := gbytes.NewBuffer()
		b.Write([]byte(boshOutput.StdOut))

		By("upgrading the canary instance first")
		Expect(b).To(SatisfyAll(
			gbytes.Say(fmt.Sprintf(`\[%s\] Starting to upgrade service instance`, instanceGUIDs[0])),
			gbytes.Say(fmt.Sprintf(`\[%s\] Result: Service Instance upgrade success`, instanceGUIDs[0])),
		))

		By("upgrading all the non-canary instances")
		Expect(b).To(SatisfyAll(
			gbytes.Say(fmt.Sprintf(`\[%s\] Starting to upgrade service instance`, instanceGUIDs[1])),
			gbytes.Say(fmt.Sprintf(`\[%s\] Starting to upgrade service instance`, instanceGUIDs[2])),
			gbytes.Say(fmt.Sprintf(`\[%s\] Result: Service Instance upgrade success`, instanceGUIDs[1])),
			gbytes.Say(fmt.Sprintf(`\[%s\] Result: Service Instance upgrade success`, instanceGUIDs[2])),
		))

		for _, service := range serviceInstances {
			deploymentName := GetServiceDeploymentName(service.Name)
			manifest := config.BoshClient.GetManifest(deploymentName)

			By(fmt.Sprintf("upgrading instance '%s'", service.Name))
			instanceGroupProperties := bosh_helpers.FindInstanceGroupProperties(manifest, "redis")
			Expect(instanceGroupProperties["redis"].(map[interface{}]interface{})["persistence"]).To(Equal("no"))

			By("running the tasks in the correct order")
			boshTasks := config.BoshClient.GetTasksForDeployment(GetServiceDeploymentName(service.Name))
			Expect(boshTasks).To(HaveLen(2))
		}
	})
})

func getInstanceGUIDs(logOutput string) []string {
	var instances []string
	lines := strings.Split(logOutput, "\n")

	for _, line := range lines {
		if strings.Contains(line, "Service Instances: ") {
			instances = strings.Split(line, " ")
			instances = instances[len(instances)-3 : len(instances)]
			return instances
		}
	}

	return instances
}
