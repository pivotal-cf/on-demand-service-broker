package telemetry_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/service/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/telemetry"
	. "github.com/pivotal-cf/on-demand-service-broker/telemetry/fakes_telemetry"
	"github.com/pkg/errors"
)

var _ = Describe("Telemetry", func() {
	var (
		instanceLister            *fakes.FakeInstanceLister
		logBuffer                 *gbytes.Buffer
		loggerFactory             *loggerfactory.LoggerFactory
		serviceOffering           config.ServiceOffering
		telemetryTime             *FakeTime
		planID1, planID2, planID3 string
	)

	Describe("Telemetry Logger enabled", func() {
		var telemetryLogger broker.TelemetryLogger

		BeforeEach(func() {
			planID1 = "plan-unique-id"
			planID2 = "another-plan-unique-id"
			planID3 = "plan-id-with-no-instances"

			serviceOffering = config.ServiceOffering{
				Name: "offering-name",
				Plans: config.Plans{
					{ID: planID1},
					{ID: planID2},
					{ID: planID3},
				},
			}
			logBuffer = gbytes.NewBuffer()
			loggerFactory = loggerfactory.New(logBuffer, serviceOffering.Name, loggerfactory.Flags)

			instanceLister = new(fakes.FakeInstanceLister)
			telemetryTime = new(FakeTime)

			telemetryLogger = telemetry.NewTelemetryLogger(loggerFactory.New(), serviceOffering, telemetryTime)
		})

		Describe("LogInstances", func() {
			It("logs telemetry about the total number of instances", func() {
				instanceLister.InstancesReturns([]service.Instance{{GUID: "test-guid"}}, nil)

				fakeTime := "2006-01-02 15:04:05"
				telemetryTime.NowReturns(fakeTime)

				telemetryLogger.LogInstances(instanceLister, "broker", "startup")

				Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf(`{"telemetry-time":"%s","telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"event":{"item":"broker","operation":"startup"},"service-instances":{"total":1}}`, fakeTime, serviceOffering.Name)))
			})

			It("logs telemetry about the number of instances per plan", func() {
				instanceLister.InstancesReturns([]service.Instance{
					{GUID: "test-guid-1", PlanUniqueID: planID1},
					{GUID: "test-guid-2", PlanUniqueID: planID1},
					{GUID: "test-guid-3", PlanUniqueID: planID2},
				}, nil)

				fakeTime := "fake-time"
				telemetryTime.NowReturns(fakeTime)

				telemetryLogger.LogInstances(instanceLister, "broker", "startup")

				Expect(logBuffer).To(gbytes.Say(fmt.Sprintf(`{"telemetry-time":"%s","telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"event":{"item":"broker","operation":"startup"},"service-instances-per-plan":{"plan-id":%q,"total":2}}`, fakeTime, serviceOffering.Name, planID1)))
				Expect(logBuffer).To(gbytes.Say(fmt.Sprintf(`{"telemetry-time":"%s","telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"event":{"item":"broker","operation":"startup"},"service-instances-per-plan":{"plan-id":%q,"total":1}}`, fakeTime, serviceOffering.Name, planID2)))
				Expect(logBuffer).To(gbytes.Say(fmt.Sprintf(`{"telemetry-time":"%s","telemetry-source":"on-demand-broker","service-offering":{"name":"%s"},"event":{"item":"broker","operation":"startup"},"service-instances-per-plan":{"plan-id":%q,"total":0}}`, fakeTime, serviceOffering.Name, planID3)))
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
				telemetryLogger = telemetry.Build(false, serviceOffering, loggerFactory.New())

				telemetryLogger.LogInstances(instanceLister, "not-relevant", "not-relevant")

				Eventually(logBuffer).ShouldNot(gbytes.Say(`{"telemetry-source":`))
				Expect(instanceLister.InstancesCallCount()).To(BeZero(), "Instance listener was not called")
			})
		})
	})
})
