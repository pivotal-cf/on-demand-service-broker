package feature_flags_test

import (
	"fmt"

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
				"register-broker", "upgrade-all-service-instances",
				"delete-all-service-instances", "orphan-deployments",
				"deregister-broker",
			}

			for _, errand := range errands {
				By("running " + errand)
				if errand == "orphan-deployments" {
					output := boshClient.RunErrandWithoutCheckingSuccess(brokerBoshDeploymentName, errand, []string{}, "")
					Expect(output.ExitCode).To(
						Or(BeZero(), Equal(orphanDeploymentsDetectedExitCode)),
						fmt.Sprintf("STDOUT:\n%s\n----\nSTDERR:\n%s\n\n", output.StdOut, output.StdErr),
					)
				} else {
					boshClient.RunErrand(brokerBoshDeploymentName, errand, []string{}, "")
				}
			}
		})
	})

	When("expose_operational_errors is true", func() {
		It("correctly exposes operational errors", func() {
			boshClient.RunErrand(brokerBoshDeploymentName, "register-broker", []string{}, "")
			serviceName := uuid.New()[:8]

			createServiceSession := cf.Cf("create-service", serviceOffering, "invalid-vm-type", serviceName)
			Eventually(createServiceSession, cf.CfTimeout).Should(
				gexec.Exit(0),
			)
			cf.AwaitServiceCreationFailure(serviceName)
			s := cf.Cf("service", serviceName)
			Eventually(s.Out).Should(gbytes.Say(`Instance group 'redis-server' references an unknown vm type`))

			cf.DeleteService(serviceName)
			cf.AwaitServiceDeletion(serviceName)
		})
	})
})
