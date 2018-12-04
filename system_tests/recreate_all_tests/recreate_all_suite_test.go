package recreate_all_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

func TestRecreateAll(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RecreateAll Suite")
}

var (
	serviceInstanceName string
	brokerInfo          bosh.BrokerInfo
)

var _ = BeforeSuite(func() {
	brokerInfo = bosh.DeployAndRegisterBroker("recreate")
	serviceInstanceName = "service" + brokerInfo.TestSuffix
	cf.CreateService(brokerInfo.ServiceOffering, "redis-with-post-deploy", serviceInstanceName, "")
})

var _ = AfterSuite(func() {
	bosh.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
})
