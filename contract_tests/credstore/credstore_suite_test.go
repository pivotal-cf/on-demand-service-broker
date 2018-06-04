package credstore_test

import (
	"os"
	"testing"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBoshcredhub(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Boshcredhub Suite")
}

var (
	credhubServerClient,
	credhubServerClientSecret,
	credhubServerCaCert,
	credhubServerURL string
)

var _ = BeforeSuite(func() {
	credhubServerClient = os.Getenv("CREDHUB_CLIENT")
	Expect(credhubServerClient).NotTo(BeEmpty(), "Expected CREDHUB_CLIENT to be set")
	credhubServerClientSecret = os.Getenv("CREDHUB_CLIENT_SECRET")
	Expect(credhubServerClientSecret).NotTo(BeEmpty(), "Expected CREDHUB_CLIENT_SECRET to be set")
	credhubServerCaCert = os.Getenv("CREDHUB_CA_CERT")
	Expect(credhubServerCaCert).NotTo(BeEmpty(), "Expected CREDHUB_CA_CERT to be set")
	credhubServerURL = os.Getenv("CREDHUB_URL")
	Expect(credhubServerURL).NotTo(BeEmpty(), "Expected CREDHUB_URL to be set")
})

var _ = AfterSuite(func() {
})

func credhubCorrectAuth() *credhub.CredHub {
	credhubClient, err := credhub.New(
		credhubServerURL,
		credhub.Auth(auth.UaaClientCredentials(credhubServerClient, credhubServerClientSecret)),
		credhub.CaCerts(credhubServerCaCert),
	)
	Expect(err).NotTo(HaveOccurred())
	return credhubClient
}
