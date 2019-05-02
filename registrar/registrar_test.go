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

		fakeCFClient.ServiceBrokersReturns([]cf.ServiceBroker{}, nil)
	})

	It("creates a broker in CF when the broker does not already exist in CF", func() {
		err := runner.Run()
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeCFClient.CreateServiceBrokerCallCount()).To(Equal(1), "create service broker wasn't called")

		actualName, actualUsername, actualPassword, actualURL := fakeCFClient.CreateServiceBrokerArgsForCall(0)
		Expect(actualName).To(Equal(expectedName))
		Expect(actualUsername).To(Equal(expectedUsername))
		Expect(actualPassword).To(Equal(expectedPassword))
		Expect(actualURL).To(Equal(expectedURL))
	})

	It("errors when it cannot retrieve list of service brokers", func() {
		fakeCFClient.ServiceBrokersReturns(nil, errors.New("failed"))

		err := runner.Run()
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("failed to retrieve list of service brokers: failed"))
	})

	It("errors when it cannot create a service broker", func() {
		fakeCFClient.CreateServiceBrokerReturns(errors.New("failed to create"))

		err := runner.Run()
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("failed to create service broker: failed to create"))
	})
})
