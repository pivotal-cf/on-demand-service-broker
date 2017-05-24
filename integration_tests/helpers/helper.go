package helpers

import (
	"os/exec"

	"github.com/onsi/gomega/gexec"

	"io/ioutil"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func StartBinaryWithParams(binaryPath string, params []string) *gexec.Session {
	cmd := exec.Command(binaryPath, params...)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session
}

func WriteConfig(config []byte, dir string) string {
	configFilePath := filepath.Join(dir, "config.yml")
	Expect(ioutil.WriteFile(configFilePath, config, 0644)).To(Succeed())
	return configFilePath
}
