package registrar_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/registrar"
	"github.com/pivotal-cf/on-demand-service-broker/registrar/fakes"
)

var _ = Describe("RegisterBrokerRunner", func() {
	Describe("create or update service broker", func() {
		var (
			runner           registrar.RegisterBrokerRunner
			fakeCFClient     *fakes.FakeRegisterBrokerCFClient
			expectedUsername string
			expectedPassword string
			expectedURL      string
			expectedName     string
			plans            []config.PlanAccess
		)

		BeforeEach(func() {
			expectedName = "brokername"
			expectedUsername = "brokeruser"
			expectedPassword = "brokerpass"
			expectedURL = "http://broker.url.example.com"
			plans = []config.PlanAccess{{
				Name:            "not-relevant",
				CFServiceAccess: config.PlanEnabled,
			}}

			fakeCFClient = new(fakes.FakeRegisterBrokerCFClient)

			runner = registrar.RegisterBrokerRunner{
				Config: config.RegisterBrokerErrandConfig{
					BrokerName:     expectedName,
					BrokerUsername: expectedUsername,
					BrokerPassword: expectedPassword,
					BrokerURL:      expectedURL,
					ServiceName:    "not-relevant",
					Plans:          plans,
				},
				CFClient: fakeCFClient,
			}
		})

		It("creates a broker in CF when the broker does not already exist in CF", func() {
			fakeCFClient.ServiceBrokersReturns([]cf.ServiceBroker{}, nil)

			err := runner.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCFClient.CreateServiceBrokerCallCount()).To(Equal(1), "create service broker wasn't called")

			actualName, actualUsername, actualPassword, actualURL := fakeCFClient.CreateServiceBrokerArgsForCall(0)
			Expect(actualName).To(Equal(expectedName))
			Expect(actualUsername).To(Equal(expectedUsername))
			Expect(actualPassword).To(Equal(expectedPassword))
			Expect(actualURL).To(Equal(expectedURL))
		})

		It("updates a broker in CF when the broker already exists", func() {
			expectedGUID := "broker-guid"
			fakeCFClient.ServiceBrokersReturns([]cf.ServiceBroker{{
				GUID: expectedGUID,
				Name: expectedName,
			}}, nil)

			err := runner.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCFClient.UpdateServiceBrokerCallCount()).To(Equal(1), "update service broker wasn't called")
			Expect(fakeCFClient.CreateServiceBrokerCallCount()).To(Equal(0), "create service broker was called")

			actualGUID, actualName, actualUsername, actualPassword, actualURL := fakeCFClient.UpdateServiceBrokerArgsForCall(0)

			Expect(actualGUID).To(Equal(expectedGUID))
			Expect(actualName).To(Equal(expectedName))
			Expect(actualUsername).To(Equal(expectedUsername))
			Expect(actualPassword).To(Equal(expectedPassword))
			Expect(actualURL).To(Equal(expectedURL))
		})
	})

	Describe("Cf service access", func() {
		var (
			runner              registrar.RegisterBrokerRunner
			fakeCFClient        *fakes.FakeRegisterBrokerCFClient
			expectedServiceName string
			expectedPlanName1   string
			expectedPlanName2   string
			servicePlans        []config.PlanAccess
		)

		BeforeEach(func() {
			expectedServiceName = "serviceName"
			expectedPlanName1 = "planName-1"
			expectedPlanName2 = "planName-2"
			servicePlans = []config.PlanAccess{
				{
					Name:            expectedPlanName1,
					CFServiceAccess: config.PlanEnabled,
				},
				{
					Name:            expectedPlanName2,
					CFServiceAccess: config.PlanEnabled,
				},
			}

			fakeCFClient = new(fakes.FakeRegisterBrokerCFClient)
		})

		It("enable service access for a plan", func() {
			runner = registrar.RegisterBrokerRunner{
				Config: config.RegisterBrokerErrandConfig{
					ServiceName: expectedServiceName,
					Plans:       servicePlans,
				},
				CFClient: fakeCFClient,
			}
			fakeCFClient.ServiceBrokersReturns([]cf.ServiceBroker{}, nil)

			err := runner.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCFClient.EnableServiceAccessCallCount()).To(Equal(2), "EnableServiceAccess wasn't called")

			serviceName, planName, _ := fakeCFClient.EnableServiceAccessArgsForCall(0)
			Expect(serviceName).To(Equal(expectedServiceName))
			Expect(planName).To(Equal(expectedPlanName1))

			serviceName, planName2, _ := fakeCFClient.EnableServiceAccessArgsForCall(1)
			Expect(serviceName).To(Equal(expectedServiceName))
			Expect(planName2).To(Equal(expectedPlanName2))
		})

		It("disables service access for a plan that is set to disable access", func() {
			disabledPlanName := "disabled-plan"

			servicePlans = []config.PlanAccess{
				{
					Name:            disabledPlanName,
					CFServiceAccess: config.PlanDisabled,
				},
			}

			fakeCFClient.ServiceBrokersReturns([]cf.ServiceBroker{}, nil)
			runner = registrar.RegisterBrokerRunner{
				Config: config.RegisterBrokerErrandConfig{
					ServiceName: expectedServiceName,
					Plans:       servicePlans,
				},
				CFClient: fakeCFClient,
			}

			err := runner.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCFClient.DisableServiceAccessCallCount()).To(Equal(1), "DisableServiceAccess wasn't called")

			serviceName, planName, _ := fakeCFClient.DisableServiceAccessArgsForCall(0)
			Expect(serviceName).To(Equal(expectedServiceName))
			Expect(planName).To(Equal(disabledPlanName))
		})
	})

	Describe("error handling", func() {
		var (
			runner             registrar.RegisterBrokerRunner
			fakeCFClient       *fakes.FakeRegisterBrokerCFClient
			expectedBrokerName string
		)

		BeforeEach(func() {
			fakeCFClient = new(fakes.FakeRegisterBrokerCFClient)

			runner = registrar.RegisterBrokerRunner{
				Config: config.RegisterBrokerErrandConfig{
					BrokerName:  expectedBrokerName,
					ServiceName: "not-relevant",
					Plans: []config.PlanAccess{
						{
							Name:            "not-relevant",
							CFServiceAccess: config.PlanEnabled,
						}, {
							Name:            "not-relevant-but-different",
							CFServiceAccess: config.PlanDisabled,
						},
					},
				},
				CFClient: fakeCFClient,
			}
		})

		It("errors when it cannot retrieve list of service brokers", func() {
			fakeCFClient.ServiceBrokersReturns(nil, errors.New("failed to retrieve list of brokers"))

			err := runner.Run()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("failed to execute register-broker: failed to retrieve list of brokers"))
		})

		It("errors when it cannot create a service broker", func() {
			fakeCFClient.CreateServiceBrokerReturns(errors.New("failed to create broker"))

			err := runner.Run()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("failed to execute register-broker: failed to create broker"))
		})

		It("errors when it cannot enable access for a plan", func() {
			fakeCFClient.ServiceBrokersReturns([]cf.ServiceBroker{}, nil)
			fakeCFClient.EnableServiceAccessReturns(errors.New("I messed up"))

			err := runner.Run()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("failed to execute register-broker: I messed up"))
		})

		It("errors when it cannot disable access for a plan", func() {
			fakeCFClient.ServiceBrokersReturns([]cf.ServiceBroker{}, nil)
			fakeCFClient.DisableServiceAccessReturns(errors.New("I messed up"))

			err := runner.Run()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("failed to execute register-broker: I messed up"))
		})

		It("errors when it cannot update a service broker", func() {
			fakeCFClient.ServiceBrokersReturns([]cf.ServiceBroker{{GUID: "a-guid", Name: expectedBrokerName}}, nil)
			fakeCFClient.UpdateServiceBrokerReturns(errors.New("failed to update broker"))

			err := runner.Run()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("failed to execute register-broker: failed to update broker"))
		})
	})
})
