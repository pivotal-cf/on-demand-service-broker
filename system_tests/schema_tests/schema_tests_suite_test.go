package schema_tests

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var (
	serviceOffering string
	brokerName      string
	brokerUsername  string
	brokerPassword  string
	brokerURL       string
)

var _ = BeforeSuite(func() {
	serviceOffering = envMustHave("SERVICE_NAME")
	brokerName = envMustHave("BROKER_NAME")

	brokerUsername = envMustHave("BROKER_USERNAME")
	brokerPassword = envMustHave("BROKER_PASSWORD")
	brokerURL = envMustHave("BROKER_URL")

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
