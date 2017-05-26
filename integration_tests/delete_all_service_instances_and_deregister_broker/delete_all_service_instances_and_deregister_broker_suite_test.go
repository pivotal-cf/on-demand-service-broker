package delete_all_service_instances_and_deregister_broker_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"io/ioutil"
	"os"
	"testing"
)

var (
	binaryPath, tempDir string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	binary, err := gexec.Build("github.com/pivotal-cf/on-demand-service-broker/cmd/delete-all-service-instances-and-deregister-broker")
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

func TestPurgeInstancesAndDeregister(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PurgeInstancesAndDeregister Suite")
}
