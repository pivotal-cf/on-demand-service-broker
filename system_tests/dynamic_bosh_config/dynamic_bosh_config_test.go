package dynamic_bosh_config_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"

	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

var _ = Describe("DynamicBoshConfig", func() {
	var serviceId string

	BeforeEach(func() {
		serviceId = ""
	})

	AfterEach(func() {
		if serviceId != "" {
			bosh_helpers.DeleteBOSHConfig("cloud", serviceId)
		}
	})

	It("handles bosh configs during the lifecycle of a service instance", func() {
		serviceInstanceName = "service" + brokerInfo.TestSuffix
		boshConfig := fmt.Sprintf(`{"vm_extensions_config": "vm_extensions: [{name: vm-ext%s}]"}`, brokerInfo.TestSuffix)

		By("creating a service with the right bosh config")
		cf.CreateService(brokerInfo.ServiceName, "redis-with-post-deploy", serviceInstanceName, boshConfig)
		serviceId = "service-instance_" + cf.GetServiceInstanceGUID(serviceInstanceName)

		configDetails, err := bosh_helpers.GetBOSHConfig("cloud", serviceId)
		Expect(err).NotTo(HaveOccurred())
		Expect(configDetails).To(ContainSubstring("name: vm-ext" + brokerInfo.TestSuffix))

		By("updating the service with a new bosh config")
		newBoshConfig := fmt.Sprintf(`{"vm_extensions_config": "vm_extensions: [{name: vm-new-ext%s}]"}`, brokerInfo.TestSuffix)
		session := cf.CfWithTimeout(cf.CfTimeout, "update-service", serviceInstanceName, "-c", newBoshConfig)
		Expect(session).To(gexec.Exit(0))
		cf.AwaitServiceUpdate(serviceInstanceName)

		configDetails, err = bosh_helpers.GetBOSHConfig("cloud", serviceId)
		Expect(err).NotTo(HaveOccurred())
		Expect(configDetails).ToNot(ContainSubstring("name: vm-ext" + brokerInfo.TestSuffix))
		Expect(configDetails).To(ContainSubstring("name: vm-new-ext" + brokerInfo.TestSuffix))

		By("deleting the bosh config when service is deleted")
		cf.DeleteService(serviceInstanceName)

		_, err = bosh_helpers.GetBOSHConfig("cloud", serviceId)
		Expect(err).To(HaveOccurred(), "cloud config wasn't deleted during DeleteService")
	})
})
