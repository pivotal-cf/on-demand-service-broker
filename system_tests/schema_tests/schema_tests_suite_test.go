package schema_tests

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/bosh_helpers"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var (
	serviceOffering          string
	brokerName               string
	brokerBoshDeploymentName string
	brokerUsername           string
	brokerPassword           string
	brokerURL                string
	boshClient               *bosh_helpers.BoshHelperClient
)

var _ = BeforeSuite(func() {
	serviceOffering = envMustHave("SERVICE_NAME")
	brokerName = envMustHave("BROKER_NAME")
	brokerBoshDeploymentName = envMustHave("BROKER_DEPLOYMENT_NAME")

	boshURL := envMustHave("BOSH_URL")
	boshUsername := envMustHave("BOSH_USERNAME")
	boshPassword := envMustHave("BOSH_PASSWORD")
	brokerUsername = envMustHave("BROKER_USERNAME")
	brokerPassword = envMustHave("BROKER_PASSWORD")
	brokerURL = envMustHave("BROKER_URL")
	boshCACert := os.Getenv("BOSH_CA_CERT_FILE")
	disableTLSVerification := boshCACert == ""
	uaaURL := os.Getenv("UAA_URL")

	if uaaURL == "" {
		boshClient = bosh_helpers.NewBasicAuth(boshURL, boshUsername, boshPassword, boshCACert, disableTLSVerification)
	} else {
		boshClient = bosh_helpers.New(boshURL, uaaURL, boshUsername, boshPassword, boshCACert)
	}
	SetDefaultEventuallyTimeout(cf.CfTimeout)
	Eventually(cf.Cf("create-service-broker", brokerName, brokerUsername, brokerPassword, brokerURL), cf.CfTimeout).Should(gexec.Exit(0), fmt.Sprintf("create-service-broker %s -f with timeout %v", brokerName, cf.CfTimeout))
	Eventually(cf.Cf("enable-service-access", serviceOffering), cf.CfTimeout).Should(gexec.Exit(0), fmt.Sprintf("enable-service-access %s -f with timeout %v", brokerName, cf.CfTimeout))
})

var _ = AfterSuite(func() {
	Eventually(cf.Cf("delete-service-broker", brokerName, "-f")).Should(gexec.Exit(0), fmt.Sprintf("delete-service-broker %s -f with timeout %v", brokerName, cf.CfTimeout))
	gexec.CleanupBuildArtifacts()
})

func TestSystemTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Schema Suite")
}

func envMustHave(key string) string {
	value := os.Getenv(key)
	Expect(value).ToNot(BeEmpty(), fmt.Sprintf("must set %s", key))
	return value
}
