package with_lifecycle_errands_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
	. "github.com/pivotal-cf/on-demand-service-broker/system_tests/upgrade_all/shared"
)

var serviceInstances []*TestService
var canaryServiceInstances []*TestService

var dataPersistenceEnabled bool

var _ = Describe("upgrade-all-service-instances errand", func() {
	var (
		filterParams map[string]string
		spaceName    string
	)

	BeforeEach(func() {
		spaceName = ""
		config.CurrentPlan = "lifecycle-post-deploy-plan"
		dataPersistenceEnabled = false
		serviceInstances = []*TestService{}
		filterParams = map[string]string{}
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

	It("upgrade-all-service-instances runs successfully", func() {
		brokerManifest := config.BoshClient.GetManifest(config.BrokerBoshDeploymentName)
		serviceInstances = CreateServiceInstances(config, dataPersistenceEnabled)

		UpdatePlanProperties(brokerManifest, config)
		MigrateJobProperty(brokerManifest, config)

		By("deploying the modified broker manifest")
		config.BoshClient.DeployODB(*brokerManifest)

		By("logging stdout to the errand output")
		boshOutput := config.BoshClient.RunErrand(config.BrokerBoshDeploymentName, "upgrade-all-service-instances", []string{}, "")
		Expect(boshOutput.StdOut).To(ContainSubstring("STARTING UPGRADES"))

		expectedBoshTasksOrder := []string{"create deployment", "run errand", "create deployment", "run errand", "create deployment", "run errand"}
		for _, service := range serviceInstances {
			deploymentName := GetServiceDeploymentName(service.Name)
			manifest := config.BoshClient.GetManifest(deploymentName)

			By(fmt.Sprintf("upgrading instance '%s'", service.Name))
			instanceGroupProperties := bosh_helpers.FindInstanceGroupProperties(manifest, "redis")
			Expect(instanceGroupProperties["redis"].(map[interface{}]interface{})["persistence"]).To(Equal("no"))

			By(fmt.Sprintf("running the post-deploy errand for instance '%s'", service.Name))
			boshTasks := config.BoshClient.GetTasksForDeployment(GetServiceDeploymentName(service.Name))
			Expect(boshTasks).To(HaveLen(4))

			Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
			Expect(boshTasks[0].Description).To(ContainSubstring(expectedBoshTasksOrder[3]))

			Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
			Expect(boshTasks[1].Description).To(ContainSubstring(expectedBoshTasksOrder[2]))

			Expect(boshTasks[2].State).To(Equal(boshdirector.TaskDone))
			Expect(boshTasks[2].Description).To(ContainSubstring(expectedBoshTasksOrder[1]))

			Expect(boshTasks[3].State).To(Equal(boshdirector.TaskDone))
			Expect(boshTasks[3].Description).To(ContainSubstring(expectedBoshTasksOrder[0]))
		}
	})
})
