package manifest_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/manifest"
)

var _ = Describe("DisabledPersister", func() {
	var (
		tmpdir           string
		deploymentName   string
		fileName         string
		manifestContents []byte
		persister        manifest.DisabledPersister
	)

	BeforeEach(func() {
		var err error
		tmpdir, err = os.MkdirTemp("", "manifest_persister_test")
		Expect(err).NotTo(HaveOccurred())
		persister = manifest.DisabledPersister{}
		deploymentName = "service-instance_guid"
		fileName = "manifest.yml"
		manifestContents = []byte("some stuff")
	})

	It("does not create a directory or any manifest files", func() {
		path := filepath.Join(tmpdir, deploymentName)
		persister.PersistManifest(deploymentName, fileName, manifestContents)

		Expect(path).ToNot(BeADirectory())

		_, err := os.Stat(path)
		Expect(err).To(HaveOccurred())
		Expect(os.IsNotExist(err)).To(BeTrue())
	})
})
