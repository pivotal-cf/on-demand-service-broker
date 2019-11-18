package decider_test

import (
	"bytes"
	"encoding/json"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/v7/domain"
	"github.com/pivotal-cf/brokerapi/v7/domain/apiresponses"
	"github.com/pivotal-cf/on-demand-service-broker/broker/decider"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"io"
	"log"
)

var _ = Describe("Decider", func() {
	var (
		catalog   []domain.Service
		logBuffer *bytes.Buffer
		logger    *log.Logger
	)

	BeforeEach(func() {
		catalog = []domain.Service{
			{
				ID:          "fake-service-id",
				Name:        "fake-service-name",
				Description: "fake service description",
				Plans: []domain.ServicePlan{
					{
						ID: "fake-plan-id-with-mi",
						MaintenanceInfo: &domain.MaintenanceInfo{
							Public: map[string]string{
								"foo": "bar",
							},
						},
					},
					{
						ID: "fake-plan-id-no-mi",
					},
					{
						ID: "fake-other-plan-id-no-mi",
					},
				},
			},
		}

		logBuffer = new(bytes.Buffer)
		loggerFactory := loggerfactory.New(io.MultiWriter(GinkgoWriter, logBuffer), "broker-unit-tests", log.LstdFlags)
		logger = loggerFactory.New()
	})

	It("fails when the requested plan is not in the catalog", func() {
		details := domain.UpdateDetails{
			PlanID: "not-in-catalog",
		}

		_, err := decider.Decider{}.Decide(catalog, details, logger)

		Expect(err).To(MatchError("plan not-in-catalog does not exist"))
	})

	Context("without maintenance_info", func() {
		When("the request is a change of plan", func() {
			It("is an update", func() {
				details := domain.UpdateDetails{
					PlanID: "fake-other-plan-id-no-mi",
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-plan-id-no-mi",
					},
				}

				isUpgrade, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(isUpgrade).To(BeFalse())
			})
		})

		When("there are request parameters", func() {
			It("is an update", func() {
				details := domain.UpdateDetails{
					PlanID:        "fake-plan-id-no-mi",
					RawParameters: json.RawMessage(`{"foo": "bar"}`),
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-plan-id-no-mi",
					},
				}

				isUpgrade, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(isUpgrade).To(BeFalse())
			})
		})

		When("the plan does not change and there are no request parameters", func() {
			It("is an upgrade", func() {
				details := domain.UpdateDetails{
					PlanID:        "fake-plan-id-no-mi",
					RawParameters: json.RawMessage(`{ }`),
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-plan-id-no-mi",
					},
				}

				isUpgrade, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(isUpgrade).To(BeTrue())
			})
		})

		When("the request parameters is invalid JSON (and the plan does not change)", func() {
			It("is an update", func() {
				details := domain.UpdateDetails{
					PlanID:        "fake-plan-id-no-mi",
					RawParameters: json.RawMessage(`{ --- }`),
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-plan-id-no-mi",
					},
				}

				isUpgrade, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(isUpgrade).To(BeFalse())
			})
		})
	})

	Context("maintenance_info mismatches", func() {
		It("fails when the maintenance_info requested does not match the plan", func() {
			details := domain.UpdateDetails{
				PlanID: "fake-plan-id-with-mi",
				MaintenanceInfo: &domain.MaintenanceInfo{
					Public: map[string]string{
						"other-key": "other-value",
					},
				},
			}

			_, err := decider.Decider{}.Decide(catalog, details, logger)

			Expect(err).To(MatchError(apiresponses.ErrMaintenanceInfoConflict))
		})

		It("fails when the request contains maintenance_info but the plan does not", func() {
			details := domain.UpdateDetails{
				PlanID: "fake-plan-id-with-mi",
			}

			_, err := decider.Decider{}.Decide(catalog, details, logger)
			Expect(err).To(MatchError(apiresponses.ErrMaintenanceInfoConflict))
		})

		It("fails when the plan contains maintenance_info but the request does not", func() {
			details := domain.UpdateDetails{
				PlanID: "fake-plan-id-no-mi",
				MaintenanceInfo: &domain.MaintenanceInfo{
					Public: map[string]string{"some": "thing"},
				},
			}

			_, err := decider.Decider{}.Decide(catalog, details, logger)
			Expect(err).To(MatchError(apiresponses.ErrMaintenanceInfoConflict))

		})
	})

	It("does not fail when the requested plan is in the catalog and has matching maintenance_info", func() {
		details := domain.UpdateDetails{
			PlanID: "fake-plan-id-with-mi",
			MaintenanceInfo: &domain.MaintenanceInfo{
				Public: map[string]string{
					"foo": "bar",
				},
			},
		}

		_, err := decider.Decider{}.Decide(catalog, details, logger)
		Expect(err).NotTo(HaveOccurred())
	})

	It("does not fail if the request and the catalog do not have maintenance_info", func() {
		details := domain.UpdateDetails{
			PlanID: "fake-plan-id-no-mi",
		}

		_, err := decider.Decider{}.Decide(catalog, details, logger)

		Expect(err).NotTo(HaveOccurred())
	})

})

