package deregister_broker_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"io/ioutil"
	"os"
	"testing"

	"github.com/onsi/gomega/gexec"
)

var (
	binaryPath, tempDir string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	binary, err := gexec.Build("github.com/pivotal-cf/on-demand-service-broker/cmd/deregister-broker")
	Expect(err).NotTo(HaveOccurred())

	return []byte(binary)
}, func(rawBinary []byte) {
	binaryPath = string(rawBinary)

	var err error
	tempDir, err = ioutil.TempDir("", fmt.Sprintf("broker-integration-tests-%d", GinkgoParallelNode()))
	Expect(err).ToNot(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {
	Expect(os.RemoveAll(tempDir)).To(Succeed())
}, func() {
	gexec.CleanupBuildArtifacts()
})

func TestDeregisterBroker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DeregisterBroker Suite")
}
