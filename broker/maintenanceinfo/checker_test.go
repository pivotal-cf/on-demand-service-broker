package maintenanceinfo_test

import (
	"bytes"
	"io"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"

	"github.com/pivotal-cf/on-demand-service-broker/broker/maintenanceinfo"
)

var _ = Describe("Checker", func() {
	var (
		logger                          *log.Logger
		logBuffer                       *bytes.Buffer
		planID                          string
		serviceCatalog                  []brokerapi.Service
		expectedPlanMaintenanceInfo     *brokerapi.MaintenanceInfo
		maintenanceInfoNotPassedWarning = "warning: maintenance info defined in broker service catalog, but not passed in request"
	)

	BeforeEach(func() {
		logBuffer = new(bytes.Buffer)
		loggerFactory := loggerfactory.New(io.MultiWriter(GinkgoWriter, logBuffer), "broker-unit-tests", log.LstdFlags)
		logger = loggerFactory.New()

		planID = "plan-id"

		expectedPlanMaintenanceInfo = &brokerapi.MaintenanceInfo{}

		serviceCatalog = []brokerapi.Service{
			{
				ID:   "some-service",
				Name: "some-service",
				Plans: []brokerapi.ServicePlan{
					{
						ID:              planID,
						Name:            "lol",
						MaintenanceInfo: expectedPlanMaintenanceInfo,
					},
				},
			},
		}
	})

	It("fails when plan not found", func() {
		checker := maintenanceinfo.Checker{}

		err := checker.Check("invalid-plan", brokerapi.MaintenanceInfo{}, serviceCatalog, logger)

		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(`plan invalid-plan not found`))
	})

	Context("configured without maintenance info", func() {
		It("succeeds and don't warn when maintenance_info is not passed", func() {
			checker := maintenanceinfo.Checker{}

			err := checker.Check(planID, brokerapi.MaintenanceInfo{}, serviceCatalog, logger)

			Expect(err).NotTo(HaveOccurred())
			Expect(logBuffer.String()).ToNot(ContainSubstring(maintenanceInfoNotPassedWarning))
		})

		It("fails when maintenance info is passed", func() {
			checker := maintenanceinfo.Checker{}

			err := checker.Check(planID, brokerapi.MaintenanceInfo{Version: "1.5.0"}, serviceCatalog, logger)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(brokerapi.ErrMaintenanceInfoNilConflict))
		})
	})

	Context("configured with maintenance info", func() {
		It("succeeds with a warning when maintenance info is not passed", func() {
			expectedPlanMaintenanceInfo.Version = "1.5.0"
			checker := maintenanceinfo.Checker{}

			err := checker.Check(planID, brokerapi.MaintenanceInfo{}, serviceCatalog, logger)

			Expect(err).NotTo(HaveOccurred())
			Expect(logBuffer.String()).To(ContainSubstring(maintenanceInfoNotPassedWarning))
		})

		It("succeeds when the maintenance info matches", func() {
			expectedPlanMaintenanceInfo.Public = map[string]string{
				"edition": "gold millennium",
			}
			expectedPlanMaintenanceInfo.Private = "test"
			expectedPlanMaintenanceInfo.Version = "1.2.3"

			maintenanceInfo := brokerapi.MaintenanceInfo{
				Public: map[string]string{
					"edition": "gold millennium",
				},
				Private: "test",
				Version: "1.2.3",
			}

			checker := maintenanceinfo.Checker{}

			err := checker.Check(planID, maintenanceInfo, serviceCatalog, logger)

			Expect(err).NotTo(HaveOccurred())
		})

		It("errors when the maintenance info doesn't match", func() {
			expectedPlanMaintenanceInfo.Public = map[string]string{
				"edition": "gold millennium",
			}
			expectedPlanMaintenanceInfo.Private = "test"
			expectedPlanMaintenanceInfo.Version = "1.2.3"

			maintenanceInfo := brokerapi.MaintenanceInfo{
				Public: map[string]string{
					"edition": "NEXT millennium",
				},
				Private: "test",
				Version: "1.2.3-rc3",
			}

			checker := maintenanceinfo.Checker{}

			err := checker.Check(planID, maintenanceInfo, serviceCatalog, logger)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(brokerapi.ErrMaintenanceInfoConflict))
		})
	})
})
