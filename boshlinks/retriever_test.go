package boshlinks_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/boshlinks"
)

var _ = Describe("Boshlinks", func() {
	var (
		fakeBoshHTTP *fakes.FakeHTTP
		subject      boshdirector.DNSRetriever
	)

	BeforeEach(func() {
		fakeBoshHTTP = new(fakes.FakeHTTP)
		subject = boshlinks.NewDNSRetriever(fakeBoshHTTP)
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
			actualLinkId, err := subject.LinkProviderID(deploymentName, instanceGroupName, providerName)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLinkId).To(Equal(linkId))
		})

		It("returns a not found error when requested criteria not in the returned list", func() {
			deploymentName := "cf"
			instanceGroupName := "log-api"
			providerName := "not-there"
			fakeBoshHTTP.RawGetReturns(providerLinkListJSON, nil)
			_, err := subject.LinkProviderID(deploymentName, instanceGroupName, providerName)
			Expect(err).To(MatchError(ContainSubstring("could not find link provider matching")))
		})

		It("returns a not found error when requested criteria is in the list, but shared is not true", func() {
			deploymentName := "cf"
			instanceGroupName := "credhub"
			providerName := "private_link"
			fakeBoshHTTP.RawGetReturns(providerLinkListJSON, nil)
			_, err := subject.LinkProviderID(deploymentName, instanceGroupName, providerName)
			Expect(err).To(MatchError(ContainSubstring("could not find link provider matching")))
		})

		It("returns an error when RawGet errors", func() {
			deploymentName := "cf"
			instanceGroupName := "log-api"
			providerName := "reverse_log_proxy"
			fakeBoshHTTP.RawGetReturns(`{}`, errors.New("something went wrong"))
			_, err := subject.LinkProviderID(deploymentName, instanceGroupName, providerName)
			Expect(err).To(MatchError(ContainSubstring("HTTP GET on /link_providers?deployment=cf endpoint failed")))
		})

		It("returns an error when the response is invalid JSON", func() {
			deploymentName := "cf"
			instanceGroupName := "log-api"
			providerName := "reverse_log_proxy"
			fakeBoshHTTP.RawGetReturns(`{"a":"b"}`, nil)
			_, err := subject.LinkProviderID(deploymentName, instanceGroupName, providerName)
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
			actualLinkId, err := subject.CreateLinkConsumer(providerID)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLinkId).To(Equal("3808"))
		})

		It("returns an error when the response is invalid JSON", func() {
			fakeBoshHTTP.RawPostReturns(`[]`, nil)
			_, err := subject.CreateLinkConsumer("123")
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal create link consumer response")))
		})

		It("returns an error when RawPost errors", func() {
			fakeBoshHTTP.RawPostReturns(`{}`, errors.New("something failed"))
			_, err := subject.CreateLinkConsumer("123")
			Expect(err).To(MatchError(ContainSubstring("HTTP POST on /links endpoint failed")))
		})
	})

	Describe("DeleteLinkConsumer", func() {
		It("succeeds when the consumer link id exists", func() {
			fakeBoshHTTP.RawDeleteReturns("", nil)
			Expect(subject.DeleteLinkConsumer("3808")).To(Succeed())
		})

		It("returns an error when RawDelete errors", func() {
			errResponse := `{"code": 810000, "description": "invalid link id 123"}`
			fakeBoshHTTP.RawDeleteReturns(errResponse, errors.New("something failed"))
			err := subject.DeleteLinkConsumer("123")
			Expect(err).To(MatchError(ContainSubstring("HTTP DELETE on /links/:id endpoint failed: " + errResponse)))
		})
	})

	Describe("GetLinkAddress", func() {
		BeforeEach(func() {
			fakeBoshHTTP.RawGetReturns(`{
				"address": "q-s0.dummy.default.dep-with-link.bosh"
			}`, nil)
		})

		It("returns the address when bosh get call is successful", func() {
			azs := []string{"europe1"}
			consumerLinkID := "123"
			addr, err := subject.GetLinkAddress(consumerLinkID, azs, "healthy")
			Expect(err).NotTo(HaveOccurred())
			Expect(addr).To(Equal("q-s0.dummy.default.dep-with-link.bosh"))

			path := fakeBoshHTTP.RawGetArgsForCall(0)
			Expect(path).To(Equal("/link_address?link_id=123&azs[]=europe1&status=healthy"))
		})

		It("correctly supports multi-azs with special characters", func() {
			azs := []string{"europe1", "europe/2"}
			consumerLinkID := "123"
			_, err := subject.GetLinkAddress(consumerLinkID, azs, "")
			Expect(err).NotTo(HaveOccurred())

			path := fakeBoshHTTP.RawGetArgsForCall(0)
			Expect(path).To(Equal("/link_address?link_id=123&azs[]=europe1&azs[]=europe%2F2"))
		})

		It("calls the api without azs when no azs are supplied", func() {
			consumerLinkID := "123"
			_, err := subject.GetLinkAddress(consumerLinkID, []string{}, "")
			Expect(err).NotTo(HaveOccurred())

			path := fakeBoshHTTP.RawGetArgsForCall(0)
			Expect(path).To(Equal("/link_address?link_id=123"))
		})

		It("calls the api without status when no status is supplied", func() {
			consumerLinkID := "123"
			_, err := subject.GetLinkAddress(consumerLinkID, []string{}, "")
			Expect(err).NotTo(HaveOccurred())

			path := fakeBoshHTTP.RawGetArgsForCall(0)
			Expect(path).To(Equal("/link_address?link_id=123"))
		})

		It("returns an error if status option is invalid", func() {
			consumerLinkID := "123"
			_, err := subject.GetLinkAddress(consumerLinkID, []string{}, "invalid-option")

			Expect(err).To(MatchError(ContainSubstring("status must be one of the following options: <default | healthy | unhealthy | all>")))
		})

		It("returns an error when RawGet errors", func() {
			fakeBoshHTTP.RawGetReturns(`{}`, errors.New("something went wrong"))
			_, err := subject.GetLinkAddress("123", nil, "healthy")
			Expect(err).To(MatchError(ContainSubstring("HTTP GET on /link_address?link_id=123&status=healthy endpoint failed")))
		})

		It("returns an error when the response is not marshalable to obj", func() {
			fakeBoshHTTP.RawGetReturns(`[]`, nil)
			_, err := subject.GetLinkAddress("123", nil, "")
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal link address JSON")))
		})
	})
})
