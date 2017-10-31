package credhub_tests

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"

	"log"
	"testing"
	"time"

	"crypto/x509"

	"github.com/craigfurman/herottp"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/totherme/unstructured"
)

const credhubURL = "https://credhub.service.cf.internal:8844"

var (
	dev_env string
	caCerts []string
)

func TestContractTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Credhub Contract Tests Suite")
}

var _ = BeforeSuite(func() {
	dev_env = os.Getenv("DEV_ENV")
	caCerts = extractCAsFromManifest()
	ensureCredhubIsClean()
})

var _ = AfterSuite(func() {
	ensureCredhubIsClean()
})

type TestingAuthHeaderBuilder struct{}

func (a *TestingAuthHeaderBuilder) NewAuthHeaderBuilder(
	boshInfo boshdirector.Info,
	disableSSLCertVerification bool,
) (boshdirector.AuthHeaderBuilder, error) {

	username := os.Getenv("BOSH_USERNAME")
	Expect(username).NotTo(BeEmpty(), "Expected BOSH_USERNAME to be set")
	password := os.Getenv("BOSH_PASSWORD")
	Expect(password).NotTo(BeEmpty(), "Expected BOSH_PASSWORD to be set")

	return authorizationheader.NewClientTokenAuthHeaderBuilder(
		boshInfo.UserAuthentication.Options.URL,
		username,
		password,
		true,
		[]byte{},
	)
}

func getBoshManifest(deploymentName string) ([]byte, error) {
	logger := log.New(GinkgoWriter, "", loggerfactory.Flags)
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return []byte{}, err
	}

	boshURL := os.Getenv("BOSH_URL")
	Expect(boshURL).NotTo(BeEmpty(), "Expected BOSH_URL to be set")

	boshClient, err := boshdirector.New(
		boshURL,
		true,
		[]byte{},
		herottp.New(herottp.Config{
			NoFollowRedirect:                  true,
			DisableTLSCertificateVerification: true,
			Timeout: 30 * time.Second,
		}),
		&TestingAuthHeaderBuilder{},
		certPool,
		logger,
	)

	if err != nil {
		return []byte{}, err
	}

	cfRawManifest, exists, err := boshClient.GetDeployment(deploymentName, logger)
	if err != nil {
		return []byte{}, err
	}

	if !exists {
		return []byte{}, fmt.Errorf("deployment '%s' not found", deploymentName)
	}
	return cfRawManifest, nil
}

func nameIsCredhubMatcher(data unstructured.Data) bool {
	val, err := data.GetByPointer("/name")
	if err != nil {
		return false
	}
	stringVal, err := val.StringValue()
	if err != nil {
		return false
	}
	return stringVal == "credhub"
}

func extractCAsFromManifest() []string {
	cfRawManifest, err := getBoshManifest("cf")
	Expect(err).NotTo(HaveOccurred())

	cfManifest, err := unstructured.ParseYAML(string(cfRawManifest))
	Expect(err).NotTo(HaveOccurred())

	igs, err := cfManifest.GetByPointer("/instance_groups")
	Expect(err).NotTo(HaveOccurred())

	credhubGroup, found := igs.FindElem(nameIsCredhubMatcher)
	Expect(found).To(BeTrue())

	jobs, err := credhubGroup.GetByPointer("/jobs")
	Expect(err).NotTo(HaveOccurred())
	credhubJob, found := jobs.FindElem(nameIsCredhubMatcher)
	Expect(found).To(BeTrue())

	credhubProperties, err := credhubJob.GetByPointer("/properties/credhub")
	Expect(err).NotTo(HaveOccurred())

	uaaCert, err := credhubProperties.GetByPointer("/authentication/uaa/ca_certs/0")
	Expect(err).NotTo(HaveOccurred())
	credhubCert, err := credhubProperties.GetByPointer("/tls/ca")
	Expect(err).NotTo(HaveOccurred())

	return []string{uaaCert.UnsafeStringValue(), credhubCert.UnsafeStringValue()}
}

func testKeyPrefix() string {
	return fmt.Sprintf("/test-%s", dev_env)
}

func makeKeyPath(name string) string {
	return fmt.Sprintf("%s/%s", testKeyPrefix(), name)
}

func ensureCredhubIsClean() {
	clientSecret := os.Getenv("TEST_CREDHUB_CLIENT_SECRET")
	Expect(clientSecret).ToNot(BeEmpty(), "Expected TEST_CREDHUB_CLIENT_SECRET to be set")

	Expect(caCerts).ToNot(BeEmpty())
	credhubClient, err := credhub.New(
		credhubURL,
		credhub.CaCerts(caCerts...),
		credhub.Auth(auth.UaaClientCredentials("credhub_cli", clientSecret)),
	)
	if err != nil {
		Fail(fmt.Sprintf("Could not connect to credhub. %s\n", err.Error()))
	}

	testKeys, err := credhubClient.FindByPath(testKeyPrefix())
	Expect(err).NotTo(HaveOccurred())
	for _, key := range testKeys.Credentials {
		credhubClient.Delete(key.Name)
	}
}

func credhubCorrectAuth() credhubbroker.CredentialStore {
	clientSecret := os.Getenv("TEST_CREDHUB_CLIENT_SECRET")
	Expect(clientSecret).NotTo(BeEmpty(), "Expected TEST_CREDHUB_CLIENT_SECRET to be set")

	credentialStore, err := credhubbroker.NewCredHubStore(
		credhubURL,
		credhub.Auth(auth.UaaClientCredentials("credhub_cli", clientSecret)),
		credhub.CaCerts(caCerts...),
	)
	Expect(err).NotTo(HaveOccurred())
	return credentialStore
}

func requiredCaCerts(paths ...string) (certs []string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return certs, err
	}

	for _, path := range paths {
		caCertPath := filepath.Join(cwd, path)
		caCert, err := ioutil.ReadFile(caCertPath)
		if err != nil {
			return certs, err
		}

		certs = append(certs, string(caCert))
	}
	return
}

func credhubIncorrectAuth() credhubbroker.CredentialStore {
	credentialStore, err := credhubbroker.NewCredHubStore(
		credhubURL,
		credhub.CaCerts(caCerts...),
		credhub.Auth(auth.UaaClientCredentials("credhub_cli", "reallybadsecret")),
	)
	Expect(err).NotTo(HaveOccurred())
	return credentialStore
}

func credhubNoUAAConfig() credhubbroker.CredentialStore {
	credentialStore, err := credhubbroker.NewCredHubStore(
		credhubURL,
		credhub.CaCerts(caCerts...),
	)
	Expect(err).NotTo(HaveOccurred())
	return credentialStore
}
