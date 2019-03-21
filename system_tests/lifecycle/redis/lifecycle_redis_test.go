package lifecycle_test

import (
	. "github.com/onsi/ginkgo"
	"github.com/pborman/uuid"
	. "github.com/pivotal-cf/on-demand-service-broker/system_tests/lifecycle/all_lifecycle_tests"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

var _ = Describe("Redis Lifecycle Tests", func() {
	Context("for a basic configuration", func() {
		BeforeEach(func() {
			uniqueID := uuid.New()[:6]

			brokerInfo = bosh_helpers.DeployAndRegisterBroker(
				"-redis-lifecycle-"+uniqueID,
				deploymentOptions,
				service_helpers.Redis,
				[]string{"basic_service_catalog.yml"},
			)
		})

		It("can complete successfully", func() {
			BasicLifecycleTest(
				service_helpers.Redis,
				brokerInfo,
				"redis-small",
				"redis-medium",
				`{ "maxclients": 100 }`)
		})
	})

	Context("for a configuration with features enabled", func() {
		BeforeEach(func() {
			uniqueID := uuid.New()[:6]

			brokerInfo = bosh_helpers.DeployAndRegisterBroker(
				"-redis-lifecycle-"+uniqueID,
				deploymentOptions,
				service_helpers.Redis,
				[]string{
					metricsOpsFile,
					"basic_service_catalog.yml",
					"add_binding_with_dns.yml",
				},
			)
		})

		FIt("can complete successfully", func() {
			FeatureToggledLifecycleTest(
				service_helpers.Redis,
				brokerInfo,
				"redis-small",
				"redis-medium",
				`{ "maxclients": 100 }`,
				dopplerAddress)
		})
	})
})
