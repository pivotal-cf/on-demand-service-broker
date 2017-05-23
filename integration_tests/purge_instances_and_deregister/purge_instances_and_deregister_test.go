package purge_instances_and_deregister_test

import (
	"io"
	"os/exec"
	"time"

	yaml "gopkg.in/yaml.v2"

	"io/ioutil"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/deleter"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
)

var _ = Describe("purge instances and deregister tool", func() {
	const (
		serviceOfferingName = "service-id"
		serviceOfferingGUID = "some-cc-service-offering-guid"

		planID   = "plan-id"
		planGUID = "some-cc-plan-guid"

		cfAccessToken      = "cf-oauth-token"
		cfUaaClientID      = "cf-uaa-client-id"
		cfUaaClientSecret  = "cf-uaa-client-secret"
		instanceGUID       = "some-instance-guid"
		boundAppGUID       = "some-bound-app-guid"
		serviceBindingGUID = "some-binding-guid"
		serviceKeyGUID     = "some-key-guid"

		serviceBrokerGUID = "some-service-broker-guid"
		serviceBrokerName = "some-broker-name"
	)

	var (
		cfAPI          *mockhttp.Server
		cfUAA          *mockuaa.ClientCredentialsServer
		purgerSession  *gexec.Session
		logBuffer      *gbytes.Buffer
		configuration  deleter.Config
		configFilePath string
	)

	BeforeEach(func() {
		cfAPI = mockcfapi.New()
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, cfAccessToken)

		configuration = deleter.Config{
			ServiceCatalog: deleter.ServiceCatalog{
				ID: serviceOfferingName,
			},
			DisableSSLCertVerification: true,
			CF: config.CF{
				URL: cfAPI.URL,
				Authentication: config.UAAAuthentication{
					URL: cfUAA.URL,
					ClientCredentials: config.ClientCredentials{
						ID:     cfUaaClientID,
						Secret: cfUaaClientSecret,
					},
				},
			},
			PollingInitialOffset: 0,
			PollingInterval:      0,
		}

		configYAML, err := yaml.Marshal(configuration)
		Expect(err).ToNot(HaveOccurred())

		configFilePath = writePurgeAndDeregisterToolConfig(configYAML)
	})

	AfterEach(func() {
		cfAPI.VerifyMocks()
		cfAPI.Close()
		cfUAA.Close()
	})

	It("deletes the service instance and deregisters the service broker", func() {
		cfAPI.VerifyAndMock(
			mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceOfferingName, serviceOfferingGUID),
			mockcfapi.ListServicePlans(serviceOfferingGUID).RespondsWithServicePlan(planID, planGUID),
			mockcfapi.DisablePlanAccess(planGUID).RespondsCreated(),

			mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceOfferingName, serviceOfferingGUID),
			mockcfapi.ListServicePlans(serviceOfferingGUID).RespondsWithServicePlan(planID, planGUID),
			mockcfapi.ListServiceInstances(planGUID).RespondsWithServiceInstances(instanceGUID),
			mockcfapi.ListServiceBindings(instanceGUID).RespondsWithServiceBinding(serviceBindingGUID, instanceGUID, boundAppGUID),
			mockcfapi.DeleteServiceBinding(boundAppGUID, serviceBindingGUID).RespondsNoContent(),
			mockcfapi.ListServiceKeys(instanceGUID).RespondsWithServiceKey(serviceKeyGUID, instanceGUID),
			mockcfapi.DeleteServiceKey(serviceKeyGUID).RespondsNoContent(),
			mockcfapi.DeleteServiceInstance(instanceGUID).RespondsAcceptedWith(""),
			mockcfapi.GetServiceInstance(instanceGUID).RespondsWithInProgress(mockcfapi.Delete),
			mockcfapi.GetServiceInstance(instanceGUID).RespondsNotFoundWith(""),
			mockcfapi.ListServiceOfferings().RespondsWithServiceOffering(serviceOfferingName, serviceOfferingGUID),
			mockcfapi.ListServicePlans(serviceOfferingGUID).RespondsWithServicePlan(planID, planGUID),
			mockcfapi.ListServiceInstances(planGUID).RespondsWithNoServiceInstances(),

			mockcfapi.ListServiceBrokers().RespondsWithBrokers(serviceBrokerName, serviceBrokerGUID),
			mockcfapi.DeregisterBroker(serviceBrokerGUID).RespondsNoContent(),
		)

		params := []string{"-configFilePath", configFilePath, "-brokerName", serviceBrokerName}
		purgerSession, logBuffer = startPurgeAndDeregisterTool(params)
		Eventually(purgerSession, 10*time.Second).Should(gexec.Exit(0))

		Expect(logBuffer).To(gbytes.Say("FINISHED PURGE INSTANCES AND DEREGISTER BROKER"))
	})

	It("fails when the purger fails", func() {
		cfAPI.VerifyAndMock(
			mockcfapi.ListServiceOfferings().RespondsInternalServerErrorWith("failed"),
		)

		params := []string{"-configFilePath", configFilePath, "-brokerName", serviceBrokerName}
		purgerSession, logBuffer = startPurgeAndDeregisterTool(params)
		Eventually(purgerSession, 10*time.Second).Should(gexec.Exit(1))
		Eventually(logBuffer).Should(gbytes.Say("Purger Failed:"))

	})

	It("fails when broker name is not provided", func() {
		params := []string{"-configFilePath", configFilePath}
		purgerSession, logBuffer = startPurgeAndDeregisterTool(params)

		Eventually(purgerSession).Should(gexec.Exit(1))
		Eventually(logBuffer).Should(gbytes.Say("Missing argument -brokerName"))
	})

	It("fails when config file path is not provided", func() {
		params := []string{"-brokerName", serviceBrokerName}
		purgerSession, logBuffer = startPurgeAndDeregisterTool(params)

		Eventually(purgerSession).Should(gexec.Exit(1))
		Eventually(logBuffer).Should(gbytes.Say("Missing argument -configFilePath"))
	})

	It("fails when configFilePath cannot be read", func() {
		params := []string{"-configFilePath", "/tmp/foo/bar", "-brokerName", serviceBrokerName}
		purgerSession, logBuffer = startPurgeAndDeregisterTool(params)

		Eventually(purgerSession).Should(gexec.Exit(1))
		Eventually(logBuffer).Should(gbytes.Say("Error reading config file:"))
	})

	It("fails when the config is not valid yaml", func() {
		configFilePath := writePurgeAndDeregisterToolConfig([]byte("not valid yaml"))
		params := []string{"-configFilePath", configFilePath, "-brokerName", serviceBrokerName}
		purgerSession, logBuffer = startPurgeAndDeregisterTool(params)

		Eventually(purgerSession).Should(gexec.Exit(1))
		Eventually(logBuffer).Should(gbytes.Say("Invalid config file:"))
	})

})

// TODO: dedup with delete all
func startPurgeAndDeregisterTool(params []string) (*gexec.Session, *gbytes.Buffer) {
	cmd := exec.Command(binaryPath, params...)
	logBuffer := gbytes.NewBuffer()

	session, err := gexec.Start(cmd, io.MultiWriter(GinkgoWriter, logBuffer), GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session, logBuffer
}

// TODO: dedup with delete all
func writePurgeAndDeregisterToolConfig(config []byte) string {
	configFilePath := filepath.Join(tempDir, "purge_and_deregister_test_config.yml")
	Expect(ioutil.WriteFile(configFilePath, config, 0644)).To(Succeed())
	return configFilePath
}
