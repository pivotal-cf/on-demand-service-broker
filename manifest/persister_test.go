package manifest_test

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"

	"github.com/pivotal-cf/on-demand-service-broker/manifest"
)

var _ = Describe("Persister", func() {
	var (
		tmpdir           string
		deploymentName   string
		fileName         string
		manifestContents []byte
		buffer           *Buffer
		persister        manifest.Persister
	)
	BeforeEach(func() {
		var err error
		tmpdir, err = os.MkdirTemp("", "manifest_persister_test")
		Expect(err).NotTo(HaveOccurred())
		buffer = NewBuffer()
		logger := log.New(buffer, "", log.LstdFlags)
		persister = manifest.Persister{
			Prefix: tmpdir,
			Logger: logger,
		}
		deploymentName = "service-instance_guid"
		fileName = "manifest.yml"
		manifestContents = []byte("some stuff")

	})
	// we pick 0750 so the directory is writeable by the vcap user
	// users can ssh and see the contents of the directory without being the root user
	It("creates a directory with appropriate permissions", func() {
		path := filepath.Join(tmpdir, deploymentName)
		persister.PersistManifest(deploymentName, fileName, manifestContents)
		Expect(path).To(BeADirectory())
		fileInfo, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(fileInfo.Mode().Perm()).To(Equal(fs.FileMode(0750)))
	})

	It("creates a file with appropriate permissions", func() {
		path := filepath.Join(tmpdir, deploymentName, fileName)
		persister.PersistManifest(deploymentName, fileName, manifestContents)
		Expect(path).To(BeAnExistingFile())
		fileInfo, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(fileInfo.Mode().Perm()).To(Equal(fs.FileMode(0640)))
	})

	It("writes contents to the file", func() {
		path := filepath.Join(tmpdir, deploymentName, fileName)
		persister.PersistManifest(deploymentName, fileName, manifestContents)
		observedContents, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(observedContents).To(Equal(manifestContents))
	})

	It("logs an error when the subdirectory cannot be created", func() {
		persister.Prefix = tmpdir + "/does/not/exist"
		persister.PersistManifest(deploymentName, fileName, manifestContents)
		dir := filepath.Join(persister.Prefix, deploymentName)
		Expect(buffer).To(Say(`Failed to create directory %s:`, dir))
	})

	It("logs an error when the manifest file cannot be written", func() {
		err := os.MkdirAll(tmpdir+"/"+deploymentName+"/"+fileName, 0750)
		Expect(err).NotTo(HaveOccurred())
		persister.PersistManifest(deploymentName, fileName, manifestContents)
		path := filepath.Join(tmpdir, deploymentName, fileName)
		Expect(buffer).To(Say(`Failed to persist manifest %s:`, path))
	})
})
