package service_catalog_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
)

func TestServiceCatalog(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ServiceCatalog Suite")
}

var (
	brokerInfo bosh.BrokerInfo
)

var _ = BeforeSuite(func() {
	brokerInfo = bosh.DeployAndRegisterBroker("catalog")
})

var _ = AfterSuite(func() {
	bosh.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
})
