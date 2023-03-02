package telemetry_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

var _ = Describe("Telemetry", func() {
	It("logs telemetry when telemetry enabled", func() {

		brokerDeployment := bosh_helpers.DeployBroker(
			"-"+uuid.New()[:8]+"-telemetry-contract-tests",
			bosh_helpers.BrokerDeploymentOptions{},
			service_helpers.Redis,
			[]string{
				"basic_service_catalog.yml",
				"add_telemetry.yml",
			},
			"--var", "telemetry_env_type="+"contract-test-env",
			"--var", "foundation_id="+"contract-test-foundation",
		)

		logs := bosh_helpers.GetTelemetryLogs(brokerDeployment.DeploymentName)

		Expect(logs).To(SatisfyAll(
			ContainSubstring(`"telemetry-env-type":"contract-test-env"`),
			ContainSubstring(`"telemetry-foundation-id":"contract-test-foundation"`),
			ContainSubstring(`"telemetry-source":"on-demand-broker"`),
			ContainSubstring(`"event":{"item":"broker","operation":"startup"}`),
			ContainSubstring(fmt.Sprintf(`"service-offering":{"name":"%s"}`, brokerDeployment.ServiceName)),
		))

		bosh_helpers.DeleteDeployment(brokerDeployment.DeploymentName)
	})
})
