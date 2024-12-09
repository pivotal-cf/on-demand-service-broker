package credhub_helpers

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type CredHubCLI struct {
	ClientName   string
	ClientSecret string
}

func NewCredHubCLI(clientName, clientSecret string) *CredHubCLI {
	return &CredHubCLI{ClientName: clientName, ClientSecret: clientSecret}
}

func (c *CredHubCLI) VerifyCredhubKeysExist(serviceID, guid string) {
	creds := c.VerifyCredhubKeysForInstance(serviceID, guid)

	var found bool
	path := fmt.Sprintf("/odb/%s/service-instance_%s/odb_managed_secret", serviceID, guid)
	for _, cred := range creds {
		if cred["name"] == path {
			found = true
			break
		}
	}
	Expect(found).To(BeTrue(), "credhub key not found")
}

func (c *CredHubCLI) VerifyCredhubKeysEmpty(serviceOffering, guid string) {
	creds := c.VerifyCredhubKeysForInstance(serviceOffering, guid)

	Expect(creds).To(BeEmpty(), "expected to have no Credhub keys for instance")
}

func (c *CredHubCLI) VerifyCredhubKeysForInstance(serviceOffering, guid string) []map[string]string {
	credhubPath := fmt.Sprintf("/odb/%s/service-instance_%s", serviceOffering, guid)

	command := exec.Command("credhub", "find", "-p", credhubPath, "-j")
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(session, time.Second*6).Should(gexec.Exit(0))

	var credhubFindResults map[string][]map[string]string
	err = json.Unmarshal(session.Buffer().Contents(), &credhubFindResults)
	Expect(err).NotTo(HaveOccurred())

	return credhubFindResults["credentials"]
}

func (c *CredHubCLI) GetCredhubValueFor(serviceOffering, serviceInstanceGUID, secretName string) map[string]string {
	credhubRefs := c.VerifyCredhubKeysForInstance(serviceOffering, serviceInstanceGUID)
	for _, ref := range credhubRefs {
		if strings.HasSuffix(ref["name"], secretName) {
			return c.getValue(ref["name"])
		}
	}
	return nil
}

func (c *CredHubCLI) SetCredhubValueFor(path string) {
	command := exec.Command("credhub", "s", "-t", "value", "-n", path, "-v", "secret-value")
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(session, time.Second*6).Should(gexec.Exit(0))
}

func (c *CredHubCLI) DeleteCredhubValueFor(path string) {
	command := exec.Command("credhub", "d", "-n", path)
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(session, time.Second*6).Should(gexec.Exit(0))
}

func (c *CredHubCLI) getValue(path string) map[string]string {
	command := exec.Command("credhub", "get", "-n", path, "-j")
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(session, time.Second*6).Should(gexec.Exit(0))

	var getResults map[string]string
	err = json.Unmarshal(session.Buffer().Contents(), &getResults)
	Expect(err).NotTo(HaveOccurred())

	return getResults
}
