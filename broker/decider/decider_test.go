package decider_test

import (
	"bytes"
	"encoding/json"
	"errors"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi/v9/domain"
	"github.com/pivotal-cf/brokerapi/v9/domain/apiresponses"
	"github.com/pivotal-cf/on-demand-service-broker/broker/decider"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"io"
	"log"
	"net/http"
)

var _ = Describe("Decider", func() {
	const (
		planWithMI         = "fake-plan-id-with-mi"
		otherPlanWithMI    = "fake-other-plan-id-with-mi"
		planWithoutMI      = "fake-plan-id-no-mi"
		otherPlanWithoutMI = "fake-other-plan-id-no-mi"
	)
	var (
		catalog   []domain.Service
		logBuffer *bytes.Buffer
		logger    *log.Logger

		defaultMI *domain.MaintenanceInfo
		higherMI  *domain.MaintenanceInfo
	)

	BeforeEach(func() {
		defaultMI = &domain.MaintenanceInfo{
			Version: "1.2.3",
		}

		higherMI = &domain.MaintenanceInfo{
			Version: "1.2.4",
		}

		catalog = []domain.Service{
			{
				ID: "fake-service-id",
				Plans: []domain.ServicePlan{
					{ID: planWithMI, MaintenanceInfo: defaultMI},
					{ID: otherPlanWithMI, MaintenanceInfo: higherMI},
					{ID: planWithoutMI},
					{ID: otherPlanWithoutMI},
				},
			},
		}

		logBuffer = new(bytes.Buffer)
		loggerFactory := loggerfactory.New(io.MultiWriter(GinkgoWriter, logBuffer), "broker-unit-tests", log.LstdFlags)
		logger = loggerFactory.New()
	})

	Describe("DecideOperation()", func() {
		It("fails when the requested plan is not in the catalog", func() {
			details := domain.UpdateDetails{
				PlanID: "not-in-catalog",
			}

			_, err := decider.Decider{}.DecideOperation(catalog, details, logger)
			Expect(err).To(MatchError("plan not-in-catalog does not exist"))
		})

		It("is an update when the request doesn't include previous values", func() {
			details := domain.UpdateDetails{
				PlanID:          planWithMI,
				MaintenanceInfo: defaultMI,
			}

			operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(operation).To(Equal(decider.Update))
		})

		Context("request without maintenance_info", func() {
			It("does not warn when the catalog's plan doesn't have maintenance info either", func() {
				details := domain.UpdateDetails{
					PlanID: otherPlanWithoutMI,
					PreviousValues: domain.PreviousValues{
						PlanID: planWithoutMI,
					},
				}

				_, err := decider.Decider{}.DecideOperation(catalog, details, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(logBuffer.String()).To(BeEmpty())
			})

			When("the request is a change of plan", func() {
				It("is an update", func() {
					details := domain.UpdateDetails{
						PlanID: otherPlanWithoutMI,
						PreviousValues: domain.PreviousValues{
							PlanID: planWithoutMI,
						},
					}

					operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(operation).To(Equal(decider.Update))
				})
			})

			When("there are request parameters", func() {
				It("is an update", func() {
					details := domain.UpdateDetails{
						PlanID:        planWithoutMI,
						RawParameters: json.RawMessage(`{"foo": "bar"}`),
					}

					operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(operation).To(Equal(decider.Update))
				})
			})

			When("the plan does not change and there are no request parameters", func() {
				It("is an update", func() {
					details := domain.UpdateDetails{
						PlanID:        planWithoutMI,
						RawParameters: json.RawMessage(`{ }`),
						PreviousValues: domain.PreviousValues{
							PlanID: planWithoutMI,
						},
					}

					operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(operation).To(Equal(decider.Update))
				})
			})

			When("the request parameters is invalid JSON", func() {
				It("is an update", func() {
					details := domain.UpdateDetails{
						PlanID:        planWithoutMI,
						RawParameters: json.RawMessage(`{ --- }`),
						PreviousValues: domain.PreviousValues{
							PlanID: planWithoutMI,
						},
					}

					operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(operation).To(Equal(decider.Update))
				})
			})

			When("the desired plan has maintenance_info in the catalog", func() {
				When("no previous maintenance_info is present in the request", func() {
					When("the previous plan has maintenance info", func() {
						It("is an update, and it warns", func() {
							details := domain.UpdateDetails{
								PlanID: otherPlanWithMI,
								PreviousValues: domain.PreviousValues{
									PlanID: planWithMI,
								},
							}

							operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
							Expect(err).NotTo(HaveOccurred())
							Expect(operation).To(Equal(decider.Update))
							Expect(logBuffer.String()).To(ContainSubstring(
								"warning: maintenance info defined in broker service catalog, but not passed in request",
							))
						})
					})

					When("the previous plan doesn't have maintenance info", func() {
						It("is an update, and it warns", func() {
							details := domain.UpdateDetails{
								PlanID: planWithMI,
								PreviousValues: domain.PreviousValues{
									PlanID: planWithoutMI,
								},
							}

							operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
							Expect(err).NotTo(HaveOccurred())
							Expect(operation).To(Equal(decider.Update))
							Expect(logBuffer.String()).To(ContainSubstring(
								"warning: maintenance info defined in broker service catalog, but not passed in request",
							))
						})
					})

				})

				When("previous maintenance_info is present in the request", func() {
					It("fails when it does not match the catalog's maintenance info for the previous plan", func() {
						details := domain.UpdateDetails{
							PlanID: otherPlanWithMI,
							PreviousValues: domain.PreviousValues{
								PlanID:          planWithMI,
								MaintenanceInfo: higherMI,
							},
						}

						_, err := decider.Decider{}.DecideOperation(catalog, details, logger)
						Expect(err).To(MatchError(apiresponses.NewFailureResponseBuilder(
							errors.New("service instance needs to be upgraded before updating"),
							http.StatusUnprocessableEntity,
							"previous-maintenance-info-check",
						).Build()))
						Expect(logBuffer.String()).To(ContainSubstring(
							"warning: maintenance info defined in broker service catalog, but not passed in request",
						))
					})

					It("is an update when it matches the catalog's maintenance info for the previous plan", func() {
						details := domain.UpdateDetails{
							PlanID: otherPlanWithMI,
							PreviousValues: domain.PreviousValues{
								PlanID:          planWithMI,
								MaintenanceInfo: defaultMI,
							},
						}

						op, err := decider.Decider{}.DecideOperation(catalog, details, logger)
						Expect(err).ToNot(HaveOccurred())
						Expect(op).To(Equal(decider.Update))
						Expect(logBuffer.String()).To(ContainSubstring(
							"warning: maintenance info defined in broker service catalog, but not passed in request",
						))
					})
				})
			})
		})

		Context("request and plan have the same maintenance_info", func() {
			When("the request is a change of plan", func() {
				It("is an update", func() {
					details := domain.UpdateDetails{
						PlanID:          otherPlanWithMI,
						MaintenanceInfo: higherMI,
						PreviousValues: domain.PreviousValues{
							PlanID:          planWithMI,
							MaintenanceInfo: defaultMI,
						},
					}

					operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(operation).To(Equal(decider.Update))
				})
			})

			When("there are request parameters", func() {
				It("is an update", func() {
					details := domain.UpdateDetails{
						PlanID:          planWithMI,
						MaintenanceInfo: defaultMI,
						RawParameters:   json.RawMessage(`{"foo": "bar"}`),
					}

					operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(operation).To(Equal(decider.Update))
				})
			})

			When("the plan does not change and there are no request parameters", func() {
				It("is an update", func() {
					details := domain.UpdateDetails{
						PlanID:          planWithMI,
						MaintenanceInfo: defaultMI,
						RawParameters:   json.RawMessage(`{ }`),
						PreviousValues: domain.PreviousValues{
							PlanID:          planWithMI,
							MaintenanceInfo: defaultMI,
						},
					}

					operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(operation).To(Equal(decider.Update))
				})
			})

			When("the request parameters is invalid JSON", func() {
				It("is an update", func() {
					details := domain.UpdateDetails{
						PlanID:          planWithMI,
						MaintenanceInfo: defaultMI,
						RawParameters:   json.RawMessage(`{ --- }`),
						PreviousValues: domain.PreviousValues{
							PlanID:          planWithMI,
							MaintenanceInfo: defaultMI,
						},
					}

					operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(operation).To(Equal(decider.Update))
				})
			})
		})

		Context("request has different maintenance_info values", func() {
			When("adding maintenance_info when there was none before", func() {
				It("is an upgrade", func() {
					details := domain.UpdateDetails{
						PlanID:          planWithMI,
						MaintenanceInfo: defaultMI,
						PreviousValues: domain.PreviousValues{
							PlanID: planWithMI,
						},
					}

					operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(operation).To(Equal(decider.Upgrade))
				})
			})

			When("removing maintenance_info when it was there before", func() {
				It("is an upgrade", func() {
					details := domain.UpdateDetails{
						PlanID: planWithoutMI,
						PreviousValues: domain.PreviousValues{
							PlanID:          planWithoutMI,
							MaintenanceInfo: defaultMI,
						},
					}

					operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(operation).To(Equal(decider.Upgrade))
				})
			})

			When("the plan has not changed and there are no request parameters", func() {
				It("is an upgrade", func() {
					details := domain.UpdateDetails{
						PlanID:          planWithMI,
						MaintenanceInfo: defaultMI,
						PreviousValues: domain.PreviousValues{
							PlanID:          planWithMI,
							MaintenanceInfo: higherMI,
						},
					}

					operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(operation).To(Equal(decider.Upgrade))
				})
			})

			When("there is a change of plan", func() {
				It("fails when the previous maintenance_info does not match the previous plan maintenance_info", func() {
					details := domain.UpdateDetails{
						PlanID:          planWithMI,
						MaintenanceInfo: defaultMI,
						PreviousValues: domain.PreviousValues{
							PlanID:          otherPlanWithMI,
							MaintenanceInfo: defaultMI,
						},
					}

					_, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).To(MatchError(apiresponses.NewFailureResponseBuilder(
						errors.New("service instance needs to be upgraded before updating"),
						http.StatusUnprocessableEntity,
						"previous-maintenance-info-check",
					).Build()))
				})

				It("is an update when the previous maintenance_info matches the previous plan", func() {
					details := domain.UpdateDetails{
						PlanID:          otherPlanWithMI,
						MaintenanceInfo: higherMI,
						PreviousValues: domain.PreviousValues{
							PlanID:          planWithMI,
							MaintenanceInfo: defaultMI,
						},
					}

					operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(operation).To(Equal(decider.Update))
				})

				It("is an update when the previous plan is not in the catalog", func() {
					details := domain.UpdateDetails{
						PlanID:          planWithMI,
						MaintenanceInfo: defaultMI,
						PreviousValues: domain.PreviousValues{
							PlanID: "fake-plan-that-does-not-exist",
							MaintenanceInfo: &domain.MaintenanceInfo{
								Version: "1.2.1",
							},
						},
					}

					operation, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(operation).To(Equal(decider.Update))
				})
			})

			When("there are request parameters", func() {
				It("fails", func() {
					details := domain.UpdateDetails{
						PlanID:          planWithMI,
						RawParameters:   json.RawMessage(`{"foo": "bar"}`),
						MaintenanceInfo: defaultMI,
						PreviousValues: domain.PreviousValues{
							PlanID:          planWithMI,
							MaintenanceInfo: higherMI,
						},
					}

					_, err := decider.Decider{}.DecideOperation(catalog, details, logger)
					Expect(err).To(MatchError(apiresponses.NewFailureResponseBuilder(
						errors.New("service instance needs to be upgraded before updating"),
						http.StatusUnprocessableEntity,
						"previous-maintenance-info-check",
					).Build()))
				})
			})
		})

		Context("request and plan have different maintenance_info values", func() {
			It("fails when the maintenance_info requested does not match the plan", func() {
				details := domain.UpdateDetails{
					PlanID:          planWithMI,
					MaintenanceInfo: higherMI,
				}

				_, err := decider.Decider{}.DecideOperation(catalog, details, logger)

				Expect(err).To(MatchError(apiresponses.ErrMaintenanceInfoConflict))
			})

			It("fails when the request has maintenance_info but the plan does not", func() {
				details := domain.UpdateDetails{
					PlanID:          planWithoutMI,
					MaintenanceInfo: defaultMI,
				}

				_, err := decider.Decider{}.DecideOperation(catalog, details, logger)
				Expect(err).To(MatchError(apiresponses.ErrMaintenanceInfoNilConflict))
			})
		})
	})

	Describe("CanProvision()", func() {
		It("fails when the requested plan is not in the catalog", func() {
			err := decider.Decider{}.CanProvision(catalog, "not-in-catalog", nil, logger)
			Expect(err).To(MatchError("plan not-in-catalog does not exist"))
		})

		It("succeeds when the request maintenance_info matches the plan maintenance_info", func() {
			err := decider.Decider{}.CanProvision(catalog, planWithMI, defaultMI, logger)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("request and plan have different maintenance_info values", func() {
			It("fails when the maintenance_info requested does not match the plan", func() {
				err := decider.Decider{}.CanProvision(catalog, planWithMI, higherMI, logger)
				Expect(err).To(MatchError(apiresponses.ErrMaintenanceInfoConflict))
			})

			It("fails when the request has maintenance_info but the plan does not", func() {
				err := decider.Decider{}.CanProvision(catalog, planWithoutMI, defaultMI, logger)
				Expect(err).To(MatchError(apiresponses.ErrMaintenanceInfoNilConflict))
			})
		})

		Context("request without maintenance_info", func() {
			It("does not warn when the catalog's plan doesn't have maintenance info either", func() {
				err := decider.Decider{}.CanProvision(catalog, otherPlanWithoutMI, nil, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(logBuffer.String()).To(BeEmpty())
			})

			When("the plan has maintenance_info", func() {
				It("warns", func() {
					err := decider.Decider{}.CanProvision(catalog, planWithMI, nil, logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(logBuffer.String()).To(ContainSubstring(
						"warning: maintenance info defined in broker service catalog, but not passed in request",
					))
				})
			})
		})
	})
})
