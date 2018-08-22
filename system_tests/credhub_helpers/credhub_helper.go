package credhub_helpers

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func RelogInToCredhub(client, secret string) {
	command := exec.Command("credhub", "login", "--client-name", client, "--client-secret", secret)
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(session, time.Second*6).Should(gexec.Exit(0))
}

func VerifyCredhubKeysExist(serviceOffering, guid string) {
	creds := VerifyCredhubKeysForInstance(serviceOffering, guid)

	Expect(creds).To(HaveLen(1), "expected to have 1 Credhub key for instance")
	Expect(creds[0]["name"]).To(Equal(fmt.Sprintf("/odb/%s/service-instance_%s/odb_managed_secret", serviceOffering, guid)))
}

func VerifyCredhubKeysEmpty(serviceOffering, guid string) {
	creds := VerifyCredhubKeysForInstance(serviceOffering, guid)

	Expect(creds).To(BeEmpty(), "expected to have no Credhub keys for instance")
}

func VerifyCredhubKeysForInstance(serviceOffering, guid string) []map[string]string {
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

func GetCredhubValueFor(serviceOffering, serviceInstanceGUID, secretName string) map[string]string {
	credhubRefs := VerifyCredhubKeysForInstance(serviceOffering, serviceInstanceGUID)
	for _, ref := range credhubRefs {
		if strings.HasSuffix(ref["name"], secretName) {
			return getValue(ref["name"])
		}
	}
	return nil
}

func getValue(path string) map[string]string {
	command := exec.Command("credhub", "get", "-n", path, "-j")
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(session, time.Second*6).Should(gexec.Exit(0))

	var getResults map[string]string
	err = json.Unmarshal(session.Buffer().Contents(), &getResults)
	Expect(err).NotTo(HaveOccurred())

	return getResults
}
