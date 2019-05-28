package service_test

import (
	"errors"
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

	Describe("Instances", func() {
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

	Describe("FilteredInstances", func() {
		var subject service.InstanceLister

		BeforeEach(func() {
			var err error
			subject, err = service.BuildInstanceLister(fakeCfClient, "some-offering-id", config.ServiceInstancesAPI{}, fakeLogger)
			Expect(err).ToNot(HaveOccurred(), "unexpected error while building the lister")
			Expect(subject).To(BeAssignableToTypeOf(&service.CFServiceInstanceLister{}))
		})

		It("can filter instances by org and space", func() {
			fakeCfClient.GetInstancesOfServiceOfferingByOrgSpaceReturns([]service.Instance{
				{GUID: "some-guid", PlanUniqueID: "some-plan"},
				{GUID: "some-other-guid", PlanUniqueID: "some-plan"},
			}, nil)

			instances, err := subject.FilteredInstances(map[string]string{"cf_org": "some-org", "cf_space": "some-space"})

			Expect(err).ToNot(HaveOccurred())
			Expect(instances).To(ConsistOf(
				service.Instance{GUID: "some-guid", PlanUniqueID: "some-plan"},
				service.Instance{GUID: "some-other-guid", PlanUniqueID: "some-plan"},
			))

			Expect(fakeCfClient.GetInstancesOfServiceOfferingByOrgSpaceCallCount()).To(Equal(1), "cf client wasn't called")
			serviceOffering, orgName, spaceName, logger := fakeCfClient.GetInstancesOfServiceOfferingByOrgSpaceArgsForCall(0)
			Expect(serviceOffering).To(Equal("some-offering-id"))
			Expect(orgName).To(Equal("some-org"))
			Expect(spaceName).To(Equal("some-space"))
			Expect(logger).To(Equal(fakeLogger))
		})

		It("fails when unsupported filters are passed", func() {
			_, err := subject.FilteredInstances(map[string]string{"cf_org": "org", "some-org": "some-org", "cf_space": "some-space", "unknown-key": "what"})

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(SatisfyAll(
				ContainSubstring(`unsupported filters`),
				ContainSubstring(`supported filters are: cf_org, cf_space`)),
			))
		})

		It("fails when org is passed, but not space", func() {
			_, err := subject.FilteredInstances(map[string]string{"cf_org": "some-org"})

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring(`missing required filter cf_space`)))
		})

		It("fails when space is passed, but not org", func() {
			_, err := subject.FilteredInstances(map[string]string{"cf_space": "some-space"})

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring(`missing required filter cf_org`)))
		})

		It("fails when it cannot talk to CF", func() {
			fakeCfClient.GetInstancesOfServiceOfferingByOrgSpaceReturns(nil, errors.New("some error"))

			_, err := subject.FilteredInstances(map[string]string{"cf_org": "some-org", "cf_space": "some-space"})

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("could not retrieve list of instances: some error")))
		})
	})
})
