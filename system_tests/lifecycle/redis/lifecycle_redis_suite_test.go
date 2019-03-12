package lifecycle_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	. "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/env_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

func TestRedisLifecycle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Redis Lifecycle Suite")
}

var (
	brokerInfo        BrokerInfo
	dopplerAddress    string
	deploymentOptions BrokerDeploymentOptions
)

var _ = BeforeSuite(func() {
	brokerInfo = BrokerInfo{}

	err := env_helpers.ValidateEnvVars("DOPPLER_ADDRESS")
	Expect(err).ToNot(HaveOccurred(), "Doppler address must be set")

	dopplerAddress = os.Getenv("DOPPLER_ADDRESS")
	legacyMetrics := os.Getenv("LEGACY_SERVICE_METRICS")
	metricsOpsFile := "service_metrics.yml"
	if legacyMetrics == "true" {
		metricsOpsFile = "service_metrics_with_metron_agent.yml"
	}

	deploymentOptions = bosh_helpers.BrokerDeploymentOptions{
		ServiceMetrics: true,
		BrokerTLS:      true,
	}

	uniqueID := uuid.New()[:6]

	brokerInfo = bosh_helpers.DeployAndRegisterBroker(
		"-redis-lifecycle-"+uniqueID,
		deploymentOptions,
		service_helpers.Redis,
		[]string{"basic_service_catalog.yml", metricsOpsFile},
	)
})

var _ = AfterSuite(func() {
	bosh_helpers.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
})
