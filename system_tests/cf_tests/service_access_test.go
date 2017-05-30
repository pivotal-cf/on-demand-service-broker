package cf_tests

import (
	"fmt"
	"os"

	"log"

	cf_helper "github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/cf"

	"github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var _ = Describe("cf.Client.DisableServiceAccess", func() {
	var conf testConfig

	BeforeEach(func() {
		conf = testConfigFromEnv()
		Eventually(cf_helper.Cf("create-service-broker", conf.brokerName, conf.brokerUsername, conf.brokerPassword, conf.brokerURL), cf_helpers.CfTimeout).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		Eventually(cf_helper.Cf("delete-service-broker", conf.brokerName, "-f"), cf_helpers.CfTimeout).Should(gexec.Exit(0))
	})

	It("disables service access", func() {
		Eventually(cf_helper.Cf("enable-service-access", conf.serviceOffering), cf_helpers.CfTimeout).Should(gexec.Exit(0))

		client := getClient(conf.cFUAAURL, conf.cfAPIURL, conf.cfUser, conf.cfPassword)
		err := client.DisableServiceAccess(conf.serviceGUID, testLogger())

		Expect(err).NotTo(HaveOccurred())

		session := cf_helper.Cf("m")
		Eventually(session, cf_helpers.CfTimeout).Should(gexec.Exit(0))
		Expect(session.Out).NotTo(gbytes.Say(conf.serviceOffering))
	})
})

type testConfig struct {
	cfAPIURL        string
	cFUAAURL        string
	cfUser          string
	cfPassword      string
	serviceGUID     string
	serviceOffering string
	brokerName      string
	brokerURL       string
	brokerUsername  string
	brokerPassword  string
}

func testLogger() *log.Logger {
	return log.New(GinkgoWriter, "", log.LstdFlags)
}

func getClient(uaaURL, apiURL, user, password string) cf.Client {

	auth, err := authorizationheader.NewUserTokenAuthHeaderBuilder(
		uaaURL,
		"cf",
		"",
		user,
		password,
		true,
		[]byte{},
	)

	Expect(err).NotTo(HaveOccurred())
	client, err := cf.New(
		apiURL,
		auth,
		[]byte{},
		true,
	)
	Expect(err).ToNot(HaveOccurred())

	return client
}

func testConfigFromEnv() testConfig {
	return testConfig{
		cfAPIURL:        envMustHave("CF_URL"),
		cFUAAURL:        envMustHave("CF_UAA_URL"),
		cfUser:          envMustHave("CF_USERNAME"),
		cfPassword:      envMustHave("CF_PASSWORD"),
		serviceGUID:     envMustHave("SERVICE_GUID"),
		serviceOffering: envMustHave("SERVICE_NAME"),
		brokerName:      envMustHave("BROKER_NAME"),
		brokerURL:       envMustHave("BROKER_URL"),
		brokerUsername:  envMustHave("BROKER_USERNAME"),
		brokerPassword:  envMustHave("BROKER_PASSWORD"),
	}
}

func envMustHave(envVar string) string {
	envVal := os.Getenv(envVar)
	Expect(envVal).ToNot(BeEmpty(), fmt.Sprintf("must set %s", envVar))
	return envVal
}
