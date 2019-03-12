package lifecycle_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/pivotal-cf/on-demand-service-broker/system_tests/lifecycle/basic_lifecycle_tests"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

var _ = Describe("Redis Lifecycle Tests", func() {
	Context("HTTPS Broker", func() {
		It("can perform the basic service lifecycle", func() {
			BasicLifecycleTest(service_helpers.Redis, brokerInfo, "redis-small", dopplerAddress)
		})
	})
})
