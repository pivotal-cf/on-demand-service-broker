package shared

import (
	"fmt"
	"os"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

type Config struct {
	BrokerName               string
	BrokerUsername           string
	BrokerPassword           string
	BrokerURL                string
	BrokerBoshDeploymentName string
	ServiceOffering          string
	BoshUsername             string
	BoshPassword             string
	BoshURL                  string
	BoshCACert               string
	OriginalBrokerManifest   *bosh.BoshManifest
	ParallelUpgradesEnabled  bool
	CiRootPath               string
	ExampleAppDirName        string
	BoshClient               *bosh_helpers.BoshHelperClient

	CurrentPlan string
	ServiceGUID string
	CfSpace     string
}

func (c *Config) InitConfig() {
	c.BrokerName = envMustHave("BROKER_NAME")
	c.BrokerUsername = envMustHave("BROKER_USERNAME")
	c.BrokerPassword = envMustHave("BROKER_PASSWORD")
	c.BrokerURL = envMustHave("BROKER_URL")
	c.BrokerBoshDeploymentName = envMustHave("BROKER_DEPLOYMENT_NAME")
	c.ServiceOffering = envMustHave("SERVICE_NAME")

	c.BoshURL = envMustHave("BOSH_URL")
	c.BoshUsername = envMustHave("BOSH_USERNAME")
	c.BoshPassword = envMustHave("BOSH_PASSWORD")

	uaaURL := os.Getenv("UAA_URL")
	c.BoshCACert = os.Getenv("BOSH_CA_CERT_FILE")
	c.ParallelUpgradesEnabled = os.Getenv("PARALLEL_UPGRADES_ENABLED") == "true"

	c.CiRootPath = envMustHave("CI_ROOT_PATH")
	c.ExampleAppDirName = envMustHave("EXAMPLE_APP_DIR_NAME")

	c.ServiceGUID = envMustHave("SERVICE_GUID")
	c.CfSpace = envMustHave("CF_SPACE")

	if uaaURL == "" {
		c.BoshClient = bosh_helpers.NewBasicAuth(c.BoshURL, c.BoshUsername, c.BoshPassword, c.BoshCACert, c.BoshCACert == "")
	} else {
		c.BoshClient = bosh_helpers.New(c.BoshURL, uaaURL, c.BoshUsername, c.BoshPassword, c.BoshCACert)
	}

	c.OriginalBrokerManifest = c.BoshClient.GetManifest(c.BrokerBoshDeploymentName)
	Expect(c.OriginalBrokerManifest).NotTo(BeNil(),
		fmt.Sprintf("Deployment '%s' does not exist", c.BrokerBoshDeploymentName))
}

func (c *Config) RegisterBroker() {
	Eventually(cf.Cf("create-service-broker", c.BrokerName, c.BrokerUsername, c.BrokerPassword, c.BrokerURL), cf.CfTimeout).Should(gexec.Exit(0))
	Eventually(cf.Cf("enable-service-access", c.ServiceOffering), cf.CfTimeout).Should(gexec.Exit(0))
}

func (c *Config) DeregisterBroker() {
	Eventually(cf.Cf("delete-service-broker", c.BrokerName, "-f"), cf.CfTimeout).Should(gexec.Exit(0))
}

func envMustHave(key string) string {
	value := os.Getenv(key)
	Expect(value).NotTo(BeEmpty(), fmt.Sprintf("must set %s", key))
	return value
}
