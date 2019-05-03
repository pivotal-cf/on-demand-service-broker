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
	var (
		runner           registrar.RegisterBrokerRunner
		fakeCFClient     *fakes.FakeRegisterBrokerCFClient
		expectedUsername string
		expectedPassword string
		expectedURL      string
		expectedName     string
	)

	BeforeEach(func() {
		expectedName = "brokername"
		expectedUsername = "brokeruser"
		expectedPassword = "brokerpass"
		expectedURL = "http://broker.url.example.com"

		fakeCFClient = new(fakes.FakeRegisterBrokerCFClient)

		runner = registrar.RegisterBrokerRunner{
			Config: config.RegisterBrokerErrandConfig{
				BrokerName:     expectedName,
				BrokerUsername: expectedUsername,
				BrokerPassword: expectedPassword,
				BrokerURL:      expectedURL,
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

	Describe("error handling", func() {
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

		It("errors when it cannot update a service broker", func() {
			fakeCFClient.ServiceBrokersReturns([]cf.ServiceBroker{{GUID: "a-guid", Name: expectedName}}, nil)
			fakeCFClient.UpdateServiceBrokerReturns(errors.New("failed to update broker"))

			err := runner.Run()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("failed to execute register-broker: failed to update broker"))
		})
	})
})
