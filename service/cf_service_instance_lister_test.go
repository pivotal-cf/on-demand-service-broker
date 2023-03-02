package service_test

import (
	"errors"
	"fmt"
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/cf"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/service/fakes"
)

var _ = Describe("CFServiceInstanceLister", func() {
	var (
		fakeCfClient *fakes.FakeCFListerClient
		fakeLogger   *log.Logger
	)

	BeforeEach(func() {
		fakeLogger = new(log.Logger)
		fakeCfClient = new(fakes.FakeCFListerClient)
	})

	Describe("Instances", func() {
		It("queries CF for a list of service instances", func() {
			fakeCfClient.GetServiceInstancesReturns([]cf.Instance{
				{GUID: "some-guid", PlanUniqueID: "some-plan", SpaceGUID: "space_id"},
				{GUID: "some-other-guid", PlanUniqueID: "some-plan", SpaceGUID: "space_id"},
				{GUID: "yet-another-guid", PlanUniqueID: "some-other-plan", SpaceGUID: "space_id"},
			}, nil)

			l, err := service.BuildInstanceLister(fakeCfClient, "some-offering-id", config.ServiceInstancesAPI{}, fakeLogger)
			Expect(err).ToNot(HaveOccurred(), "unexpected error while building the lister")

			instances, err := l.Instances(nil)

			Expect(err).ToNot(HaveOccurred())
			Expect(instances).To(ConsistOf(
				service.Instance{GUID: "some-guid", PlanUniqueID: "some-plan", SpaceGUID: "space_id"},
				service.Instance{GUID: "some-other-guid", PlanUniqueID: "some-plan", SpaceGUID: "space_id"},
				service.Instance{GUID: "yet-another-guid", PlanUniqueID: "some-other-plan", SpaceGUID: "space_id"},
			))

			Expect(fakeCfClient.GetServiceInstancesCallCount()).To(Equal(1), "cf client wasn't called")
			filters, logger := fakeCfClient.GetServiceInstancesArgsForCall(0)
			Expect(filters.ServiceOfferingID).To(Equal("some-offering-id"))
			Expect(logger).To(Equal(fakeLogger))
		})

		It("errors when pulling the list of instances fails", func() {
			fakeLogger := new(log.Logger)
			fakeCfClient := new(fakes.FakeCFListerClient)
			fakeCfClient.GetServiceInstancesReturns(nil, fmt.Errorf("boom"))

			l, err := service.BuildInstanceLister(fakeCfClient, "some-offering-id", config.ServiceInstancesAPI{}, fakeLogger)
			Expect(err).ToNot(HaveOccurred(), "unexpected error while building the lister")

			_, err = l.Instances(nil)
			Expect(err).To(MatchError(ContainSubstring("boom")))
		})
	})

	Describe("Instances Filtering", func() {
		var subject service.InstanceLister

		BeforeEach(func() {
			var err error
			subject, err = service.BuildInstanceLister(fakeCfClient, "some-offering-id", config.ServiceInstancesAPI{}, fakeLogger)
			Expect(err).ToNot(HaveOccurred(), "unexpected error while building the lister")
			Expect(subject).To(BeAssignableToTypeOf(&service.CFServiceInstanceLister{}))
		})

		It("can filter instances by org and space", func() {
			fakeCfClient.GetServiceInstancesReturns([]cf.Instance{
				{GUID: "some-guid", PlanUniqueID: "some-plan"},
				{GUID: "some-other-guid", PlanUniqueID: "some-plan"},
			}, nil)

			instances, err := subject.Instances(map[string]string{"cf_org": "some-org", "cf_space": "some-space"})

			Expect(err).ToNot(HaveOccurred())
			Expect(instances).To(ConsistOf(
				service.Instance{GUID: "some-guid", PlanUniqueID: "some-plan"},
				service.Instance{GUID: "some-other-guid", PlanUniqueID: "some-plan"},
			))

			Expect(fakeCfClient.GetServiceInstancesCallCount()).To(Equal(1), "cf client wasn't called")
			instancesFilter, logger := fakeCfClient.GetServiceInstancesArgsForCall(0)
			Expect(instancesFilter.ServiceOfferingID).To(Equal("some-offering-id"))
			Expect(instancesFilter.OrgName).To(Equal("some-org"))
			Expect(instancesFilter.SpaceName).To(Equal("some-space"))
			Expect(logger).To(Equal(fakeLogger))
		})

		It("fails when unsupported filters are passed", func() {
			_, err := subject.Instances(map[string]string{"cf_org": "org", "some-org": "some-org", "cf_space": "some-space", "unknown-key": "what"})

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(SatisfyAll(
				ContainSubstring(`unsupported filters`),
				ContainSubstring(`supported filters are: cf_org, cf_space`)),
			))
		})

		It("fails when org is passed, but not space", func() {
			_, err := subject.Instances(map[string]string{"cf_org": "some-org"})

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring(`missing required filter cf_space`)))
		})

		It("fails when space is passed, but not org", func() {
			_, err := subject.Instances(map[string]string{"cf_space": "some-space"})

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring(`missing required filter cf_org`)))
		})

		It("fails when it cannot talk to CF", func() {
			fakeCfClient.GetServiceInstancesReturns(nil, errors.New("some error"))

			_, err := subject.Instances(map[string]string{"cf_org": "some-org", "cf_space": "some-space"})

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("could not retrieve list of instances: some error")))
		})
	})
})
