package cf_test

import (
	"log"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

func TestCF(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CF Suite")
}

func NewCFClient(logger *log.Logger) *cf.Client {
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
	disableSSL := true
	cfAuthenticator, err := cfConfig.NewAuthHeaderBuilder(disableSSL)
	Expect(err).ToNot(HaveOccurred())

	cfClient, err := cf.New(cfConfig.URL, cfAuthenticator, []byte(cfConfig.TrustedCert), disableSSL, logger)
	Expect(err).ToNot(HaveOccurred())

	return &cfClient
}
