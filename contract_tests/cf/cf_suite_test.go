package cf_test

import (
	"io"
	"log"
	"os"
	"testing"

	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

func TestCF(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CF Suite")
}

func NewCFClient(disableSSL bool) *cf.Client {
	cfConfig := config.CF{
		URL:         os.Getenv("CF_URL"),
		TrustedCert: os.Getenv("CF_CA_CERT"),
		Authentication: config.Authentication{
			Basic: config.UserCredentials{
				Username: os.Getenv("CF_USERNAME"),
				Password: os.Getenv("CF_PASSWORD"),
			},
			UAA: config.UAAAuthentication{
				URL: "https://uaa." + os.Getenv("BROKER_SYSTEM_DOMAIN"),
				ClientCredentials: config.ClientCredentials{
					ID:     os.Getenv("CF_CLIENT_ID"),
					Secret: os.Getenv("CF_CLIENT_SECRET"),
				},
				UserCredentials: config.UserCredentials{
					Username: os.Getenv("CF_USERNAME"),
					Password: os.Getenv("CF_PASSWORD"),
				},
			},
		},
	}
	cfAuthenticator, err := cfConfig.NewAuthHeaderBuilder(disableSSL)
	Expect(err).ToNot(HaveOccurred())

	logBuffer := gbytes.NewBuffer()
	cfClient, err := cf.New(cfConfig.URL, cfAuthenticator, []byte(cfConfig.TrustedCert), disableSSL, log.New(io.MultiWriter(logBuffer, GinkgoWriter), "my-app", log.LstdFlags))
	Expect(err).ToNot(HaveOccurred())

	return &cfClient
}
