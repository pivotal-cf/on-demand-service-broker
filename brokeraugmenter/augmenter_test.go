package brokeraugmenter_test

import (
	"errors"

	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/brokeraugmenter"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	fakecredentialstorefactory "github.com/pivotal-cf/on-demand-service-broker/credhubbroker/fakes"
)

var _ = Describe("Broker CredHub Augmenter", func() {
	var (
		factory *fakecredentialstorefactory.FakeCredentialStoreFactory
	)

	BeforeEach(func() {
		factory = new(fakecredentialstorefactory.FakeCredentialStoreFactory)
	})

	It("returns a base broker when CredHub is not configured", func() {
		var conf config.Config

		baseBroker := &broker.Broker{}
		loggerFactory := loggerfactory.New(GinkgoWriter, "broker-augmenter", loggerfactory.Flags)

		Expect(brokeraugmenter.New(conf, baseBroker, factory, loggerFactory)).To(Equal(baseBroker))
	})

	It("returns a credhub broker when Credhub is configured", func() {
		var conf config.Config
		conf.CredHub = config.CredHub{
			APIURL:       "https://a.credhub.url.example.com",
			ClientID:     "test-id",
			ClientSecret: "test-secret",
		}

		loggerFactory := loggerfactory.New(GinkgoWriter, "broker-augmenter", loggerfactory.Flags)
		broker, err := brokeraugmenter.New(conf, &broker.Broker{}, factory, loggerFactory)
		Expect(err).NotTo(HaveOccurred())

		Expect(broker).To(BeAssignableToTypeOf(&credhubbroker.CredHubBroker{}))
	})

	It("retries if DNS of credhub cannot be found", func() {
		var conf config.Config
		conf.CredHub = config.CredHub{
			APIURL:       "https://a.credhub.url.example.com",
			ClientID:     "test-id",
			ClientSecret: "test-secret",
		}

		baseBroker := &broker.Broker{}
		loggerFactory := loggerfactory.New(GinkgoWriter, "broker-augmenter", loggerfactory.Flags)

		factory.NewReturns(nil, errors.New("dial blah blah blah: no such host"))
		go func() {
			brokeraugmenter.New(conf, baseBroker, factory, loggerFactory)
		}()
		Eventually(factory.NewCallCount).Should(BeNumerically(">", 1))
	})

	It("returns an error if cannot create a credential store", func() {
		var conf config.Config
		conf.CredHub = config.CredHub{
			APIURL:       "https://a.credhub.url.example.com",
			ClientID:     "test-id",
			ClientSecret: "test-secret",
		}

		baseBroker := &broker.Broker{}
		loggerFactory := loggerfactory.New(GinkgoWriter, "broker-augmenter", loggerfactory.Flags)

		factory.NewReturns(nil, errors.New("oops"))
		_, err := brokeraugmenter.New(conf, baseBroker, factory, loggerFactory)

		Expect(err).To(HaveOccurred())
	})

})
