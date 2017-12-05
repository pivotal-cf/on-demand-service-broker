package upgrader_test

import (
	"time"

	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	. "github.com/pivotal-cf/on-demand-service-broker/upgrader"

	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
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
			factory := UpgradeAllInstancesErrandFactory{Conf: conf, Logger: logger}

			brokerServices, err := factory.BrokerServices()

			Expect(err).NotTo(HaveOccurred())
			Expect(brokerServices).To(BeAssignableToTypeOf(&services.BrokerServices{}))
		})

		DescribeTable(
			"when provided with config missing",
			func(user, password, url string) {
				conf := updateAllInstanceErrandConfig(user, password, url)
				factory := UpgradeAllInstancesErrandFactory{Conf: conf, Logger: logger}

				_, err := factory.BrokerServices()

				Expect(err).To(MatchError(Equal("the brokerUsername, brokerPassword and brokerUrl are required to function")))
			},
			Entry("broker username", "", "password", "http://example.org"),
			Entry("broker password", "user", "", "http://example.org"),
			Entry("broker url", "user", "password", ""),
			Entry("all broker values", "", "", ""),
		)
	})

	Describe("Service Instance Lister", func() {
		It("when provided with valid conf returns an expected ServiceInstanceLister", func() {
			conf := updateAllInstanceErrandConfig("user", "password", "http://example.org")
			factory := UpgradeAllInstancesErrandFactory{Conf: conf, Logger: logger}

			serviceInstanceLister, err := factory.ServiceInstanceLister()

			Expect(err).NotTo(HaveOccurred())
			Expect(serviceInstanceLister).To(BeAssignableToTypeOf(&service.ServiceInstanceLister{}))
		})
	})

	Describe("Polling Interval", func() {
		DescribeTable(
			"config is invalidly set to",
			func(val int) {
				conf := config.UpgradeAllInstanceErrandConfig{
					PollingInterval: val,
				}
				factory := UpgradeAllInstancesErrandFactory{Conf: conf, Logger: logger}

				_, err := factory.PollingInterval()

				Expect(err).To(MatchError(Equal("the pollingInterval must be greater than zero")))
			},
			Entry("zero", 0),
			Entry("negative", -1),
		)

		It("when configured returns the value", func() {
			conf := updateAllInstanceErrandConfig("user", "password", "http://example.org")
			conf.PollingInterval = 10
			factory := UpgradeAllInstancesErrandFactory{Conf: conf, Logger: logger}

			pollingInterval, err := factory.PollingInterval()

			Expect(err).NotTo(HaveOccurred())
			Expect(pollingInterval).To(Equal(10 * time.Second))
		})
	})

	Describe("Attempt Limit", func() {
		DescribeTable(
			"config is invalidly set to",
			func(val int) {
				conf := config.UpgradeAllInstanceErrandConfig{
					AttemptLimit: val,
				}
				factory := UpgradeAllInstancesErrandFactory{Conf: conf, Logger: logger}

				_, err := factory.AttemptLimit()

				Expect(err).To(MatchError(Equal("the attempt limit must be greater than zero")))
			},
			Entry("zero", 0),
			Entry("negative", -1),
		)

		It("when configured returns the value", func() {
			conf := updateAllInstanceErrandConfig("user", "password", "http://example.org")
			conf.AttemptLimit = 42
			factory := UpgradeAllInstancesErrandFactory{Conf: conf, Logger: logger}

			attemptLimit, err := factory.AttemptLimit()

			Expect(err).NotTo(HaveOccurred())
			Expect(attemptLimit).To(Equal(42))
		})
	})

	// Describe("Build", func() {
	// 	It("returns factory objects", func() {
	// 		conf := updateAllInstanceErrandConfig("user", "password", "http://example.org")
	// 		conf.PollingInterval = 10
	// 		factory := UpgradeAllInstancesErrandFactory{Conf: conf, Logger: logger}

	// 		brokerServices, serviceInstanceLister, pollingInterval, err := factory.Build()

	// 		Expect(brokerServices).To(BeAssignableToTypeOf(&services.BrokerServices{}))
	// 		Expect(serviceInstanceLister).To(BeAssignableToTypeOf(&service.ServiceInstanceLister{}))
	// 		Expect(pollingInterval).To(Equal(10))
	// 		Expect(err).NotTo(HaveOccurred())
	// 	})

	// 	It("returns the polling interval error when configured to zero", func() {
	// 		conf := updateAllInstanceErrandConfig("user", "password", "http://example.org")
	// 		factory := UpgradeAllInstancesErrandFactory{Conf: conf, Logger: logger}

	// 		_, _, pollingInterval, err := factory.Build()

	// 		Expect(pollingInterval).To(Equal(0))
	// 		Expect(err).To(MatchError(Equal("the pollingInterval must be greater than zero")))
	// 	})

	// 	It("returns the broker services error when it fails to build", func() {
	// 		conf := updateAllInstanceErrandConfig("", "password", "http://example.org")
	// 		factory := UpgradeAllInstancesErrandFactory{Conf: conf, Logger: logger}

	// 		brokerServices, _, _, err := factory.Build()

	// 		Expect(brokerServices).To(BeNil())
	// 		Expect(err).To(MatchError(Equal("the brokerUsername, brokerPassword and brokerUrl are required to function")))
	// 	})

	// 	It("prioritizes the broker services error when polling interval is also invalid", func() {
	// 		conf := updateAllInstanceErrandConfig("", "password", "http://example.org")
	// 		conf.PollingInterval = 0
	// 		factory := UpgradeAllInstancesErrandFactory{Conf: conf, Logger: logger}

	// 		_, _, _, err := factory.Build()

	// 		Expect(err).To(MatchError(Equal("the brokerUsername, brokerPassword and brokerUrl are required to function")))
	// 	})
	// })
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
