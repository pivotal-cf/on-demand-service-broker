package telemetry_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/service/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/telemetry"
	. "github.com/pivotal-cf/on-demand-service-broker/telemetry/fakes_telemetry"
	"github.com/pkg/errors"
)

var _ = Describe("Telemetry", func() {
	var (
		instanceLister   *fakes.FakeInstanceLister
		logBuffer        *gbytes.Buffer
		loggerFactory    *loggerfactory.LoggerFactory
		brokerIdentifier string
		telemetryTime    *FakeTime
	)

	Describe("Telemetry Logger enabled", func() {
		var telemetryLogger broker.TelemetryLogger

		BeforeEach(func() {
			brokerIdentifier = "a-cute-broker"
			logBuffer = gbytes.NewBuffer()
			loggerFactory = loggerfactory.New(logBuffer, brokerIdentifier, loggerfactory.Flags)

			instanceLister = new(fakes.FakeInstanceLister)
			telemetryTime = new(FakeTime)

			telemetryLogger = &telemetry.TelemetryLogger{Logger: loggerFactory.New(), BrokerIdentifier: brokerIdentifier, Time: telemetryTime}

		})

		Describe("LogTotalInstances", func() {
			It("logs telemetry log the total number of instances", func() {
				instanceLister.InstancesReturns([]service.Instance{
					{
						GUID:         "test-guid",
						PlanUniqueID: "plan-id",
					},
				}, nil)

				fakeTime := "2006-01-02 15:04:05"
				telemetryTime.NowReturns(fakeTime)

				telemetryLogger.LogTotalInstances(instanceLister, "broker", "startup")

				Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf(`{"telemetry-time":"%s","telemetry-source":"odb-%s","service-instances":{"total":1},"event":{"item":"broker","operation":"startup"}}`, fakeTime, brokerIdentifier)))
			})

			It("logs error log when it cant get the total number of instances", func() {
				errorMessage := "opsie"
				instanceLister.InstancesReturns([]service.Instance{}, errors.New(errorMessage))

				telemetryLogger.LogTotalInstances(instanceLister, "not-relevant", "not-relevant")

				Eventually(logBuffer).Should(gbytes.Say(`Failed to query list of instances for telemetry`))
				Eventually(logBuffer).Should(gbytes.Say(errorMessage))
				Eventually(logBuffer).ShouldNot(gbytes.Say(`{"telemetry-source":`))
			})
		})
	})

	Describe("NoOp Telemetry Logger ", func() {
		var telemetryLogger broker.TelemetryLogger

		BeforeEach(func() {
			logBuffer = gbytes.NewBuffer()
			loggerFactory = loggerfactory.New(logBuffer, "telemetry-test", loggerfactory.Flags)

			instanceLister = new(fakes.FakeInstanceLister)
		})

		Describe("LogTotalInstances", func() {
			It("does not log telemetry", func() {
				telemetryLogger = telemetry.Build(false, brokerIdentifier, loggerFactory.New())

				telemetryLogger.LogTotalInstances(instanceLister, "not-relevant", "not-relevant")

				Eventually(logBuffer).ShouldNot(gbytes.Say(`{"telemetry-source":`))
				Expect(instanceLister.InstancesCallCount()).To(BeZero(), "Instance listener was not called")
			})
		})
	})
})
