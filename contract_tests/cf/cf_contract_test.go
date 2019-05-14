package cf_test

import (
	"encoding/json"
	"io"
	"log"
	"os"

	. "github.com/onsi/ginkgo"
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
		brokerDeployment bosh_helpers.BrokerInfo
		subject          *cf.Client
		logger           *log.Logger
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

		logBuffer := gbytes.NewBuffer()
		logger = log.New(io.MultiWriter(logBuffer, GinkgoWriter), "my-app", log.LstdFlags)
		subject = NewCFClient(logger)
	})

	AfterEach(func() {
		bosh_helpers.DeleteDeployment(brokerDeployment.DeploymentName)
	})

	Describe("CreateServiceBroker", func() {
		var brokerName string

		BeforeEach(func() {
			brokerName = "contract-" + brokerDeployment.TestSuffix
		})

		AfterEach(func() {
			session := cf_helpers.Cf("delete-service-broker", "-f", brokerName)
			Eventually(session).Should(gexec.Exit(0))
		})

		It("creates a service broker", func() {
			err := subject.CreateServiceBroker(
				brokerName,
				brokerDeployment.BrokerUsername,
				brokerDeployment.BrokerPassword,
				"http://"+brokerDeployment.URI,
			)
			Expect(err).NotTo(HaveOccurred())

			session := cf_helpers.Cf("service-brokers")
			Eventually(session).Should(gexec.Exit(0))

			Expect(session).To(gbytes.Say(brokerDeployment.URI))
		})
	})

	Describe("ServiceBrokers", func() {
		var brokerName string

		BeforeEach(func() {
			brokerName = "contract-" + brokerDeployment.TestSuffix
			session := cf_helpers.Cf("create-service-broker",
				brokerName,
				brokerDeployment.BrokerUsername,
				brokerDeployment.BrokerPassword,
				"http://"+brokerDeployment.URI,
			)
			Eventually(session).Should(gexec.Exit(0))
		})

		AfterEach(func() {
			session := cf_helpers.Cf("delete-service-broker", "-f", brokerName)
			Eventually(session).Should(gexec.Exit())
		})

		It("returns a list of service brokers", func() {
			serviceBrokers, err := subject.ServiceBrokers()
			Expect(err).NotTo(HaveOccurred())

			found := false
			for _, serviceBroker := range serviceBrokers {
				if serviceBroker.Name == brokerName {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "List of brokers did not include the created broker")
		})
	})

	Describe("UpdateServiceBroker", func() {
		var (
			brokerName string
			brokerGUID string
		)

		BeforeEach(func() {
			brokerName = "contract-" + brokerDeployment.TestSuffix
			session := cf_helpers.Cf("create-service-broker",
				brokerName,
				brokerDeployment.BrokerUsername,
				brokerDeployment.BrokerPassword,
				"http://"+brokerDeployment.URI,
			)
			Eventually(session).Should(gexec.Exit(0))

			var err error
			brokerGUID, err = subject.GetServiceOfferingGUID(brokerName, logger)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			session := cf_helpers.Cf("delete-service-broker", "-f", brokerName)
			Eventually(session).Should(gexec.Exit())
		})

		It("returns a list of service brokers", func() {
			brokerName = "new-" + brokerName
			err := subject.UpdateServiceBroker(brokerGUID, brokerName, brokerDeployment.BrokerUsername, brokerDeployment.BrokerPassword, "http://"+brokerDeployment.URI)
			Expect(err).NotTo(HaveOccurred())

			session := cf_helpers.Cf("service-brokers")
			Eventually(session).Should(gexec.Exit(0))
			Expect(session).To(gbytes.Say(brokerName))
		})
	})

	Describe("EnableServiceAccess", func() {
		var (
			brokerName string
			brokerGUID string
			planName   string
		)

		BeforeEach(func() {
			planName = "redis-small"

			brokerName = "contract-" + brokerDeployment.TestSuffix
			session := cf_helpers.Cf("create-service-broker",
				brokerName,
				brokerDeployment.BrokerUsername,
				brokerDeployment.BrokerPassword,
				"http://"+brokerDeployment.URI,
			)
			Eventually(session).Should(gexec.Exit(0))

			Eventually(
				cf_helpers.Cf("disable-service-access", brokerDeployment.ServiceOffering, "-p", planName),
			).Should(gexec.Exit(0))

			Eventually(
				cf_helpers.Cf("enable-service-access", brokerDeployment.ServiceOffering, "-p", planName, "-o", os.Getenv("CF_ORG")),
			).Should(gexec.Exit(0))

			var err error
			brokerGUID, err = subject.GetServiceOfferingGUID(brokerName, logger)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			session := cf_helpers.Cf("delete-service-broker", "-f", brokerName)
			Eventually(session).Should(gexec.Exit(0))
		})

		It("enables service access", func() {
			plan := servicePlan(brokerGUID, planName)
			Expect(plan.Public).To(BeFalse())

			planVisibilities := servicePlanVisibilities(plan.GUID)
			Expect(len(planVisibilities)).To(BeNumerically(">=", 1))

			err := subject.EnableServiceAccess(brokerDeployment.ServiceOffering, planName, logger)
			Expect(err).ToNot(HaveOccurred())

			updatedPlan := servicePlan(brokerGUID, planName)
			Expect(updatedPlan.Public).To(BeTrue(), "plan was not made public")

			updatedPlanVisibilities := servicePlanVisibilities(plan.GUID)
			Expect(len(updatedPlanVisibilities)).To(Equal(0), "plan visibilities were not cleaned")
		})
	})

	Describe("DisableServiceAccess", func() {
		var (
			brokerName string
			brokerGUID string
			planName   string
		)

		BeforeEach(func() {
			planName = "redis-small"

			brokerName = "contract-" + brokerDeployment.TestSuffix
			session := cf_helpers.Cf("create-service-broker",
				brokerName,
				brokerDeployment.BrokerUsername,
				brokerDeployment.BrokerPassword,
				"http://"+brokerDeployment.URI,
			)
			Eventually(session).Should(gexec.Exit(0))

			Eventually(
				cf_helpers.Cf("enable-service-access", brokerDeployment.ServiceOffering, "-p", planName),
			).Should(gexec.Exit(0))

			var err error
			brokerGUID, err = subject.GetServiceOfferingGUID(brokerName, logger)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			session := cf_helpers.Cf("delete-service-broker", "-f", brokerName)
			Eventually(session).Should(gexec.Exit(0))
		})

		It("disables service access", func() {
			plan := servicePlan(brokerGUID, planName)

			By("setting plan.public to false", func() {
				Expect(plan.Public).To(BeTrue())

				err := subject.DisableServiceAccess(brokerDeployment.ServiceOffering, planName, logger)
				Expect(err).ToNot(HaveOccurred())

				updatedPlan := servicePlan(brokerGUID, planName)
				Expect(updatedPlan.Public).To(BeFalse(), "plan was not made private")
			})

			By("removing any plan visibilities", func() {
				Eventually(
					cf_helpers.Cf("enable-service-access", brokerDeployment.ServiceOffering, "-p", planName, "-o", os.Getenv("CF_ORG")),
				).Should(gexec.Exit(0))

				planVisibilities := servicePlanVisibilities(plan.GUID)
				Expect(len(planVisibilities)).To(BeNumerically(">=", 1))

				err := subject.DisableServiceAccess(brokerDeployment.ServiceOffering, planName, logger)
				Expect(err).ToNot(HaveOccurred())

				updatedPlanVisibilities := servicePlanVisibilities(plan.GUID)
				Expect(len(updatedPlanVisibilities)).To(Equal(0), "plan visibilities were not cleaned")
			})
		})
	})
})

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
	Eventually(session).Should(gexec.Exit(0))
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
	Eventually(session).Should(gexec.Exit(0))
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
