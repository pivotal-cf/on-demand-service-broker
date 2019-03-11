package lifecycle_test

import (
	. "github.com/onsi/ginkgo"
	"github.com/pborman/uuid"
	. "github.com/pivotal-cf/on-demand-service-broker/system_tests/lifecycle/basic_lifecycle_tests"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

var _ = Describe("Redis Lifecycle Tests", func() {
	Context("HTTPS Broker", func() {
		BeforeEach(func() {
			uniqueID := uuid.New()[:6]

			brokerInfo = bosh_helpers.DeployAndRegisterBroker(
				"-redis-lifecycle-"+uniqueID,
				deploymentOptions,
				service_helpers.Redis,
				[]string{"basic_service_catalog.yml", "service_metrics.yml"},
			)
		})

		AfterEach(func() {
			bosh_helpers.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
		})

		It("can perform the basic service lifecycle", func() {
			BasicLifecycleTest(service_helpers.Redis, brokerInfo, "redis-small", dopplerAddress)
		})
	})
})
