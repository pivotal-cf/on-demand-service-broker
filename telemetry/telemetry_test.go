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

			telemetryLogger = telemetry.NewTelemetryLogger(loggerFactory.New(), brokerIdentifier, telemetryTime)
		})

		Describe("LogInstances", func() {
			It("logs telemetry about the total number of instances", func() {
				instanceLister.InstancesReturns([]service.Instance{
					{
						GUID:         "test-guid",
						PlanUniqueID: "plan-id",
					},
				}, nil)

				fakeTime := "2006-01-02 15:04:05"
				telemetryTime.NowReturns(fakeTime)

				telemetryLogger.LogInstances(instanceLister, "broker", "startup")

				Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf(`{"telemetry-time":"%s","telemetry-source":"odb-%s","service-instances":{"total":1},"event":{"item":"broker","operation":"startup"}}`, fakeTime, brokerIdentifier)))
			})

			It("logs telemetry about the number of instances per plan", func() {
				instanceLister.InstancesReturns([]service.Instance{
					{
						GUID:         "test-guid-1",
						PlanUniqueID: "plan-unique-id",
					},
					{
						GUID:         "test-guid-2",
						PlanUniqueID: "plan-unique-id",
					},
					{
						GUID:         "test-guid-3",
						PlanUniqueID: "another-plan-unique-id",
					},
				}, nil)

				fakeTime := "fake-timer"
				telemetryTime.NowReturns(fakeTime)

				telemetryLogger.LogInstances(instanceLister, "broker", "startup")

				Expect(logBuffer).To(gbytes.Say(fmt.Sprintf(`{"telemetry-time":"%s","telemetry-source":"odb-%s","service-instances-per-plan":{"plan-id":"plan-unique-id","total":2},"event":{"item":"broker","operation":"startup"}}`, fakeTime, brokerIdentifier)))
				Expect(logBuffer).To(gbytes.Say(fmt.Sprintf(`{"telemetry-time":"%s","telemetry-source":"odb-%s","service-instances-per-plan":{"plan-id":"another-plan-unique-id","total":1},"event":{"item":"broker","operation":"startup"}}`, fakeTime, brokerIdentifier)))
			})

			It("logs only about total when there are no instances", func() {
				instanceLister.InstancesReturns([]service.Instance{}, nil)

				fakeTime := "fake-timer"
				telemetryTime.NowReturns(fakeTime)

				telemetryLogger.LogInstances(instanceLister, "not-relevant", "not-relevant")

				Expect(logBuffer).To(gbytes.Say(fmt.Sprintf(`"service-instances":{"total":0}`)))
				Expect(logBuffer).ToNot(gbytes.Say(fmt.Sprintf(`service-instances-per-plan`)))
			})

			It("logs error log when it cant get the total number of instances", func() {
				errorMessage := "opsie"
				instanceLister.InstancesReturns([]service.Instance{}, errors.New(errorMessage))

				telemetryLogger.LogInstances(instanceLister, "not-relevant", "not-relevant")

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

		Describe("LogInstances", func() {
			It("does not log telemetry", func() {
				telemetryLogger = telemetry.Build(false, brokerIdentifier, loggerFactory.New())

				telemetryLogger.LogInstances(instanceLister, "not-relevant", "not-relevant")

				Eventually(logBuffer).ShouldNot(gbytes.Say(`{"telemetry-source":`))
				Expect(instanceLister.InstancesCallCount()).To(BeZero(), "Instance listener was not called")
			})
		})
	})
})
