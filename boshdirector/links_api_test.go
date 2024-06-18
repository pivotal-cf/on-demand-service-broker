package boshdirector_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

var _ = Describe("LinksApi", func() {
	Describe("GetDNSAddresses", func() {
		var (
			fakeDNSRetriever *fakes.FakeDNSRetriever

			providerID     = "78"
			consumerID     = "3808"
			dopplerAddress = "doppler.dns.bosh"
			azs            = []string{"bob", "jim"}
			status         = "healthy"
		)

		BeforeEach(func() {
			fakeDNSRetriever = new(fakes.FakeDNSRetriever)
			fakeDNSRetrieverFactory.Returns(fakeDNSRetriever)
			fakeDNSRetriever.LinkProviderIDReturns(providerID, nil)
			fakeDNSRetriever.CreateLinkConsumerReturns(consumerID, nil)
			fakeDNSRetriever.GetLinkAddressReturns(dopplerAddress, nil)
		})

		It("returns a map of dns addresses", func() {
			boshDnsAddresses, err := c.GetDNSAddresses("cf", []config.BindingDNS{
				{
					Name:          "config-1",
					LinkProvider:  "linker",
					InstanceGroup: "doppler",
					Properties: config.BindingDNSProperties{
						AZS:    azs,
						Status: status,
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeDNSRetriever.LinkProviderIDCallCount()).To(Equal(1))
			Expect(fakeDNSRetriever.CreateLinkConsumerCallCount()).To(Equal(1))
			Expect(fakeDNSRetriever.GetLinkAddressCallCount()).To(Equal(1))
			Expect(fakeDNSRetriever.DeleteLinkConsumerCallCount()).To(Equal(1))

			deploymentName, instanceGroup, linkProvider := fakeDNSRetriever.LinkProviderIDArgsForCall(0)
			Expect(deploymentName).To(Equal("cf"))
			Expect(instanceGroup).To(Equal("doppler"))
			Expect(linkProvider).To(Equal("linker"))

			Expect(fakeDNSRetriever.CreateLinkConsumerArgsForCall(0)).To(Equal(providerID))
			actualConsumerID, actualAzs, actualStatus := fakeDNSRetriever.GetLinkAddressArgsForCall(0)
			Expect(actualConsumerID).To(Equal(consumerID))
			Expect(actualAzs).To(Equal(azs))
			Expect(actualStatus).To(Equal(status))

			Expect(fakeDNSRetriever.DeleteLinkConsumerArgsForCall(0)).To(Equal(consumerID))

			Expect(boshDnsAddresses).To(Equal(map[string]string{"config-1": dopplerAddress}))
		})

		It("errors when requesting the provider id errors", func() {
			fakeDNSRetriever.LinkProviderIDReturns("", errors.New("boom"))

			_, err := c.GetDNSAddresses("cf", []config.BindingDNS{{Name: "config-1", LinkProvider: "doppler", InstanceGroup: "doppler"}})
			Expect(err).To(MatchError(ContainSubstring("boom")))
		})

		It("errors when creating the consumer errors", func() {
			fakeDNSRetriever.CreateLinkConsumerReturns("", errors.New("pow"))

			_, err := c.GetDNSAddresses("cf", []config.BindingDNS{{Name: "config-1", LinkProvider: "doppler", InstanceGroup: "doppler"}})
			Expect(err).To(MatchError(ContainSubstring("pow")))
		})

		It("errors when requesting the link address errors", func() {
			fakeDNSRetriever.GetLinkAddressReturns("", errors.New("smash"))

			_, err := c.GetDNSAddresses("cf", []config.BindingDNS{{Name: "config-1", LinkProvider: "doppler", InstanceGroup: "doppler"}})
			Expect(err).To(MatchError(ContainSubstring("smash")))
		})

		It("ignores errors when deleting the link fails", func() {
			fakeDNSRetriever.DeleteLinkConsumerReturns(errors.New("kaboom"))

			_, err := c.GetDNSAddresses("cf", []config.BindingDNS{{Name: "config-1", LinkProvider: "doppler", InstanceGroup: "doppler"}})
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
