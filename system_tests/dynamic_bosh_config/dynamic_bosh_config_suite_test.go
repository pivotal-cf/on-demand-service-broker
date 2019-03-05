package dynamic_bosh_config_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

func TestDynamicBoshConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DynamicBoshConfig Suite")
}

var (
	serviceInstanceName string
	brokerInfo          bosh.BrokerInfo
)

var _ = BeforeSuite(func() {
	uniqueID := uuid.New()[:6]
	brokerInfo = bosh.DeployAndRegisterBroker(
		"-bosh-config-"+uniqueID,
		service_helpers.Redis,
		[]string{"update_service_catalog.yml"})
})

var _ = AfterSuite(func() {
	bosh.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
})
