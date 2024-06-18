package uaa_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"

	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/uaa"
)

var _ = Describe("UAA Contract Tests", func() {
	It("controls the lifecycle of the UAA client", func() {
		var client *uaa.Client
		clientID := uuid.New()[:8]

		By("successfully connecting to UAA", func() {
			var err error
			cfCert := os.Getenv("CF_CA_CERT")
			client, err = uaa.New(config.UAAConfig{
				URL: "https://uaa." + os.Getenv("BROKER_SYSTEM_DOMAIN"),
				ClientDefinition: config.ClientDefinition{
					Authorities:          "cloud_controller.admin",
					AuthorizedGrantTypes: "implicit",
					ResourceIDs:          "resource_1",
					Scopes:               "some-scope",
				},
				Authentication: config.UAACredentials{
					ClientCredentials: config.ClientCredentials{
						ID:     os.Getenv("CF_CLIENT_ID"),
						Secret: os.Getenv("CF_CLIENT_SECRET"),
					},
				},
			}, cfCert, false)
			Expect(err).NotTo(HaveOccurred())
		})

		By("successfully creating a client on uaa", func() {
			createdClient, err := client.CreateClient(clientID, "some-name", "some-space-guid")
			Expect(err).NotTo(HaveOccurred())
			Expect(createdClient).NotTo(BeNil())
		})

		By("successfully retrieving the client from uaa", func() {
			existingClient, err := client.GetClient(clientID)
			Expect(err).NotTo(HaveOccurred())
			Expect(existingClient["client_id"]).To(Equal(clientID))
			Expect(existingClient["redirect_uri"]).To(Equal("https://placeholder.example.com"))
			Expect(existingClient["authorized_grant_types"]).To(Equal("implicit"))
		})

		By("successfully updating the client", func() {
			_, err := client.UpdateClient(clientID, "https://new-placeholder.example.com", "some-space-guid")
			Expect(err).NotTo(HaveOccurred())
			updatedClient, err := client.GetClient(clientID)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedClient["redirect_uri"]).To(Equal("https://new-placeholder.example.com"))
		})

		By("successfully removing the client", func() {
			err := client.DeleteClient(clientID)
			Expect(err).NotTo(HaveOccurred())
			updatedClient, err := client.GetClient(clientID)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedClient).To(BeNil())
		})
	})
})
