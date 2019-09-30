package cf_test

import (
	"log"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

func TestCF(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CF Suite")
}

const (
	planName            = "redis-small"
	serviceInstanceName = "our-cool-instance"
)

var (
	brokerDeployment                            bosh_helpers.BrokerInfo
	brokerName, brokerGUID, serviceInstanceGUID string
)

var _ = BeforeSuite(func() {
	brokerDeployment = bosh_helpers.DeployAndRegisterBroker(
		uuid.New()[:8]+"-cf-contract-tests",
		bosh_helpers.BrokerDeploymentOptions{
			ServiceMetrics: false,
			BrokerTLS:      false,
		},
		service_helpers.Redis,
		[]string{"basic_service_catalog.yml"},
	)

	brokerName = "contract-" + brokerDeployment.TestSuffix

	cf_helpers.CreateService(brokerDeployment.ServiceName, planName, serviceInstanceName, "")
	serviceInstanceGUID = cf_helpers.GetServiceInstanceGUID(serviceInstanceName)
})

var _ = AfterSuite(func() {
	cf_helpers.DeleteService(serviceInstanceName)

	session := cf_helpers.Cf("delete-service-broker", "-f", brokerName)
	Expect(session).To(gexec.Exit(0))

	bosh_helpers.DeleteDeployment(brokerDeployment.DeploymentName)
})

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
