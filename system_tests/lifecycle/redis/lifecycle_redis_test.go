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
		var brokerInfo bosh_helpers.BrokerInfo

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

		AfterEach(func() {
			bosh_helpers.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
		})
	})

	Context("for a configuration with features enabled", func() {
		var brokerInfo bosh_helpers.BrokerInfo

		BeforeEach(func() {
			if !bosh_helpers.BOSHSupportsLinksAPIForDNS() {
				Skip("PCF2.1 and lower versions do not support DNS links")
			}
			uniqueID := uuid.New()[:6]

			brokerInfo = bosh_helpers.DeployAndRegisterBroker(
				"-redis-lifecycle-"+uniqueID,
				deploymentOptions,
				service_helpers.Redis,
				[]string{
					metricsOpsFile,
					"basic_service_catalog.yml",
					"add_binding_with_dns.yml",
					"add_secure_binding.yml",
					"enable_telemetry.yml",
					"add_client_definition.yml",
				},
			)
		})

		It("can complete successfully", func() {
			FeatureToggledLifecycleTest(
				service_helpers.Redis,
				brokerInfo,
				"redis-small",
				"redis-medium",
				`{ "maxclients": 100 }`,
				dopplerAddress)
		})

		AfterEach(func() {
			bosh_helpers.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
		})
	})
})
