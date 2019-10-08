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

var _ = Describe("Configurator", func() {
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
			configurator, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())

			Expect(configurator.BrokerServices).To(BeAssignableToTypeOf(&services.BrokerServices{}))
		})

		DescribeTable(
			"when provided with config missing",
			func(user, password, url string) {
				conf := newErrandConfig(user, password, url)
				_, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)

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
				_, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)

				Expect(err).To(MatchError(Equal("the pollingInterval must be greater than zero")))
			},
			Entry("zero", 0),
			Entry("negative", -1),
		)

		It("when configured returns the value", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			conf.PollingInterval = 10
			configurator, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())

			Expect(configurator.PollingInterval).To(Equal(10 * time.Second))
		})
	})

	Describe("Attempt Interval", func() {
		DescribeTable(
			"config is invalidly set to",
			func(val int) {
				conf := newErrandConfig("user", "password", "http://example.org")
				conf.AttemptInterval = val
				_, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)

				Expect(err).To(MatchError(Equal("the attemptInterval must be greater than zero")))
			},
			Entry("zero", 0),
			Entry("negative", -1),
		)

		It("when configured returns the value", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			conf.AttemptInterval = 60
			configurator, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())

			Expect(configurator.AttemptInterval).To(Equal(60 * time.Second))
		})
	})

	Describe("Attempt Limit", func() {
		DescribeTable(
			"config is invalidly set to",
			func(val int) {
				conf := newErrandConfig("user", "password", "http://example.org")
				conf.AttemptLimit = val
				_, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)

				Expect(err).To(MatchError(Equal("the attempt limit must be greater than zero")))
			},
			Entry("zero", 0),
			Entry("negative", -1),
		)

		It("when configured returns the value", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			conf.AttemptLimit = 42
			configurator, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())
			Expect(configurator.AttemptLimit).To(Equal(42))
		})
	})

	Describe("Max In flight", func() {
		DescribeTable(
			"config is invalidly set to",
			func(val int) {
				conf := newErrandConfig("user", "password", "http://example.org")
				conf.MaxInFlight = val
				_, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)

				Expect(err).To(MatchError(Equal("the max in flight must be greater than zero")))
			},
			Entry("zero", 0),
			Entry("negative", -1),
		)

		It("when configured returns the value", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			conf.MaxInFlight = 10
			configurator, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())
			Expect(configurator.MaxInFlight).To(Equal(10))
		})
	})

	Describe("Canaries", func() {
		DescribeTable(
			"config is invalidly set to",
			func(val int) {
				conf := newErrandConfig("user", "password", "http://example.org")
				conf.Canaries = val
				_, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)

				Expect(err).To(MatchError(Equal("the number of canaries cannot be negative")))
			},
			Entry("negative", -1),
		)

		It("when configured returns the value", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			conf.Canaries = 10
			configurator, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())
			Expect(configurator.Canaries).To(Equal(10))
		})

		It("can parse canaries selection params", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			conf.CanarySelectionParams = config.CanarySelectionParams{
				"size": "small",
				"test": "true",
			}
			configurator, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())
			Expect(configurator.CanarySelectionParams).To(Equal(config.CanarySelectionParams{
				"size": "small",
				"test": "true",
			}))
		})
	})

	Describe("SetUpgradeTriggererToBOSH", func() {
		It("sets the triggerer to a BOSH triggerer", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			configurator, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())

			configurator.SetUpgradeTriggererToBOSH()
			Expect(configurator.Triggerer).ToNot(BeNil())
			Expect(configurator.Triggerer).To(BeAssignableToTypeOf(new(instanceiterator.BOSHTriggerer)))
		})
	})

	Describe("SetUpgradeTriggererToCF", func() {
		var (
			configurator *instanceiterator.Configurator
			fakeCfClient instanceiterator.CFClient
		)

		BeforeEach(func() {
			fakeCfClient = new(fakes.FakeCFClient)
			conf := newErrandConfig("user", "password", "http://example.org")

			var err error
			configurator, err = instanceiterator.NewConfigurator(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())
		})

		It("sets the triggerer to CF triggerer", func() {
			configurator.SetUpgradeTriggererToCF(fakeCfClient, logger)

			Expect(configurator.Triggerer).ToNot(BeNil())
			Expect(configurator.Triggerer).To(BeAssignableToTypeOf(new(instanceiterator.CFTriggerer)))
		})
	})

	Describe("SetRecreateTriggerer", func() {
		It("sets a recreate triggerer on a properly initiated configurator", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			configurator, err := instanceiterator.NewConfigurator(conf, logger, logPrefix)
			Expect(err).NotTo(HaveOccurred())

			err = configurator.SetRecreateTriggerer()
			Expect(err).NotTo(HaveOccurred())
			Expect(configurator.Triggerer).ToNot(BeNil())
			Expect(configurator.Triggerer).To(BeAssignableToTypeOf(new(instanceiterator.BOSHTriggerer)))
		})

		It("returns an error wh en configurator not properly initialised", func() {
			configurator := new(instanceiterator.Configurator)

			err := configurator.SetRecreateTriggerer()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("passing logging prefix into configurator", func() {
		It("sets an appropriately configured logger on the configurator", func() {
			conf := newErrandConfig("user", "password", "http://example.org")
			configurator, err := instanceiterator.NewConfigurator(conf, logger, "pseudo-")
			Expect(err).NotTo(HaveOccurred())

			configurator.Listener.CanariesFinished()
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
