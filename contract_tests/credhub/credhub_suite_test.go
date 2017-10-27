package credhub_tests

import (
	"fmt"
	"os"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"

	"testing"
)

var dev_env string

func TestContractTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Credhub Contract Tests Suite")
}

var _ = BeforeSuite(func() {
	dev_env = os.Getenv("DEV_ENV")
	ensureCredhubIsClean()
})

var _ = AfterSuite(func() {
	ensureCredhubIsClean()
})

func testKeyPrefix() string {
	return fmt.Sprintf("/test-%s", dev_env)
}

func makeKeyPath(name string) string {
	return fmt.Sprintf("%s/%s", testKeyPrefix(), name)
}

func ensureCredhubIsClean() {
	clientSecret := os.Getenv("TEST_CREDHUB_CLIENT_SECRET")
	if clientSecret == "" {
		panic("Expected TEST_CREDHUB_CLIENT_SECRET to be set")
	}

	credhubClient, err := credhub.New(
		"https://credhub.service.cf.internal:8844",
		credhub.SkipTLSValidation(true),
		credhub.Auth(auth.UaaClientCredentials("credhub_cli", clientSecret)),
	)
	Expect(err).NotTo(HaveOccurred())

	oauth, ok := credhubClient.Auth.(*auth.OAuthStrategy)
	Expect(ok).To(BeTrue(), "Could not cast Auth to OAuthStrategy")
	oauth.Login()

	testKeys, err := credhubClient.FindByPath(testKeyPrefix())
	Expect(err).NotTo(HaveOccurred())
	for _, key := range testKeys.Credentials {
		credhubClient.Delete(key.Name)
	}
}

func credhubCorrectAuth() credhubbroker.CredentialStore {
	clientSecret := os.Getenv("TEST_CREDHUB_CLIENT_SECRET")
	if clientSecret == "" {
		panic("Expected TEST_CREDHUB_CLIENT_SECRET to be set")
	}
	credentialStore, err := credhubbroker.NewCredHubStore(
		"https://credhub.service.cf.internal:8844",
		credhub.SkipTLSValidation(true),
		credhub.Auth(auth.UaaClientCredentials("credhub_cli", clientSecret)),
	)
	if err != nil {
		panic(fmt.Sprintf("Unexpected error: %s\n", err.Error()))
	}
	return credentialStore
}

func credhubIncorrectAuth() credhubbroker.CredentialStore {
	credentialStore, err := credhubbroker.NewCredHubStore(
		"https://credhub.service.cf.internal:8844",
		credhub.SkipTLSValidation(true),
		credhub.Auth(auth.UaaClientCredentials("credhub_cli", "reallybadsecret")),
	)
	if err != nil {
		panic(fmt.Sprintf("Unexpected error: %s\n", err.Error()))
	}
	return credentialStore
}

func credhubNoUAAConfig() credhubbroker.CredentialStore {
	credentialStore, err := credhubbroker.NewCredHubStore(
		"https://credhub.service.cf.internal:8844",
		credhub.SkipTLSValidation(true),
	)
	if err != nil {
		panic(fmt.Sprintf("Unexpected error: %s\n", err.Error()))
	}
	return credentialStore
}
