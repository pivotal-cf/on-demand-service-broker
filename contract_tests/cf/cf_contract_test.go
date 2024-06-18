package cf_test

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/service_helpers"
)

var _ = Describe("CF client", func() {
	var (
		subject *cf.Client
		logger  *log.Logger
	)

	BeforeEach(func() {
		logBuffer := gbytes.NewBuffer()
		logger = log.New(io.MultiWriter(logBuffer, GinkgoWriter), "my-app", log.LstdFlags)
		subject = NewCFClient(logger)
	})

	Describe("Broker Operations", func() {
		var (
			brokerDeployment                 bosh_helpers.BrokerInfo
			brokerName, brokerGUID, planName string
		)

		BeforeEach(func() {
			brokerDeployment = bosh_helpers.DeployBroker(
				uuid.New()[:8]+"-cf-contract-tests",
				bosh_helpers.BrokerDeploymentOptions{
					ServiceMetrics: false,
					BrokerTLS:      false,
				},
				service_helpers.Redis,
				[]string{"basic_service_catalog.yml"},
			)

			brokerName = "contract-" + brokerDeployment.TestSuffix
			planName = "redis-small"
		})

		AfterEach(func() {
			session := cf_helpers.Cf("delete-service-broker", "-f", brokerName)
			Expect(session).To(gexec.Exit(0))

			bosh_helpers.DeleteDeployment(brokerDeployment.DeploymentName)
		})

		It("manages broker operations successfully", func() {
			By("creating a broker with CreateServiceBroker", func() {
				err := subject.CreateServiceBroker(
					brokerName,
					brokerDeployment.BrokerUsername,
					brokerDeployment.BrokerPassword,
					"http://"+brokerDeployment.URI,
				)
				Expect(err).NotTo(HaveOccurred())

				session := cf_helpers.Cf("service-brokers")
				Expect(session).To(gexec.Exit(0))
				Expect(session).To(gbytes.Say(brokerDeployment.URI))
			})

			By("verifying the existing brokers with ServiceBrokers", func() {
				serviceBrokers, err := subject.ServiceBrokers()
				Expect(err).NotTo(HaveOccurred())

				found := false
				for _, serviceBroker := range serviceBrokers {
					if serviceBroker.Name == brokerName {
						found = true
						brokerGUID = serviceBroker.GUID
						break
					}
				}
				Expect(found).To(BeTrue(), "List of brokers did not include the created broker")
			})

			By("updating the broker with UpdateServiceBroker", func() {
				newBrokerName := "new-" + brokerName

				err := subject.UpdateServiceBroker(brokerGUID, newBrokerName, brokerDeployment.BrokerUsername, brokerDeployment.BrokerPassword, "http://"+brokerDeployment.URI)
				Expect(err).NotTo(HaveOccurred())

				session := cf_helpers.Cf("service-brokers")
				Expect(session).To(gexec.Exit(0))
				Expect(session).To(gbytes.Say(newBrokerName))

				brokerName = newBrokerName
			})

			By("enabling plan access with EnableServiceAccess", func() {
				By("SETUP: Plan is restricted to an org", func() {
					session := cf_helpers.Cf("disable-service-access", brokerDeployment.ServiceName, "-p", planName)
					Expect(session).To(gexec.Exit(0))

					session = cf_helpers.Cf("enable-service-access", brokerDeployment.ServiceName, "-p", planName, "-o", os.Getenv("CF_ORG"))
					Expect(session).To(gexec.Exit(0))

					plan := servicePlan(brokerGUID, planName)
					Expect(plan.Public).To(BeFalse(), "Plan expected to be private")

					planVisibilities := servicePlanVisibilities(plan.GUID)
					Expect(len(planVisibilities)).To(BeNumerically(">=", 1), "Plan visibilities were not created")
				})

				err := subject.EnableServiceAccess(brokerDeployment.ServiceID, planName, logger)
				Expect(err).ToNot(HaveOccurred())
				updatedPlan := servicePlan(brokerGUID, planName)

				By("ensuring the plan is now public", func() {
					Expect(updatedPlan.Public).To(BeTrue(), "plan was not made public")
				})

				By("ensuring the plan visibilities were deleted", func() {
					updatedPlanVisibilities := servicePlanVisibilities(updatedPlan.GUID)
					Expect(len(updatedPlanVisibilities)).To(Equal(0), "plan visibilities were not cleaned")
				})
			})

			By("disabling plan access with DisableServiceAccess", func() {
				By("turning public plans into private", func() {
					By("SETUP: Plan is public", func() {
						session := cf_helpers.Cf("enable-service-access", brokerDeployment.ServiceName, "-p", planName)
						Expect(session).To(gexec.Exit(0))
						plan := servicePlan(brokerGUID, planName)
						Expect(plan.Public).To(BeTrue())
					})

					err := subject.DisableServiceAccess(brokerDeployment.ServiceID, planName, logger)
					Expect(err).ToNot(HaveOccurred())

					updatedPlan := servicePlan(brokerGUID, planName)
					Expect(updatedPlan.Public).To(BeFalse(), "plan was not made private")
				})

				By("removing visibilities of org-restricted plans", func() {
					plan := servicePlan(brokerGUID, planName)
					By("SETUP: Plan is restricted to an org", func() {
						session := cf_helpers.Cf("disable-service-access", brokerDeployment.ServiceName, "-p", planName)
						Expect(session).To(gexec.Exit(0))

						session = cf_helpers.Cf("enable-service-access", brokerDeployment.ServiceName, "-p", planName, "-o", os.Getenv("CF_ORG"))
						Expect(session).To(gexec.Exit(0))

						Expect(plan.Public).To(BeFalse(), "Plan expected to be private")

						planVisibilities := servicePlanVisibilities(plan.GUID)
						Expect(len(planVisibilities)).To(BeNumerically(">=", 1), "Plan visibilities were not created")
					})

					planVisibilities := servicePlanVisibilities(plan.GUID)
					Expect(len(planVisibilities)).To(BeNumerically(">=", 1))

					err := subject.DisableServiceAccess(brokerDeployment.ServiceID, planName, logger)
					Expect(err).ToNot(HaveOccurred())

					updatedPlanVisibilities := servicePlanVisibilities(plan.GUID)
					Expect(len(updatedPlanVisibilities)).To(Equal(0), "plan visibilities were not cleaned")
				})
			})

			By("disabling access to all plans with DisableServiceAccessForAllPlans", func() {
				By("SETUP: Plan is public", func() {
					session := cf_helpers.Cf("enable-service-access", brokerDeployment.ServiceName, "-p", planName)
					Expect(session).To(gexec.Exit(0))
					plan := servicePlan(brokerGUID, planName)
					Expect(plan.Public).To(BeTrue())
				})

				err := subject.DisableServiceAccessForAllPlans(brokerDeployment.ServiceID, logger)

				Expect(err).NotTo(HaveOccurred())

				session := cf_helpers.Cf("m")
				Expect(session).To(gexec.Exit(0))
				Expect(session.Out).NotTo(gbytes.Say(planName))
			})

			By("creating plan visibilities with CreateServicePlanVisibility", func() {
				By("SETUP: Plan is disabled", func() {
					session := cf_helpers.Cf("disable-service-access", brokerDeployment.ServiceName, "-p", planName)
					Expect(session).To(gexec.Exit(0))
				})
				orgName := os.Getenv("CF_ORG")
				plan := servicePlan(brokerGUID, planName)

				planVisibilities := servicePlanVisibilities(plan.GUID)
				Expect(len(planVisibilities)).To(BeZero())

				err := subject.CreateServicePlanVisibility(orgName, brokerDeployment.ServiceID, planName, logger)
				Expect(err).ToNot(HaveOccurred())

				createdPlanVisibilities := servicePlanVisibilities(plan.GUID)
				Expect(len(createdPlanVisibilities)).To(Equal(1), "service plan visibility was not created")

				Expect(createdPlanVisibilities[0].ServicePlanGUID).To(Equal(plan.GUID))
				Expect(createdPlanVisibilities[0].OrganizationGUID).To(Equal(orgGuid(orgName)))
			})
		})
	})
})

