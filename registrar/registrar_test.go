package registrar_test

import (
	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/registrar"
	"github.com/pivotal-cf/on-demand-service-broker/registrar/fakes"

	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
)

var _ = Describe("Registrar", func() {
	const (
		brokerGUID = "broker-guid"
		brokerName = "broker-name"
	)

	var fakeCFClient *fakes.FakeCloudFoundryClient

	BeforeEach(func() {
		fakeCFClient = new(fakes.FakeCloudFoundryClient)

	})

	It("does not return an error when deregistering", func() {
		fakeCFClient.ListServiceBrokersReturns([]cf.ServiceBroker{
			{
				GUID: brokerGUID,
				Name: brokerName,
			},
		}, nil)

		registrar := registrar.New(fakeCFClient, nil)

		Expect(registrar.Deregister(brokerName)).NotTo(HaveOccurred())
		Expect(fakeCFClient.ListServiceBrokersCallCount()).To(Equal(1))
		Expect(fakeCFClient.DeregisterBrokerCallCount()).To(Equal(1))
		Expect(fakeCFClient.DeregisterBrokerArgsForCall(0)).To(Equal(brokerGUID))
	})

	It("returns an error when cf client fails to list service brokers", func() {
		fakeCFClient.ListServiceBrokersReturns([]cf.ServiceBroker{}, errors.New("list service broker failed"))

		registrar := registrar.New(fakeCFClient, nil)

		Expect(registrar.Deregister(brokerName)).To(MatchError("list service broker failed"))
	})

	It("returns an error when it cannot find the service broker", func() {
		fakeCFClient.ListServiceBrokersReturns([]cf.ServiceBroker{{
			GUID: "different-broker-guid",
			Name: "different-broker-name",
		}}, nil)

		registrar := registrar.New(fakeCFClient, nil)

		Expect(registrar.Deregister(brokerName)).To(MatchError(fmt.Sprintf("Failed to find broker with name: %s", brokerName)))
	})

	It("returns an error when cf client fails to deregister", func() {
		fakeCFClient.ListServiceBrokersReturns([]cf.ServiceBroker{
			{
				GUID: brokerGUID,
				Name: brokerName,
			},
		}, nil)
		fakeCFClient.DeregisterBrokerReturns(errors.New("failed"))

		registrar := registrar.New(fakeCFClient, nil)

		errMsg := fmt.Sprintf("Failed to deregister broker with %s with guid %s, err: failed", brokerName, brokerGUID)
		Expect(registrar.Deregister(brokerName)).To(MatchError(errMsg))
	})
})
