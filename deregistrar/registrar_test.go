package deregistrar_test

import (
	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/deregistrar"
	"github.com/pivotal-cf/on-demand-service-broker/deregistrar/fakes"

	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deregistrar", func() {
	const (
		brokerGUID = "broker-guid"
		brokerName = "broker-name"
	)

	var fakeCFClient *fakes.FakeCloudFoundryClient

	BeforeEach(func() {
		fakeCFClient = new(fakes.FakeCloudFoundryClient)

	})

	It("does not return an error when deregistering", func() {
		fakeCFClient.GetServiceOfferingGUIDReturns(brokerGUID, nil)

		registrar := deregistrar.New(fakeCFClient, nil)

		Expect(registrar.Deregister(brokerName)).NotTo(HaveOccurred())
		Expect(fakeCFClient.GetServiceOfferingGUIDCallCount()).To(Equal(1))
		Expect(fakeCFClient.DeregisterBrokerCallCount()).To(Equal(1))
		Expect(fakeCFClient.DeregisterBrokerArgsForCall(0)).To(Equal(brokerGUID))
	})

	It("returns an error when cf client fails to ge the service offering guid", func() {
		fakeCFClient.GetServiceOfferingGUIDReturns("", errors.New("list service broker failed"))

		registrar := deregistrar.New(fakeCFClient, nil)

		Expect(registrar.Deregister(brokerName)).To(MatchError("list service broker failed"))
	})

	It("returns an error when cf client fails to deregister", func() {
		fakeCFClient.GetServiceOfferingGUIDReturns(brokerGUID, nil)
		fakeCFClient.DeregisterBrokerReturns(errors.New("failed"))

		registrar := deregistrar.New(fakeCFClient, nil)

		errMsg := fmt.Sprintf("Failed to deregister broker with %s with guid %s, err: failed", brokerName, brokerGUID)
		Expect(registrar.Deregister(brokerName)).To(MatchError(errMsg))
	})
})
