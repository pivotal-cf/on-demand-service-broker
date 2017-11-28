package factory_test

import (
	. "github.com/pivotal-cf/on-demand-service-broker/factory"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"

	"log"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

var _ = Describe("UpgradeAllInstancesErrandFactory", func() {
	var logger *log.Logger

	BeforeEach(func() {
		loggerFactory := loggerfactory.New(GinkgoWriter, "upgrade-all-service-instances", loggerfactory.Flags)
		logger = loggerFactory.New()
	})

	Describe("Broker Services", func() {
		It("when provided with valid conf returns a expected BrokerServices", func() {
			conf := updateAllInstanceErrandConfig("user", "password", "http://example.org")
			factory := NewUpgradeAllInstancesErrandFactory(conf, logger)

			brokerServices, err := factory.BrokerServices()

			Expect(err).NotTo(HaveOccurred())
			Expect(brokerServices).To(BeAssignableToTypeOf(&services.BrokerServices{}))
		})

		table.DescribeTable(
			"when provided with config missing",
			func(user, password, url string) {
				conf := updateAllInstanceErrandConfig(user, password, url)
				factory := NewUpgradeAllInstancesErrandFactory(conf, logger)

				_, err := factory.BrokerServices()

				Expect(err).To(MatchError(Equal("the brokerUsername, brokerPassword and brokerUrl are required to function")))
			},
			table.Entry("broker username", "", "password", "http://example.org"),
			table.Entry("broker password", "user", "", "http://example.org"),
			table.Entry("broker url", "user", "password", ""),
			table.Entry("all broker values", "", "", ""),
		)
	})

	Describe("Service Instance Lister", func() {
		It("when provided with valid conf returns an expected ServiceInstanceLister", func() {
			conf := updateAllInstanceErrandConfig("user", "password", "http://example.org")
			factory := NewUpgradeAllInstancesErrandFactory(conf, logger)

			serviceInstanceLister, err := factory.ServiceInstanceLister()

			Expect(err).NotTo(HaveOccurred())
			Expect(serviceInstanceLister).To(BeAssignableToTypeOf(&service.ServiceInstanceLister{}))
		})
	})

	Describe("Polling Interval", func() {
		table.DescribeTable(
			"config is invalidly set to",
			func(val int) {
				conf := config.UpgradeAllInstanceErrandConfig{
					PollingInterval: val,
				}
				factory := NewUpgradeAllInstancesErrandFactory(conf, logger)

				_, err := factory.PollingInterval()

				Expect(err).To(MatchError(Equal("the pollingInterval must be greater than zero")))
			},
			table.Entry("zero", 0),
			table.Entry("negative", -1),
		)

		It("when configured returns the value", func() {
			conf := updateAllInstanceErrandConfig("user", "password", "http://example.org")
			conf.PollingInterval = 10
			factory := NewUpgradeAllInstancesErrandFactory(conf, logger)

			pollingInterval, err := factory.PollingInterval()

			Expect(err).NotTo(HaveOccurred())
			Expect(pollingInterval).To(Equal(10))
		})
	})

	Describe("Build", func() {
		It("returns factory objects", func() {
			conf := updateAllInstanceErrandConfig("user", "password", "http://example.org")
			conf.PollingInterval = 10
			factory := NewUpgradeAllInstancesErrandFactory(conf, logger)

			brokerServices, serviceInstanceLister, pollingInterval, err := factory.Build()

			Expect(brokerServices).To(BeAssignableToTypeOf(&services.BrokerServices{}))
			Expect(serviceInstanceLister).To(BeAssignableToTypeOf(&service.ServiceInstanceLister{}))
			Expect(pollingInterval).To(Equal(10))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the polling interval error when configured to zero", func() {
			conf := updateAllInstanceErrandConfig("user", "password", "http://example.org")
			factory := NewUpgradeAllInstancesErrandFactory(conf, logger)

			_, _, pollingInterval, err := factory.Build()

			Expect(pollingInterval).To(Equal(0))
			Expect(err).To(MatchError(Equal("the pollingInterval must be greater than zero")))
		})

		It("returns the broker services error when it fails to build", func() {
			conf := updateAllInstanceErrandConfig("", "password", "http://example.org")
			factory := NewUpgradeAllInstancesErrandFactory(conf, logger)

			brokerServices, _, _, err := factory.Build()

			Expect(brokerServices).To(BeNil())
			Expect(err).To(MatchError(Equal("the brokerUsername, brokerPassword and brokerUrl are required to function")))
		})

		It("prioritizes the broker services error when polling interval is also invalid", func() {
			conf := updateAllInstanceErrandConfig("", "password", "http://example.org")
			conf.PollingInterval = 0
			factory := NewUpgradeAllInstancesErrandFactory(conf, logger)

			_, _, _, err := factory.Build()

			Expect(err).To(MatchError(Equal("the brokerUsername, brokerPassword and brokerUrl are required to function")))
		})
	})
})

func serviceInstancesAPIBlock(user, password, url string) config.ServiceInstancesAPI {
	return config.ServiceInstancesAPI{
		Authentication: config.BOSHAuthentication{
			Basic: config.UserCredentials{
				Username: user,
				Password: password,
			},
		},
		URL: url,
	}
}

func updateAllInstanceErrandConfig(brokerUser, brokerPassword, brokerURL string) config.UpgradeAllInstanceErrandConfig {
	return config.UpgradeAllInstanceErrandConfig{
		BrokerAPI: config.BrokerAPI{
			Authentication: config.BOSHAuthentication{
				Basic: config.UserCredentials{
					Username: brokerUser,
					Password: brokerPassword,
				},
			},
			URL: brokerURL,
		},
		ServiceInstancesAPI: config.ServiceInstancesAPI{
			Authentication: config.BOSHAuthentication{
				Basic: config.UserCredentials{
					Username: brokerUser,
					Password: brokerPassword,
				},
			},
			URL: brokerURL + "/mgmt/service_instances",
		},
	}
}
