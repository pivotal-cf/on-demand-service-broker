package bosh_test

import (
	"fmt"
	"os"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBosh(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bosh Suite")
}

func getUAA() boshuaa.UAA {
	logger := boshlog.NewLogger(boshlog.LevelError)
	factory := boshuaa.NewFactory(logger)

	uaaURL := envMustHave("UAA_URL")
	config, err := boshuaa.NewConfigFromURL(uaaURL)
	Expect(err).NotTo(HaveOccurred())

	config.Client = envMustHave("UAA_CLIENT")
	config.ClientSecret = envMustHave("UAA_SECRET")
	config.CACert = envMustHave("UAA_CA_CERT")

	uaaConf, err := factory.New(config)
	Expect(err).NotTo(HaveOccurred())
	return uaaConf
}

func envMustHave(key string) string {
	value := os.Getenv(key)
	Expect(value).ToNot(BeEmpty(), fmt.Sprintf("must set %s", key))
	return value
}

func getDirector() boshdir.Director {
	logger := boshlog.NewLogger(boshlog.LevelError)
	factory := boshdir.NewFactory(logger)

	directorURL := envMustHave("DIRECTOR_URL")
	config, err := boshdir.NewConfigFromURL(directorURL)
	Expect(err).NotTo(HaveOccurred())

	config.CACert = envMustHave("DIRECTOR_CA_CERT")
	config.TokenFunc = boshuaa.NewClientTokenSession(getUAA()).TokenFunc

	director, err := factory.New(config, boshdir.NewNoopTaskReporter(), boshdir.NewNoopFileReporter())
	Expect(err).NotTo(HaveOccurred())
	return director
}

func getUnauthenticatedDirector() boshdir.Director {
	logger := boshlog.NewLogger(boshlog.LevelError)
	factory := boshdir.NewFactory(logger)

	directorURL := envMustHave("DIRECTOR_URL")
	config, err := boshdir.NewConfigFromURL(directorURL)
	Expect(err).NotTo(HaveOccurred())

	config.CACert = envMustHave("DIRECTOR_CA_CERT")

	director, err := factory.New(config, boshdir.NewNoopTaskReporter(), boshdir.NewNoopFileReporter())
	Expect(err).NotTo(HaveOccurred())
	return director
}
