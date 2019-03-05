package lifecycle_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/pivotal-cf/on-demand-service-broker/system_tests/lifecycle/basic_lifecycle_tests"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

var _ = Describe("Kafka Lifecycle Tests", func() {
	It("can perform the basic service lifecycle", func() {
		BasicLifecycleTest(service_helpers.Kafka, brokerInfo, "kafka-small", dopplerAddress)
	})
})
