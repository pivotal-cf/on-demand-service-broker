package boshdirector_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

var _ = Describe("LinksApi", func() {
	var (
		fakeBoshHTTP *fakes.FakeHTTP
	)

	BeforeEach(func() {
		fakeBoshHTTP = new(fakes.FakeHTTP)
		fakeBoshHTTPFactory.Returns(fakeBoshHTTP)
	})

	Describe("GetLinkProviderId", func() {
		var providerLinkListJSON = `
		[
		{
			"id": "77",
			"name": "doppler",
			"shared": true,
			"deployment": "cf",
			"link_provider_definition": {
				"type": "doppler",
				"name": "doppler"
			},
			"owner_object": {
				"type": "job",
				"name": "doppler",
				"info": {
					"instance_group": "doppler"
				}
			}
		},
		{
			"id": "85",
			"name": "reverse_log_proxy",
			"shared": true,
			"deployment": "cf",
			"link_provider_definition": {
				"type": "reverse_log_proxy",
				"name": "reverse_log_proxy"
			},
			"owner_object": {
				"type": "job",
				"name": "reverse_log_proxy",
				"info": {
					"instance_group": "log-api"
				}
			}
		},
		{
			"id": "89",
			"name": "credhub",
			"shared": true,
			"deployment": "cf",
			"link_provider_definition": {
				"type": "credhub",
				"name": "credhub"
			},
			"owner_object": {
				"type": "job",
				"name": "credhub",
				"info": {
					"instance_group": "credhub"
				}
			}
		},
		{
			"id": "90",
			"name": "private_link",
			"shared": false,
			"deployment": "cf",
			"link_provider_definition": {
				"type": "credhub",
				"name": "credhub"
			},
			"owner_object": {
				"type": "job",
				"name": "credhub",
				"info": {
					"instance_group": "credhub"
				}
			}
		}
		]
		`
		It("returns the correct id when it receives a list of link providers containing the desired link", func() {
			deploymentName := "cf"
			instanceGroupName := "log-api"
			providerName := "reverse_log_proxy"
			linkId := "85"
			fakeBoshHTTP.RawGetReturns(providerLinkListJSON, nil)
			actualLinkId, err := c.LinkProviderID(deploymentName, instanceGroupName, providerName)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLinkId).To(Equal(linkId))
		})

		It("returns a not found error when requested criteria not in the returned list", func() {
			deploymentName := "cf"
			instanceGroupName := "log-api"
			providerName := "not-there"
			fakeBoshHTTP.RawGetReturns(providerLinkListJSON, nil)
			_, err := c.LinkProviderID(deploymentName, instanceGroupName, providerName)
			Expect(err).To(MatchError(ContainSubstring("could not find link provider matching")))
		})

		It("returns a not found error when requested criteria is in the list, but shared is not true", func() {
			deploymentName := "cf"
			instanceGroupName := "credhub"
			providerName := "private_link"
			fakeBoshHTTP.RawGetReturns(providerLinkListJSON, nil)
			_, err := c.LinkProviderID(deploymentName, instanceGroupName, providerName)
			Expect(err).To(MatchError(ContainSubstring("could not find link provider matching")))
		})

		It("returns an error when RawGet errors", func() {
			deploymentName := "cf"
			instanceGroupName := "log-api"
			providerName := "reverse_log_proxy"
			fakeBoshHTTP.RawGetReturns(`{}`, errors.New("something went wrong"))
			_, err := c.LinkProviderID(deploymentName, instanceGroupName, providerName)
			Expect(err).To(MatchError(ContainSubstring("HTTP GET on /link_providers endpoint failed")))
		})

		It("returns an error when the response is invalid JSON", func() {
			deploymentName := "cf"
			instanceGroupName := "log-api"
			providerName := "reverse_log_proxy"
			fakeBoshHTTP.RawGetReturns(`{"a":"b"}`, nil)
			_, err := c.LinkProviderID(deploymentName, instanceGroupName, providerName)
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal links provider JSON")))
		})
	})

	Describe("CreateLinkConsumer", func() {
		It("returns the consumer ID when provided with a valid provider ID", func() {
			consumerJSON := `{
				"id": "3808",
				"name": "dummy_with_link",
				"link_consumer_id": "3630",
				"link_provider_id": "2077",
				"created_at": "2018-07-03 15:53:15 UTC"
			}`

			fakeBoshHTTP.RawPostReturns(consumerJSON, nil)
			providerID := "2077"
			actualLinkId, err := c.CreateLinkConsumer(providerID)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLinkId).To(Equal("3808"))
		})

		It("returns an error when the response is invalid JSON", func() {
			fakeBoshHTTP.RawPostReturns(`[]`, nil)
			_, err := c.CreateLinkConsumer("123")
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal create link consumer response")))
		})

		It("returns an error when RawPost errors", func() {
			fakeBoshHTTP.RawPostReturns(`{}`, errors.New("something failed"))
			_, err := c.CreateLinkConsumer("123")
			Expect(err).To(MatchError(ContainSubstring("HTTP POST on /links endpoint failed")))
		})
	})

	Describe("DeleteLinkConsumer", func() {
		It("succeeds when the consumer link id exists", func() {
			fakeBoshHTTP.RawDeleteReturns("", nil)
			Expect(c.DeleteLinkConsumer("3808")).To(Succeed())
		})

		It("returns an error when RawDelete errors", func() {
			errResponse := `{"code": 810000, "description": "invalid link id 123"}`
			fakeBoshHTTP.RawDeleteReturns(errResponse, errors.New("something failed"))
			err := c.DeleteLinkConsumer("123")
			Expect(err).To(MatchError(ContainSubstring("HTTP DELETE on /links/:id endpoint failed: " + errResponse)))
		})
	})

	Describe("GetLinkAddress", func() {
		It("returns the address when bosh get call is successful", func() {
			fakeBoshHTTP.RawGetReturns(`{
				"address": "q-s0.dummy.default.dep-with-link.bosh"
			}`, nil)
			consumerLinkID := "123"
			addr, err := c.GetLinkAddress(consumerLinkID)
			Expect(err).NotTo(HaveOccurred())
			Expect(addr).To(Equal("q-s0.dummy.default.dep-with-link.bosh"))
		})

		It("returns an error when RawGet errors", func() {
			fakeBoshHTTP.RawGetReturns(`{}`, errors.New("something went wrong"))
			_, err := c.GetLinkAddress("123")
			Expect(err).To(MatchError(ContainSubstring("HTTP GET on /link_address endpoint failed")))
		})

		It("returns an error when the response is not marshalable to obj", func() {
			fakeBoshHTTP.RawGetReturns(`[]`, nil)
			_, err := c.GetLinkAddress("123")
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal link address JSON")))
		})
	})

	Describe("GetDNSAddresses", func() {
		var (
			providersListJson string = `[{
				"id": "77",
				"name": "doppler",
				"shared": true,
				"owner_object": {
					"info": {
						"instance_group": "doppler"
					}
				}
			}, {
				"id": "85",
				"name": "reverse_log_proxy",
				"shared": true,
				"owner_object": {
					"info": {
						"instance_group": "log-api"
					}
				}
			}]`

			consumerJSON = `{ "id": "3808" }`

			dopplerAddress = `{"address":"doppler.dns.bosh"}`
		)

		It("returns a map of dns addresses", func() {
			fakeBoshHTTP.RawGetReturnsOnCall(0, providersListJson, nil)
			fakeBoshHTTP.RawPostReturns(consumerJSON, nil)
			fakeBoshHTTP.RawGetReturnsOnCall(1, dopplerAddress, nil)

			boshDnsAddresses, err := c.GetDNSAddresses("cf", []config.BindingDNS{{Name: "config-1", LinkProvider: "doppler", InstanceGroup: "doppler"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeBoshHTTP.RawDeleteCallCount()).To(Equal(1))

			Expect(boshDnsAddresses).To(Equal(map[string]string{"config-1": "doppler.dns.bosh"}))
		})

		It("errors when requesting the provider id errors", func() {
			fakeBoshHTTP.RawGetReturnsOnCall(0, "", errors.New("failed"))
			fakeBoshHTTP.RawGetReturnsOnCall(1, dopplerAddress, nil)

			_, err := c.GetDNSAddresses("cf", []config.BindingDNS{{Name: "config-1", LinkProvider: "doppler", InstanceGroup: "doppler"}})
			Expect(err).To(MatchError(ContainSubstring("HTTP GET on /link_providers endpoint failed")))
		})

		It("errors when creating the consumer errors", func() {
			fakeBoshHTTP.RawGetReturnsOnCall(0, providersListJson, nil)
			fakeBoshHTTP.RawPostReturns(consumerJSON, errors.New("boom"))

			_, err := c.GetDNSAddresses("cf", []config.BindingDNS{{Name: "config-1", LinkProvider: "doppler", InstanceGroup: "doppler"}})
			Expect(err).To(MatchError(ContainSubstring("HTTP POST on /links endpoint failed")))
		})

		It("errors when requesting the link address errors", func() {
			fakeBoshHTTP.RawGetReturnsOnCall(0, providersListJson, nil)
			fakeBoshHTTP.RawPostReturns(consumerJSON, nil)
			fakeBoshHTTP.RawGetReturnsOnCall(1, "", errors.New("kaboom"))

			_, err := c.GetDNSAddresses("cf", []config.BindingDNS{{Name: "config-1", LinkProvider: "doppler", InstanceGroup: "doppler"}})
			Expect(err).To(MatchError(ContainSubstring("HTTP GET on /link_address endpoint failed")))
		})

		It("ignores errors when deleting the link fails", func() {
			fakeBoshHTTP.RawGetReturnsOnCall(0, providersListJson, nil)
			fakeBoshHTTP.RawPostReturns(consumerJSON, nil)
			fakeBoshHTTP.RawGetReturnsOnCall(1, dopplerAddress, nil)
			fakeBoshHTTP.RawDeleteReturns("", errors.New("oooh"))

			_, err := c.GetDNSAddresses("cf", []config.BindingDNS{{Name: "config-1", LinkProvider: "doppler", InstanceGroup: "doppler"}})
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
