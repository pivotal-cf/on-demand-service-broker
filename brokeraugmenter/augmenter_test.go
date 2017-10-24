package brokeraugmenter_test

import (
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/brokeraugmenter"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Broker CredHub Augmenter", func() {
	It("returns a base broker when CredHub is not configured", func() {
		var conf config.Config

		baseBroker := &broker.Broker{}
		loggerFactory := loggerfactory.New(GinkgoWriter, "broker-augmenter", loggerfactory.Flags)

		Expect(brokeraugmenter.New(conf, baseBroker, loggerFactory)).To(Equal(baseBroker))
	})

	It("returns a credhub broker when Credhub is configured", func() {
		var conf config.Config
		conf.CF = config.CF{
			Authentication: config.UAAAuthentication{
				URL: "https://a.cf.uaa.url.example.com",
			},
		}
		conf.CredHub = config.CredHub{
			APIURL:       "https://a.credhub.url.example.com",
			ClientID:     "test-id",
			ClientSecret: "test-secret",
		}

		loggerFactory := loggerfactory.New(GinkgoWriter, "broker-augmenter", loggerfactory.Flags)
		broker, err := brokeraugmenter.New(conf, &broker.Broker{}, loggerFactory)
		Expect(err).NotTo(HaveOccurred())

		Expect(broker).To(BeAssignableToTypeOf(&credhubbroker.CredHubBroker{}))
	})

	It("returns an error when CredHub is configured but the URI is bad", func() {
		var conf config.Config
		conf.CredHub = config.CredHub{
			APIURL:       "💩://a.bad.credhub.url.example.com",
			ClientID:     "test-id",
			ClientSecret: "test-secret",
		}

		loggerFactory := loggerfactory.New(GinkgoWriter, "broker-augmenter", loggerfactory.Flags)
		broker, err := brokeraugmenter.New(conf, &broker.Broker{}, loggerFactory)

		Expect(err).To(HaveOccurred())
		Expect(broker).To(BeNil())
	})
})
