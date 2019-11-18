package decider_test

import (
	"bytes"
	"encoding/json"
	"errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/v7/domain"
	"github.com/pivotal-cf/brokerapi/v7/domain/apiresponses"
	"github.com/pivotal-cf/on-demand-service-broker/broker/decider"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"io"
	"log"
	"net/http"
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
							Version: "1.2.3",
						},
					},
					{
						ID: "fake-other-plan-id-with-mi",
						MaintenanceInfo: &domain.MaintenanceInfo{
							Version: "1.2.3",
						},
					},
					{
						ID: "fake-other-plan-id-with-higher-mi",
						MaintenanceInfo: &domain.MaintenanceInfo{
							Version: "1.2.4",
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

	It("is an update when the request doesn't include previous values", func() {
		details := domain.UpdateDetails{
			PlanID: "fake-plan-id-with-mi",
			MaintenanceInfo: &domain.MaintenanceInfo{
				Version: "1.2.3",
			},
		}

		isUpgrade, err := decider.Decider{}.Decide(catalog, details, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(isUpgrade).To(BeFalse())
	})

	Context("request without maintenance_info", func() {
		It("does not warn", func() {
			details := domain.UpdateDetails{
				PlanID: "fake-other-plan-id-no-mi",
				PreviousValues: domain.PreviousValues{
					PlanID: "fake-plan-id-no-mi",
				},
			}

			_, err := decider.Decider{}.Decide(catalog, details, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(logBuffer.String()).To(BeEmpty())
		})

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

		When("the request parameters is invalid JSON", func() {
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

		When("the plan has maintenance_info", func() {
			It("is an update, and it warns", func() {
				details := domain.UpdateDetails{
					PlanID: "fake-plan-id-with-mi",
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-plan-id-with-mi",
					},
				}

				isUpgrade, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(isUpgrade).To(BeFalse())
				Expect(logBuffer.String()).To(ContainSubstring(
					"warning: maintenance info defined in broker service catalog, but not passed in request",
				))
			})
		})
	})

	Context("request and plan have the same maintenance_info", func() {
		When("the request is a change of plan", func() {
			It("is an update", func() {
				details := domain.UpdateDetails{
					PlanID: "fake-other-plan-id-with-mi",
					MaintenanceInfo: &domain.MaintenanceInfo{
						Version: "1.2.3",
					},
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-plan-id-with-mi",
						MaintenanceInfo: &domain.MaintenanceInfo{
							Version: "1.2.3",
						},
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
					PlanID: "fake-plan-id-with-mi",
					MaintenanceInfo: &domain.MaintenanceInfo{
						Version: "1.2.3",
					},
					RawParameters: json.RawMessage(`{"foo": "bar"}`),
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-plan-id-with-mi",
						MaintenanceInfo: &domain.MaintenanceInfo{
							Version: "1.2.3",
						},
					},
				}

				isUpgrade, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(isUpgrade).To(BeFalse())
			})
		})

		When("the plan does not change and there are no request parameters and the MI does not change", func() {
			It("is an upgrade", func() {
				details := domain.UpdateDetails{
					PlanID: "fake-plan-id-with-mi",
					MaintenanceInfo: &domain.MaintenanceInfo{
						Version: "1.2.3",
					},
					RawParameters: json.RawMessage(`{ }`),
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-plan-id-with-mi",
						MaintenanceInfo: &domain.MaintenanceInfo{
							Version: "1.2.3",
						},
					},
				}

				isUpgrade, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(isUpgrade).To(BeTrue())
			})
		})

		When("the request parameters is invalid JSON", func() {
			It("is an update", func() {
				details := domain.UpdateDetails{
					PlanID: "fake-plan-id-with-mi",
					MaintenanceInfo: &domain.MaintenanceInfo{
						Version: "1.2.3",
					},
					RawParameters: json.RawMessage(`{ --- }`),
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-plan-id-with-mi",
						MaintenanceInfo: &domain.MaintenanceInfo{
							Version: "1.2.3",
						},
					},
				}

				isUpgrade, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(isUpgrade).To(BeFalse())
			})
		})
	})

	Context("request and previous state have different maintenance_info", func() {
		When("adding maintenance_info when there was none before", func() {
			It("is an upgrade", func() {
				details := domain.UpdateDetails{
					PlanID: "fake-plan-id-with-mi",
					MaintenanceInfo: &domain.MaintenanceInfo{
						Version: "1.2.3",
					},
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-plan-id-with-mi",
					},
				}

				isUpgrade, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(isUpgrade).To(BeTrue())
			})
		})

		When("removing maintenance_info when it was there before", func() {
			It("is an upgrade", func() {
				details := domain.UpdateDetails{
					PlanID: "fake-plan-id-no-mi",
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-plan-id-no-mi",
						MaintenanceInfo: &domain.MaintenanceInfo{
							Version: "1.2.3",
						},
					},
				}

				isUpgrade, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(isUpgrade).To(BeTrue())
			})
		})

		When("the plan has not changed and there are no request parameters", func() {
			It("is an upgrade", func() {
				details := domain.UpdateDetails{
					PlanID: "fake-plan-id-with-mi",
					MaintenanceInfo: &domain.MaintenanceInfo{
						Version: "1.2.3",
					},
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-plan-id-with-mi",
						MaintenanceInfo: &domain.MaintenanceInfo{
							Version: "1.2.2",
						},
					},
				}

				isUpgrade, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(isUpgrade).To(BeTrue())
			})
		})

		When("there is a change of plan", func() {
			It("fails when the previous maintenance_info is does not match the previous plan maintenance_info", func() {
				details := domain.UpdateDetails{
					PlanID: "fake-plan-id-with-mi",
					MaintenanceInfo: &domain.MaintenanceInfo{
						Version: "1.2.3",
					},
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-other-plan-id-with-higher-mi",
						MaintenanceInfo: &domain.MaintenanceInfo{
							Version: "1.2.3",
						},
					},
				}

				_, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).To(MatchError(apiresponses.NewFailureResponseBuilder(
					errors.New("service instance needs to be upgraded before updating"),
					http.StatusUnprocessableEntity,
					"previous-maintenance-info-check",
				).Build()))
			})

			It("is an update when the previous maintenance_info matches the previous plan", func() {
				details := domain.UpdateDetails{
					PlanID: "fake-other-plan-id-with-higher-mi",
					MaintenanceInfo: &domain.MaintenanceInfo{
						Version: "1.2.4",
					},
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-other-plan-id-with-mi",
						MaintenanceInfo: &domain.MaintenanceInfo{
							Version: "1.2.3",
						},
					},
				}

				isUpgrade, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(isUpgrade).To(BeFalse())
			})

			It("is an update when the previous plan is not in the catalog", func() {
				details := domain.UpdateDetails{
					PlanID: "fake-plan-id-with-mi",
					MaintenanceInfo: &domain.MaintenanceInfo{
						Version: "1.2.3",
					},
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-plan-that-does-not-exist",
						MaintenanceInfo: &domain.MaintenanceInfo{
							Version: "1.2.1",
						},
					},
				}

				isUpgrade, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(isUpgrade).To(BeFalse())
			})
		})

		When("there are request parameters", func() {
			It("fails", func() {
				details := domain.UpdateDetails{
					PlanID:        "fake-plan-id-with-mi",
					RawParameters: json.RawMessage(`{"foo": "bar"}`),
					MaintenanceInfo: &domain.MaintenanceInfo{
						Version: "1.2.3",
					},
					PreviousValues: domain.PreviousValues{
						PlanID: "fake-plan-id-with-mi",
						MaintenanceInfo: &domain.MaintenanceInfo{
							Version: "1.2.2",
						},
					},
				}

				_, err := decider.Decider{}.Decide(catalog, details, logger)
				Expect(err).To(MatchError(apiresponses.NewFailureResponseBuilder(
					errors.New("service instance needs to be upgraded before updating"),
					http.StatusUnprocessableEntity,
					"previous-maintenance-info-check",
				).Build()))
			})
		})
	})

	Context("request and plan have different maintenance_info", func() {
		It("fails when the maintenance_info requested does not match the plan", func() {
			details := domain.UpdateDetails{
				PlanID: "fake-plan-id-with-mi",
				MaintenanceInfo: &domain.MaintenanceInfo{
					Public: map[string]string{
						"other-key": "other-value",
					},
				},
				PreviousValues: domain.PreviousValues{
					PlanID: "fake-plan-id-with-mi",
				},
			}

			_, err := decider.Decider{}.Decide(catalog, details, logger)

			Expect(err).To(MatchError(apiresponses.ErrMaintenanceInfoConflict))
		})

		It("fails when the request has maintenance_info but the plan does not", func() {
			details := domain.UpdateDetails{
				PlanID: "fake-plan-id-no-mi",
				MaintenanceInfo: &domain.MaintenanceInfo{
					Version: "1.2.3",
				},
				PreviousValues: domain.PreviousValues{
					PlanID: "fake-plan-id-no-mi",
				},
			}

			_, err := decider.Decider{}.Decide(catalog, details, logger)
			Expect(err).To(MatchError(apiresponses.ErrMaintenanceInfoNilConflict))
		})
	})
})

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
