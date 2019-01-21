package feature_flags_test

import (
	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

const (
	orphanDeploymentsDetectedExitCode = 10
)

var _ = Describe("FeatureFlags", func() {
	When("disable_ssl_cert_verification is true", func() {
		It("can run all the errands successfully", func() {
			errands := []string{
				"register-broker",
				"upgrade-all-service-instances",
				"delete-all-service-instances",
				"deregister-broker",
			}

			for _, errand := range errands {
				By("running " + errand)
				bosh.RunErrand(brokerInfo.DeploymentName, errand)
			}
		})

		It("can run the orphan-deployment errand successfully", func() {
			session := bosh.RunErrand(
				brokerInfo.DeploymentName,
				"orphan-deployments",
				Or(gexec.Exit(0), gexec.Exit(1)),
			)
			if session.ExitCode() == 1 {
				Expect(session.Buffer()).To(gbytes.Say("Orphan BOSH deployments detected"))
			}
		})
	})

	When("expose_operational_errors is true", func() {
		It("correctly exposes operational errors", func() {
			bosh.RunErrand(brokerInfo.DeploymentName, "register-broker")
			serviceName := uuid.New()[:8]

			createServiceSession := cf.Cf("create-service", brokerInfo.ServiceOffering, "invalid-vm-type", serviceName)
			Eventually(createServiceSession, cf.CfTimeout).Should(gexec.Exit(0))

			cf.AwaitServiceCreationFailure(serviceName)

			s := cf.Cf("service", serviceName)
			Eventually(s.Out).Should(gbytes.Say(`Instance group 'redis-server' references an unknown vm type`))

			cf.DeleteService(serviceName)
			cf.AwaitServiceDeletion(serviceName)
		})
	})
})