func orgGuid(orgName string) string {
	session := cf_helpers.Cf("org", "--guid", orgName)
	Expect(session).To(gexec.Exit(0))
	return strings.TrimSpace(string(session.Out.Contents()))
}

type ServicePlan struct {
	GUID   string `json:"guid"`
	Public bool   `json:"public"`
}

type PlanVisibility struct {
	GUID             string `json:"guid"`
	ServicePlanGUID  string `json:"service_plan_guid"`
	OrganizationGUID string `json:"organization_guid"`
}

func servicePlanVisibilities(planGUID string) []PlanVisibility {
	session := cf_helpers.Cf("curl", "/v2/service_plan_visibilities?q=service_plan_guid:"+planGUID)
	Expect(session).To(gexec.Exit(0))
	rawVisibilities := session.Out.Contents()

	var parsedVisibilities struct {
		Resources []struct {
			Metadata struct {
				GUID string `json:"guid"`
			} `json:"metadata"`
			Entity struct {
				ServicePlanGUID  string `json:"service_plan_guid"`
				OrganizationGUID string `json:"organization_guid"`
			} `json:"entity"`
		} `json:"resources"`
	}

	Expect(json.Unmarshal(rawVisibilities, &parsedVisibilities)).To(Succeed())

	visibilities := []PlanVisibility{}
	for _, visibility := range parsedVisibilities.Resources {
		visibilities = append(visibilities, PlanVisibility{
			GUID:             visibility.Metadata.GUID,
			ServicePlanGUID:  visibility.Entity.ServicePlanGUID,
			OrganizationGUID: visibility.Entity.OrganizationGUID,
		})
	}
	return visibilities
}

func servicePlan(brokerGUID, planName string) ServicePlan {
	session := cf_helpers.Cf("curl", "/v2/service_plans?q=service_broker_guid:"+brokerGUID)
	Expect(session).To(gexec.Exit(0))
	rawPlans := session.Out.Contents()

	var parsedPlans struct {
		Resources []struct {
			Metadata struct {
				GUID string `json:"guid"`
			} `json:"metadata"`
			Entity struct {
				Name   string `json:"name"`
				Public bool   `json:"public"`
			} `json:"entity"`
		} `json:"resources"`
	}

	Expect(json.Unmarshal(rawPlans, &parsedPlans)).To(Succeed())
	for _, plan := range parsedPlans.Resources {
		if plan.Entity.Name == planName {
			return ServicePlan{GUID: plan.Metadata.GUID, Public: plan.Entity.Public}
		}
	}
	Fail("Plan not found")
	return ServicePlan{}
}
