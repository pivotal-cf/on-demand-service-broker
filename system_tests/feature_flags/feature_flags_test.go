package feature_flags_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	bosh "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

var _ = Describe("FeatureFlags", func() {
	var (
		brokerInfo bosh.BrokerInfo
	)

	When("cf disable_ssl_cert_verification is true", func() {
		var brokerRegistered bool

		BeforeEach(func() {
			uniqueID := uuid.New()[:6]
			brokerInfo = bosh.DeployAndRegisterBroker(
				"-feature-flag-"+uniqueID,
				service_helpers.Redis,
				[]string{"update_service_catalog.yml", "disable_cf_ssl_verification.yml"},
			)
		})

		It("can run all the errands successfully", func() {
			By("running the register-broker", func() {
				bosh.RunErrand(brokerInfo.DeploymentName, "register-broker")
				brokerRegistered = true
			})

			By("running upgrade-all-service-instances", func() {
				bosh.RunErrand(brokerInfo.DeploymentName, "upgrade-all-service-instances")
			})

			By("running the orphan-deployments", func() {
				session := bosh.RunErrand(
					brokerInfo.DeploymentName,
					"orphan-deployments",
					Or(gexec.Exit(0), gexec.Exit(1)),
				)
				if session.ExitCode() == 1 {
					Expect(session.Buffer()).To(gbytes.Say("Orphan BOSH deployments detected"))
				}
			})

			By("running delete-all-service-instances", func() {
				bosh.RunErrand(brokerInfo.DeploymentName, "delete-all-service-instances")
			})

			By("running deregister-broker", func() {
				bosh.RunErrand(brokerInfo.DeploymentName, "deregister-broker")
				brokerRegistered = false
			})
		})

		AfterEach(func() {
			if brokerRegistered {
				bosh.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
			} else {
				bosh.DeleteDeployment(brokerInfo.DeploymentName)
			}
		})
	})

	When("Service Instance API is configured and disable_ssl_cert_verification for it is true", func() {
		var appName string

		BeforeEach(func() {
			uniqueID := uuid.New()[:6]

			appName = "si-api-" + uniqueID
			siAPIURL := "https://" + appName + "." + os.Getenv("BROKER_SYSTEM_DOMAIN") + "/service_instances"
			siAPIUsername := "siapi"
			siAPIPassword := "siapipass"

			cf.Cf("push",
				"-p", os.Getenv("SI_API_PATH"),
				"-f", os.Getenv("SI_API_PATH")+"/manifest.yml",
				"--var", "app_name="+appName,
				"--var", "username="+siAPIUsername,
				"--var", "password="+siAPIPassword,
			)

			brokerInfo = bosh.DeployBroker(
				"-feature-flag-"+uniqueID,
				service_helpers.Redis,
				[]string{"update_service_catalog.yml", "add_si_api.yml"},
				"--var", "service_instances_api_url="+siAPIURL,
				"--var", "service_instances_api_username="+siAPIUsername,
				"--var", "service_instances_api_password="+siAPIPassword,
			)

			deleteBOSHCertFromBrokerVM(brokerInfo)
		})

		AfterEach(func() {
			cf.Cf("delete", "-f", appName)
			bosh.DeleteDeployment(brokerInfo.DeploymentName)
		})

		It("runs all errands that target SI API", func() {
			By("running the orphan-deployments", func() {
				session := bosh.RunErrand(
					brokerInfo.DeploymentName,
					"orphan-deployments",
					Or(gexec.Exit(0), gexec.Exit(1)),
				)
				if session.ExitCode() == 1 {
					Expect(session.Buffer()).To(gbytes.Say("Orphan BOSH deployments detected"))
				}
			})

			By("running upgrade-all-service-instances", func() {
				bosh.RunErrand(brokerInfo.DeploymentName, "upgrade-all-service-instances")
			})

			By("running recreate-all-service-instances", func() {
				bosh.RunErrand(brokerInfo.DeploymentName, "recreate-all-service-instances")
			})
		})
	})

	When("expose_operational_errors is true", func() {
		BeforeEach(func() {
			uniqueID := uuid.New()[:6]
			brokerInfo = bosh.DeployAndRegisterBroker(
				"-feature-flag-"+uniqueID,
				service_helpers.Redis,
				[]string{"update_service_catalog.yml", "expose_operational_errors.yml"},
			)
		})

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

		AfterEach(func() {
			bosh.DeregisterAndDeleteBroker(brokerInfo.DeploymentName)
		})
	})
})

func deleteBOSHCertFromBrokerVM(brokerInfo bosh.BrokerInfo) {
	bosh.RunOnVM(
		brokerInfo.DeploymentName,
		"broker",
		"sudo rm -f /etc/ssl/certs/bosh*.pem && sudo rm /etc/ssl/certs/ca-certificates.crt && sudo touch /etc/ssl/certs/ca-certificates.crt",
	)

	bosh.Run(
		brokerInfo.DeploymentName,
		"restart",
		"-n",
		"broker",
	)

	bosh.WaitBrokerToStart(brokerInfo.URI)
}
