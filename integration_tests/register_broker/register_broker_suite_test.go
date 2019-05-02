package register_broker_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestRegisterBroker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RegisterBroker Suite")
}

var binaryPath string

var _ = BeforeSuite(func() {
	var err error
	binaryPath, err = gexec.Build("github.com/pivotal-cf/on-demand-service-broker/cmd/register-broker")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