//
//	Describe("universal validation", func() {
//
//		It("fails when the requested MI for the requested plan does not match the MI for that plan in the catalog", func() {
//			catalog[0].Plans[0].MaintenanceInfo = &domain.MaintenanceInfo{Version: "1.2.3"}
//
//			d := decider.Decider{
//				RequestPlanID:          "fake-plan-id-with-mi",
//				RequestMaintenanceInfo: &domain.MaintenanceInfo{Version: "1.2.0"},
//				ServiceCatalog:         catalog,
//			}
//
//			nature, err := d.Decide()
//			Expect(err).To(MatchError("decider validation failed: passed maintenance_info does not match the catalog maintenance_info"))
//			Expect(nature).To(Equal(decider.Failed))
//		})
//	})
//
//	Describe("deciding between updates and upgrades", func() {
//		Context("valid updates", func() {
//			It("has request parameters", func() {
//				d := decider.Decider{
//					RequestPlanID:            "fake-plan-id-with-mi",
//					PreviousPlanID:           "fake-plan-id-with-mi",
//					ServiceCatalog:           catalog,
//					RequestParametersPresent: true,
//				}
//
//				nature, err := d.Decide()
//				Expect(err).NotTo(HaveOccurred())
//				Expect(nature).To(Equal(decider.Update))
//			})
//
//			It("is a plan change", func() {
//				d := decider.Decider{
//					RequestPlanID:  "fake-plan-id-no-mi",
//					PreviousPlanID: "fake-plan-id-with-mi",
//					ServiceCatalog: catalog,
//				}
//
//				nature, err := d.Decide()
//				Expect(err).NotTo(HaveOccurred())
//				Expect(nature).To(Equal(decider.Update))
//			})
//		})
//
//		Context("valid upgrades", func() {
//			It("is an addition of MI when there was none before", func() {
//				catalog[0].Plans[0].MaintenanceInfo = &domain.MaintenanceInfo{Version: "1.2.3"}
//				d := decider.Decider{
//					RequestPlanID:           "fake-plan-id-with-mi",
//					RequestMaintenanceInfo:  &domain.MaintenanceInfo{Version: "1.2.3"},
//					PreviousPlanID:          "fake-plan-id-with-mi",
//					PreviousMaintenanceInfo: nil,
//					ServiceCatalog:          catalog,
//				}
//
//				nature, err := d.Decide()
//				Expect(err).NotTo(HaveOccurred())
//				Expect(nature).To(Equal(decider.Upgrade))
//			})
//
//			It("is a removal of MI when it was set before", func() {
//				catalog[0].Plans[0].MaintenanceInfo = nil
//				d := decider.Decider{
//					RequestPlanID:           "fake-plan-id-with-mi",
//					RequestMaintenanceInfo:  nil,
//					PreviousPlanID:          "fake-plan-id-with-mi",
//					PreviousMaintenanceInfo: &domain.MaintenanceInfo{Version: "1.2.3"},
//					ServiceCatalog:          catalog,
//				}
//
//				nature, err := d.Decide()
//				Expect(err).NotTo(HaveOccurred())
//				Expect(nature).To(Equal(decider.Upgrade))
//			})
//
//			It("is a change of MI from what was there before", func() {
//				catalog[0].Plans[0].MaintenanceInfo = &domain.MaintenanceInfo{Version: "1.2.3"}
//				d := decider.Decider{
//					RequestPlanID:           "fake-plan-id-with-mi",
//					RequestMaintenanceInfo:  &domain.MaintenanceInfo{Version: "1.2.3"},
//					PreviousPlanID:          "fake-plan-id-with-mi",
//					PreviousMaintenanceInfo: &domain.MaintenanceInfo{Version: "1.2.2"},
//					ServiceCatalog:          catalog,
//				}
//
//				nature, err := d.Decide()
//				Expect(err).NotTo(HaveOccurred())
//				Expect(nature).To(Equal(decider.Upgrade))
//			})
//		})
//
//		Context("invalid requests", func() {
//			It("is a plan change and the SI is not already at the latest MI", func() {
//				catalog[0].Plans[0].MaintenanceInfo = &domain.MaintenanceInfo{Version: "1.2.3"}
//				catalog[0].Plans[1].MaintenanceInfo = &domain.MaintenanceInfo{Version: "1.2.2"}
//
//				d := decider.Decider{
//					RequestPlanID:           "fake-plan-id-with-mi",
//					RequestMaintenanceInfo:  &domain.MaintenanceInfo{Version: "1.2.3"},
//					PreviousPlanID:          "fake-plan-id-no-mi",
//					PreviousMaintenanceInfo: &domain.MaintenanceInfo{Version: "1.2.1"},
//					ServiceCatalog:          catalog,
//				}
//
//				nature, err := d.Decide()
//				Expect(err).To(MatchError("service instance needs to be upgraded before updating"))
//				Expect(nature).To(Equal(decider.Failed))
//			})
//
//			It("is has request parameters and the SI is not already at the latest MI", func() {
//				catalog[0].Plans[0].MaintenanceInfo = &domain.MaintenanceInfo{Version: "1.2.3"}
//
//				d := decider.Decider{
//					RequestParametersPresent: true,
//					RequestPlanID:            "fake-plan-id-with-mi",
//					RequestMaintenanceInfo:   &domain.MaintenanceInfo{Version: "1.2.3"},
//					PreviousPlanID:           "fake-plan-id-with-mi",
//					PreviousMaintenanceInfo:  &domain.MaintenanceInfo{Version: "1.2.2"},
//					ServiceCatalog:           catalog,
//				}
//
//				nature, err := d.Decide()
//				Expect(err).To(MatchError("service instance needs to be upgraded before updating"))
//				Expect(nature).To(Equal(decider.Failed))
//			})
//		})
//	})
//})
