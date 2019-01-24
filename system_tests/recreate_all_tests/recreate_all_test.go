package recreate_all_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

var _ = Describe("The recreate-all errand", func() {
	It("recreates all instances and DOES NOT run their post-deploy errands", func() {
		boshServiceInstanceName := broker.InstancePrefix + cf.GetServiceInstanceGUID(serviceInstanceName)
		oldVMID := bosh.VMIDForDeployment(boshServiceInstanceName)
		Expect(oldVMID).ToNot(BeEmpty(), "unexpected empty vm id")

		bosh.RunErrand(brokerInfo.DeploymentName, "recreate-all-service-instances")

		newVMID := bosh.VMIDForDeployment(boshServiceInstanceName)
		Expect(oldVMID).ToNot(Equal(newVMID), "VM was not recreated")

		boshTasks := bosh.TasksForDeployment(boshServiceInstanceName)
		Expect(boshTasks).To(HaveLen(3), "expected bosh deploy, errand, recreate (reversed)")
		Expect(boshTasks[0].Description).ToNot(HavePrefix("run errand health-check"))
	})
})
