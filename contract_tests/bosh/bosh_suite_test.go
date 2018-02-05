package bosh_test

import (
	"crypto/x509"
	"fmt"
	"log"
	"os"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"

	"testing"
)

func TestBosh(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bosh Suite")
}

func envMustHave(key string) string {
	value := os.Getenv(key)
	Expect(value).ToNot(BeEmpty(), fmt.Sprintf("must set %s", key))
	return value
}

var (
	c              *boshdirector.Client
	logger         *log.Logger
	stdout         *gbytes.Buffer
	boshAuthConfig config.Authentication
)

func NewBOSHClient() *boshdirector.Client {
	certPEM := []byte(envMustHave("BOSH_CA_CERT"))
	var err error

	stdout = gbytes.NewBuffer()

	factory := boshdir.NewFactory(boshlog.NewLogger(boshlog.LevelError))
	uaaFactory := boshuaa.NewFactory(boshlog.NewLogger(boshlog.LevelError))

	certPool, err := x509.SystemCertPool()
	Expect(err).NotTo(HaveOccurred())

	boshAuthConfig = config.Authentication{
		UAA: config.UAAAuthentication{
			URL: envMustHave("UAA_URL"),
			ClientCredentials: config.ClientCredentials{
				ID:     envMustHave("BOSH_USERNAME"),
				Secret: envMustHave("BOSH_PASSWORD"),
			},
		},
	}

	loggerFactory := loggerfactory.New(stdout, "", loggerfactory.Flags)
	logger = loggerFactory.New()

	c, err = boshdirector.New(
		envMustHave("BOSH_URL"),
		certPEM,
		certPool,
		factory,
		uaaFactory,
		boshAuthConfig,
		logger,
	)
	Expect(err).NotTo(HaveOccurred())
	c.PollingInterval = 0
	return c
}

func NewBOSHClientWithBadCredentials() *boshdirector.Client {
	certPEM := []byte(envMustHave("BOSH_CA_CERT"))
	var err error

	stdout = gbytes.NewBuffer()

	factory := boshdir.NewFactory(boshlog.NewLogger(boshlog.LevelError))
	uaaFactory := boshuaa.NewFactory(boshlog.NewLogger(boshlog.LevelError))

	certPool, err := x509.SystemCertPool()
	Expect(err).NotTo(HaveOccurred())

	boshAuthConfig = config.Authentication{
		UAA: config.UAAAuthentication{
			URL: envMustHave("UAA_URL"),
			ClientCredentials: config.ClientCredentials{
				ID:     "foo",
				Secret: "bar",
			},
		},
	}

	loggerFactory := loggerfactory.New(stdout, "", loggerfactory.Flags)
	logger = loggerFactory.New()

	c, err = boshdirector.New(
		envMustHave("BOSH_URL"),
		certPEM,
		certPool,
		factory,
		uaaFactory,
		boshAuthConfig,
		logger,
	)
	Expect(err).NotTo(HaveOccurred())
	c.PollingInterval = 0
	return c
}
