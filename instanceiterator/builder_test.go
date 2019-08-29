// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package instanceiterator_test

import (
	"log"
	"time"

	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator"
	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

var _ = Describe("Builder", func() {
	var (
		logger    *log.Logger
		logBuffer *gbytes.Buffer
		logPrefix = "clean-all"
	)

	BeforeEach(func() {
		logBuffer = gbytes.NewBuffer()
		loggerFactory := loggerfactory.New(logBuffer, "process-all-service-instances", loggerfactory.Flags)
		logger = loggerFactory.New()
	})

	Describe("Broker Services", func() {
		It("when provided with valid conf returns a expected BrokerServices", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			builder, err := instanceiterator.NewBuilder(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())

			Expect(builder.BrokerServices).To(BeAssignableToTypeOf(&services.BrokerServices{}))
		})

		DescribeTable(
			"when provided with config missing",
			func(user, password, url string) {
				conf := newErrandConfig(user, password, url)
				_, err := instanceiterator.NewBuilder(conf, logger, logPrefix)

				Expect(err).To(MatchError(Equal("the brokerUsername, brokerPassword and brokerUrl are required to function")))
			},
			Entry("broker username", "", "password", "http://example.org"),
			Entry("broker password", "user", "", "http://example.org"),
			Entry("broker url", "user", "password", ""),
			Entry("all broker values", "", "", ""),
		)
	})

	Describe("Polling Interval", func() {
		DescribeTable(
			"config is invalidly set to",
			func(val int) {
				conf := newErrandConfig("user", "password", "http://example.org")
				conf.PollingInterval = val
				_, err := instanceiterator.NewBuilder(conf, logger, logPrefix)

				Expect(err).To(MatchError(Equal("the pollingInterval must be greater than zero")))
			},
			Entry("zero", 0),
			Entry("negative", -1),
		)

		It("when configured returns the value", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			conf.PollingInterval = 10
			builder, err := instanceiterator.NewBuilder(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())

			Expect(builder.PollingInterval).To(Equal(10 * time.Second))
		})
	})

	Describe("Attempt Interval", func() {
		DescribeTable(
			"config is invalidly set to",
			func(val int) {
				conf := newErrandConfig("user", "password", "http://example.org")
				conf.AttemptInterval = val
				_, err := instanceiterator.NewBuilder(conf, logger, logPrefix)

				Expect(err).To(MatchError(Equal("the attemptInterval must be greater than zero")))
			},
			Entry("zero", 0),
			Entry("negative", -1),
		)

		It("when configured returns the value", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			conf.AttemptInterval = 60
			builder, err := instanceiterator.NewBuilder(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())

			Expect(builder.AttemptInterval).To(Equal(60 * time.Second))
		})
	})

	Describe("Attempt Limit", func() {
		DescribeTable(
			"config is invalidly set to",
			func(val int) {
				conf := newErrandConfig("user", "password", "http://example.org")
				conf.AttemptLimit = val
				_, err := instanceiterator.NewBuilder(conf, logger, logPrefix)

				Expect(err).To(MatchError(Equal("the attempt limit must be greater than zero")))
			},
			Entry("zero", 0),
			Entry("negative", -1),
		)

		It("when configured returns the value", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			conf.AttemptLimit = 42
			builder, err := instanceiterator.NewBuilder(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())
			Expect(builder.AttemptLimit).To(Equal(42))
		})
	})

	Describe("Max In flight", func() {
		DescribeTable(
			"config is invalidly set to",
			func(val int) {
				conf := newErrandConfig("user", "password", "http://example.org")
				conf.MaxInFlight = val
				_, err := instanceiterator.NewBuilder(conf, logger, logPrefix)

				Expect(err).To(MatchError(Equal("the max in flight must be greater than zero")))
			},
			Entry("zero", 0),
			Entry("negative", -1),
		)

		It("when configured returns the value", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			conf.MaxInFlight = 10
			builder, err := instanceiterator.NewBuilder(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())
			Expect(builder.MaxInFlight).To(Equal(10))
		})
	})

	Describe("Canaries", func() {
		DescribeTable(
			"config is invalidly set to",
			func(val int) {
				conf := newErrandConfig("user", "password", "http://example.org")
				conf.Canaries = val
				_, err := instanceiterator.NewBuilder(conf, logger, logPrefix)

				Expect(err).To(MatchError(Equal("the number of canaries cannot be negative")))
			},
			Entry("negative", -1),
		)

		It("when configured returns the value", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			conf.Canaries = 10
			builder, err := instanceiterator.NewBuilder(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())
			Expect(builder.Canaries).To(Equal(10))
		})

		It("can parse canaries selection params", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			conf.CanarySelectionParams = config.CanarySelectionParams{
				"size": "small",
				"test": "true",
			}
			builder, err := instanceiterator.NewBuilder(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())
			Expect(builder.CanarySelectionParams).To(Equal(config.CanarySelectionParams{
				"size": "small",
				"test": "true",
			}))
		})
	})

	Describe("SetUpgradeTriggerer", func() {
		When("CF is not configured", func() {
			It("sets the triggerer to a broker triggerer", func() {
				conf := newErrandConfig("user", "password", "http://example.org")
				builder, err := instanceiterator.NewBuilder(conf, logger, logPrefix)
				Expect(err).NotTo(HaveOccurred())

				err = builder.SetUpgradeTriggerer(nil, false, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(builder.Triggerer).ToNot(BeNil())
				Expect(builder.Triggerer).To(BeAssignableToTypeOf(new(instanceiterator.BOSHTriggerer)))
			})

			It("returns an error when builder not properly initialised", func() {
				builder := new(instanceiterator.Builder)

				err := builder.SetUpgradeTriggerer(nil, false, logger)
				Expect(err).To(HaveOccurred())
			})
		})

		When("CF is configured", func() {
			var (
				fakeCfClient *fakes.FakeCFClient
				conf         config.InstanceIteratorConfig
			)

			BeforeEach(func() {
				fakeCfClient = new(fakes.FakeCFClient)
				conf = newErrandConfig("user", "password", "http://example.org")
			})

			It("sets the triggerer to CF triggerer when CF single instance upgrade is possible", func() {
				fakeCfClient.CheckMinimumOSBAPIVersionReturns(true)

				builder, err := instanceiterator.NewBuilder(conf, logger, logPrefix)
				Expect(err).NotTo(HaveOccurred())

				err = builder.SetUpgradeTriggerer(fakeCfClient, true, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(builder.Triggerer).ToNot(BeNil())

				Expect(builder.Triggerer).To(BeAssignableToTypeOf(new(instanceiterator.CFTriggerer)))

				Expect(fakeCfClient.CheckMinimumOSBAPIVersionCallCount()).To(Equal(1))
				min, _ := fakeCfClient.CheckMinimumOSBAPIVersionArgsForCall(0)
				Expect(min).To(Equal("2.15"))
			})

			When("CF single instance upgrade is not possible", func() {
				It("sets the triggerer to Broker triggerer when OSBAPI version is < 2.15", func() {
					fakeCfClient.CheckMinimumOSBAPIVersionReturns(false)
					builder, err := instanceiterator.NewBuilder(conf, logger, logPrefix)
					Expect(err).NotTo(HaveOccurred())

					err = builder.SetUpgradeTriggerer(fakeCfClient, false, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(builder.Triggerer).ToNot(BeNil())

					Expect(builder.Triggerer).To(BeAssignableToTypeOf(new(instanceiterator.BOSHTriggerer)))
				})

				It("sets the triggerer to Broker triggerer when maintenance_info is not set", func() {
					fakeCfClient.CheckMinimumOSBAPIVersionReturns(true)

					builder, err := instanceiterator.NewBuilder(conf, logger, logPrefix)
					Expect(err).NotTo(HaveOccurred())

					err = builder.SetUpgradeTriggerer(fakeCfClient, false, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(builder.Triggerer).ToNot(BeNil())

					Expect(builder.Triggerer).To(BeAssignableToTypeOf(new(instanceiterator.BOSHTriggerer)))
				})

				It("returns an error when Broker Services is not properly initialised", func() {
					fakeCfClient.CheckMinimumOSBAPIVersionReturns(true)
					builder := new(instanceiterator.Builder)

					err := builder.SetUpgradeTriggerer(fakeCfClient, false, logger)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})

	Describe("SetRecreateTriggerer", func() {
		It("sets a recreate triggerer on a properly initiated builder", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			builder, err := instanceiterator.NewBuilder(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())

			err = builder.SetRecreateTriggerer()
			Expect(err).NotTo(HaveOccurred())
			Expect(builder.Triggerer).ToNot(BeNil())
			Expect(builder.Triggerer).To(BeAssignableToTypeOf(new(instanceiterator.BOSHTriggerer)))
		})

		It("returns an error wh en builder not properly initialised", func() {
			builder := new(instanceiterator.Builder)

			err := builder.SetRecreateTriggerer()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("passing logging prefix into builder", func() {
		It("sets an appropriately configured logger on the builder", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			builder, err := instanceiterator.NewBuilder(conf, logger, "pseudo-")
			Expect(err).NotTo(HaveOccurred())

			builder.Listener.CanariesFinished()
			Expect(logBuffer).To(gbytes.Say(`\[pseudo-\]`))
		})
	})
})

func newErrandConfig(brokerUser, brokerPassword, brokerURL string) config.InstanceIteratorConfig {
	return config.InstanceIteratorConfig{
		BrokerAPI: config.BrokerAPI{
			Authentication: config.Authentication{
				Basic: config.UserCredentials{
					Username: brokerUser,
					Password: brokerPassword,
				},
			},
			URL: brokerURL,
		},
		PollingInterval: 10,
		AttemptInterval: 60,
		AttemptLimit:    5,
		MaxInFlight:     1,
	}
}
