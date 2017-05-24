package purger_test

import (
	"errors"
	"io"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/purger"
	"github.com/pivotal-cf/on-demand-service-broker/purger/fakes"
)

var _ = Describe("purger", func() {
	const (
		serviceOfferingGUID = "some-service-offering-guid"
		brokerName          = "some-broker-name"
	)

	var (
		fakeDeleter   *fakes.FakeDeleter
		fakeRegistrar *fakes.FakeDeregistrar
		fakeCFClient  *fakes.FakeCloudFoundryClient
		purgeTool     *purger.Purger
		logger        *log.Logger
		logBuffer     *gbytes.Buffer
	)
	BeforeEach(func() {
		logBuffer = gbytes.NewBuffer()
		logger = loggerfactory.
			New(io.MultiWriter(GinkgoWriter, logBuffer), "[purger-unit-tests] ", log.LstdFlags).
			NewWithRequestID()

		fakeDeleter = new(fakes.FakeDeleter)
		fakeRegistrar = new(fakes.FakeDeregistrar)
		fakeCFClient = new(fakes.FakeCloudFoundryClient)
		purgeTool = purger.New(fakeDeleter, fakeRegistrar, fakeCFClient, logger)

	})
	It("does not error when it deletes instances and deregisters the broker", func() {

		Expect(purgeTool.DeleteInstancesAndDeregister(serviceOfferingGUID, brokerName)).NotTo(HaveOccurred())

		Expect(logBuffer).To(gbytes.Say("Disabling service access for all plans"))
		Expect(fakeCFClient.DisableServiceAccessForServiceOfferingCallCount()).To(Equal(1))
		expectedServiceOfferingGUID, expectedLogger := fakeCFClient.DisableServiceAccessForServiceOfferingArgsForCall(0)
		Expect(expectedServiceOfferingGUID).To(Equal(serviceOfferingGUID))
		Expect(expectedLogger).To(Equal(logger))

		Expect(logBuffer).To(gbytes.Say("Deleting all service instances"))
		Expect(fakeDeleter.DeleteAllServiceInstancesCallCount()).To(Equal(1))
		Expect(fakeDeleter.DeleteAllServiceInstancesArgsForCall(0)).To(Equal(serviceOfferingGUID))

		Expect(logBuffer).To(gbytes.Say("Deregistering service broker"))
		Expect(fakeRegistrar.DeregisterCallCount()).To(Equal(1))
		Expect(fakeRegistrar.DeregisterArgsForCall(0)).To(Equal(brokerName))
	})

	It("returns an error when disabling the service access fails", func() {
		fakeCFClient.DisableServiceAccessForServiceOfferingReturns(errors.New("failed to disable service access"))
		Expect(purgeTool.DeleteInstancesAndDeregister(serviceOfferingGUID, brokerName)).To(MatchError("Purger Failed: failed to disable service access"))
	})

	It("returns an error when the deleter fails", func() {
		fakeDeleter.DeleteAllServiceInstancesReturns(errors.New("failed to delete stuff"))
		Expect(purgeTool.DeleteInstancesAndDeregister(serviceOfferingGUID, brokerName)).To(MatchError("Purger Failed: failed to delete stuff"))
	})

	It("returns an error when the deregistrar fails", func() {
		fakeRegistrar.DeregisterReturns(errors.New("failed to deregister"))
		Expect(purgeTool.DeleteInstancesAndDeregister(serviceOfferingGUID, brokerName)).To(MatchError("Purger Failed: failed to deregister"))
	})
})
