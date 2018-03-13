package upgrader_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader/fakes"
)

var _ = Describe("State checker", func() {
	var (
		guid                  string
		expectedOperationData broker.OperationData
		fakeBrokerService     *fakes.FakeBrokerServices
		stateChecker          upgrader.StateChecker
	)

	BeforeEach(func() {
		guid = "some-guid"
		expectedOperationData = broker.OperationData{BoshTaskID: 123}
		fakeBrokerService = new(fakes.FakeBrokerServices)
		stateChecker = upgrader.NewStateChecker(fakeBrokerService)
	})

	It("returns UpgradeSucceeded when last operation reports success", func() {
		fakeBrokerService.LastOperationReturns(brokerapi.LastOperation{State: brokerapi.Succeeded}, nil)

		state, err := stateChecker.CheckState(guid, expectedOperationData)
		Expect(err).NotTo(HaveOccurred())

		By("pulling the last operation with the right arguments")
		Expect(fakeBrokerService.LastOperationCallCount()).To(Equal(1))
		guidArg, operationData := fakeBrokerService.LastOperationArgsForCall(0)
		Expect(guidArg).To(Equal(guid))
		Expect(operationData).To(Equal(expectedOperationData))

		Expect(state).To(Equal(services.UpgradeOperation{Type: services.UpgradeSucceeded, Data: expectedOperationData}))
	})

	It("returns an error if it fails to pull last operation", func() {
		fakeBrokerService.LastOperationReturns(brokerapi.LastOperation{}, errors.New("oops"))

		_, err := stateChecker.CheckState(guid, expectedOperationData)
		Expect(err).To(MatchError("error getting last operation: oops"))
	})

	It("returns UpgradeFailed when last operation reports failure", func() {
		fakeBrokerService.LastOperationReturns(brokerapi.LastOperation{State: brokerapi.Failed}, nil)

		state, err := stateChecker.CheckState(guid, expectedOperationData)
		Expect(err).NotTo(HaveOccurred())

		Expect(state).To(Equal(services.UpgradeOperation{Type: services.UpgradeFailed, Data: expectedOperationData}))
	})

	It("returns UpgradeInProgress when last operation reports the upgrade is in progress", func() {
		fakeBrokerService.LastOperationReturns(brokerapi.LastOperation{State: brokerapi.InProgress}, nil)

		state, err := stateChecker.CheckState(guid, expectedOperationData)
		Expect(err).NotTo(HaveOccurred())

		Expect(state).To(Equal(services.UpgradeOperation{Type: services.UpgradeAccepted, Data: expectedOperationData}))
	})

	It("returns an error if last operation returns an unknown state", func() {
		fakeBrokerService.LastOperationReturns(brokerapi.LastOperation{State: "not-a-state"}, nil)

		_, err := stateChecker.CheckState(guid, expectedOperationData)
		Expect(err).To(MatchError("uknown state from last operation: not-a-state"))
	})
})
