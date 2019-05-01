package cf_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

var _ = Describe("CF client", func() {
	var (
		brokerDeployment bosh_helpers.BrokerInfo
		subject          *cf.Client
	)

	BeforeEach(func() {
		brokerDeployment = bosh_helpers.DeployBroker(
			uuid.New()[:8]+"-cf-contract-tests",
			bosh_helpers.BrokerDeploymentOptions{
				ServiceMetrics: false,
				BrokerTLS:      false,
			},
			service_helpers.Redis,
			[]string{"basic_service_catalog.yml"},
		)

		subject = NewCFClient(true)
	})

	AfterEach(func() {
		bosh_helpers.DeleteDeployment(brokerDeployment.DeploymentName)
	})

	Describe("CreateServiceBroker", func() {
		var brokerName string

		BeforeEach(func() {
			brokerName = "contract-" + brokerDeployment.TestSuffix
		})

		AfterEach(func() {
			session := cf_helpers.Cf("delete-service-broker", "-f", brokerName)
			Eventually(session).Should(gexec.Exit(0))
		})

		It("creates a service broker", func() {
			err := subject.CreateServiceBroker(
				brokerName,
				brokerDeployment.BrokerUsername,
				brokerDeployment.BrokerPassword,
				"http://"+brokerDeployment.URI,
			)
			Expect(err).NotTo(HaveOccurred())

			session := cf_helpers.Cf("service-brokers")
			Eventually(session).Should(gexec.Exit(0))

			Expect(session).To(gbytes.Say(brokerDeployment.URI))
		})
	})

	Describe("ServiceBrokers", func() {
		var brokerName string

		BeforeEach(func() {
			brokerName = "contract-" + brokerDeployment.TestSuffix
			session := cf_helpers.Cf("create-service-broker",
				brokerName,
				brokerDeployment.BrokerUsername,
				brokerDeployment.BrokerPassword,
				"http://"+brokerDeployment.URI,
			)
			Eventually(session).Should(gexec.Exit(0))
		})

		AfterEach(func() {
			session := cf_helpers.Cf("delete-service-broker", "-f", brokerName)
			Eventually(session).Should(gexec.Exit())
		})

		It("returns a list of service brokers", func() {
			serviceBrokers, err := subject.ServiceBrokers()
			Expect(err).NotTo(HaveOccurred())

			found := false
			for _, serviceBroker := range serviceBrokers {
				if serviceBroker.Name == brokerName {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "List of brokers did not include the created broker")
		})
	})
})
