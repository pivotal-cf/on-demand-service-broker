package parallel_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/upgrade_all/shared"
)

var (
	config *shared.Config
)

var _ = BeforeSuite(func() {
	config = &shared.Config{}
	config.InitConfig()
	config.RegisterBroker()
})

var _ = AfterSuite(func() {
	config.DeregisterBroker()
})

func TestUpgradeInstancesErrandTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Upgrade Instances with SI API set")
}
