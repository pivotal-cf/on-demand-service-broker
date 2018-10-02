package service_test

import (
	"fmt"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/service/fakes"
)

var _ = Describe("CFServiceInstanceLister", func() {
	var (
		fakeCfClient *fakes.FakeListerClient
		fakeLogger   *log.Logger
	)

	BeforeEach(func() {
		fakeLogger = new(log.Logger)
		fakeCfClient = new(fakes.FakeListerClient)
	})

	It("queries CF for a list of service instances", func() {
		fakeCfClient.GetInstancesOfServiceOfferingReturns([]service.Instance{
			{GUID: "some-guid", PlanUniqueID: "some-plan"},
			{GUID: "some-other-guid", PlanUniqueID: "some-plan"},
			{GUID: "yet-another-guid", PlanUniqueID: "some-other-plan"},
		}, nil)

		l, err := service.BuildInstanceLister(fakeCfClient, "some-offering-id", config.ServiceInstancesAPI{}, fakeLogger)
		Expect(err).ToNot(HaveOccurred(), "unexpected error while building the lister")

		instances, err := l.Instances()

		Expect(err).ToNot(HaveOccurred())
		Expect(instances).To(ConsistOf(
			service.Instance{GUID: "some-guid", PlanUniqueID: "some-plan"},
			service.Instance{GUID: "some-other-guid", PlanUniqueID: "some-plan"},
			service.Instance{GUID: "yet-another-guid", PlanUniqueID: "some-other-plan"},
		))

		Expect(fakeCfClient.GetInstancesOfServiceOfferingCallCount()).To(Equal(1), "cf client wasn't called")
		serviceOffering, logger := fakeCfClient.GetInstancesOfServiceOfferingArgsForCall(0)
		Expect(serviceOffering).To(Equal("some-offering-id"))
		Expect(logger).To(Equal(fakeLogger))
	})

	It("errors when pulling the list of instances fails", func() {
		fakeLogger := new(log.Logger)
		fakeCfClient := new(fakes.FakeListerClient)
		fakeCfClient.GetInstancesOfServiceOfferingReturns(nil, fmt.Errorf("boom"))

		l, err := service.BuildInstanceLister(fakeCfClient, "some-offering-id", config.ServiceInstancesAPI{}, fakeLogger)
		Expect(err).ToNot(HaveOccurred(), "unexpected error while building the lister")

		_, err = l.Instances()
		Expect(err).To(MatchError("boom"))
	})
})
