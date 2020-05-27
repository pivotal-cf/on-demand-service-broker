package uaa_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/uaa"
)

var _ = Describe("UAA", func() {
	Describe("Client", func() {
		var (
			uaaClient *uaa.Client
			uaaConfig config.UAAConfig
		)

		BeforeEach(func() {
			uaaConfig = config.UAAConfig{
				ClientDefinition: config.ClientDefinition{
					Authorities:          "some-authority,another-authority",
					AuthorizedGrantTypes: "client_credentials,password",
					ResourceIDs:          "resource1,resource2",
					Scopes:               "admin,read,write",
				},
			}
			uaaClient = uaa.New(uaaConfig)
		})

		Describe("#CreateClient", func() {
			It("returns a client object", func() {
				actualClient, _ := uaaClient.CreateClient("some-client-id", "some-name")

				By("generating some properties", func() {
					Expect(actualClient["client_id"]).To(Equal("some-client-id"))
					Expect(actualClient["client_secret"]).NotTo(BeEmpty())
					Expect(actualClient["name"]).To(Equal("some-name"))
				})

				By("using the configured properties", func() {
					Expect(actualClient["scopes"]).To(Equal(uaaConfig.ClientDefinition.Scopes))
					Expect(actualClient["resource_ids"]).To(Equal(uaaConfig.ClientDefinition.ResourceIDs))
					Expect(actualClient["authorities"]).To(Equal(uaaConfig.ClientDefinition.Authorities))
					Expect(actualClient["authorized_grant_types"]).To(Equal(uaaConfig.ClientDefinition.AuthorizedGrantTypes))
				})
			})

			It("generates a new password every time it is called", func() {
				c1, _ := uaaClient.CreateClient("foo", "foo")
				c2, _ := uaaClient.CreateClient("foo", "foo")

				Expect(c1["client_secret"]).NotTo(Equal(c2["client_secret"]))
			})

			It("generates unique but reproducible ids and names", func() {
				c1, _ := uaaClient.CreateClient("client1", "name1")
				c2, _ := uaaClient.CreateClient("client2", "name2")

				Expect(c1["client_id"]).NotTo(Equal(c2["client_id"]))
				Expect(c1["name"]).NotTo(Equal(c2["name"]))
			})

			It("does not generate a name if not passed", func() {
				c1, _ := uaaClient.CreateClient("client1", "")
				Expect(c1).NotTo(HaveKey("name"))
			})
		})
	})
})
