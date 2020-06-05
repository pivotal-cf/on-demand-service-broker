package uaa_test

import (
	"encoding/json"
	"encoding/pem"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/integration_tests/helpers"
	"github.com/pivotal-cf/on-demand-service-broker/uaa"
	"net/http"
	"regexp"
)

var _ = Describe("UAA", func() {
	Describe("Client", func() {
		var (
			server      *ghttp.Server
			uaaClient   *uaa.Client
			uaaConfig   config.UAAConfig
			trustedCert string
		)

		BeforeEach(func() {
			server = ghttp.NewTLSServer()
			rawPem := server.HTTPTestServer.Certificate().Raw
			pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rawPem})
			trustedCert = string(pemCert)

			uaaConfig = config.UAAConfig{
				URL: server.URL(),
				Authentication: config.UAACredentials{
					ClientCredentials: config.ClientCredentials{
						ID:     "authentication_id",
						Secret: "authentication_secret",
					},
				},
				ClientDefinition: config.ClientDefinition{
					Authorities:          "some-authority,another-authority",
					AuthorizedGrantTypes: "client_credentials,password",
					ResourceIDs:          "resource1,resource2",
					Scopes:               "admin,read,write",
				},
			}
			uaaClient, _ = uaa.New(uaaConfig, trustedCert)

			setupUAARoutes(server, uaaConfig)
		})

		AfterEach(func() {
			server.Close()
		})

		Describe("Constructor", func() {
			It("returns a new client", func() {
				uaaClient, err := uaa.New(uaaConfig, trustedCert)
				Expect(err).ToNot(HaveOccurred())
				Expect(uaaClient).NotTo(BeNil())
			})

			It("is created with a default random function", func() {
				uaaClient, err := uaa.New(uaaConfig, trustedCert)
				Expect(err).ToNot(HaveOccurred())
				Expect(uaaClient.RandFunc).NotTo(BeNil())
				Expect(uaaClient.RandFunc()).NotTo(Equal(uaaClient.RandFunc()))
			})

			It("returns an error when cannot construct the underlying go-uaa client", func() {
				uaaConfig.URL = ""
				_, err := uaa.New(uaaConfig, trustedCert)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("the target is missing"))
			})

			When("no client credentials are passed", func() {
				It("is created with a noop underlying client", func() {
					uaaConfig = config.UAAConfig{
						URL: server.URL(),
						ClientDefinition: config.ClientDefinition{
							Authorities:          "some",
							AuthorizedGrantTypes: "another",
						},
					}

					uaaClient, err := uaa.New(uaaConfig, trustedCert)
					Expect(err).ToNot(HaveOccurred())
					Expect(uaaClient).NotTo(BeNil())

					c, err := uaaClient.CreateClient("foo", "bar")
					Expect(err).NotTo(HaveOccurred())
					Expect(c).To(BeNil())

					c, err = uaaClient.UpdateClient("foo", "bar")
					Expect(err).NotTo(HaveOccurred())
					Expect(c).To(BeNil())

					err = uaaClient.DeleteClient("foo")
					Expect(err).NotTo(HaveOccurred())

					c, err = uaaClient.GetClient("foo")
					Expect(err).NotTo(HaveOccurred())
					Expect(c).To(BeNil())
				})
			})
		})

		Describe("#CreateClient", func() {
			var (
				createHandler *helpers.FakeHandler
			)

			BeforeEach(func() {
				createHandler = new(helpers.FakeHandler)

				server.RouteToHandler(http.MethodPost, regexp.MustCompile(`/oauth/clients`), ghttp.CombineHandlers(
					createHandler.Handle,
				))

				createJsonResponse := `{
				  "scope": [ "admin", "read", "write", "extra-scope" ],
				  "client_id": "some-client-id",
				  "resource_ids": ["resource1", "resource2", "some-extra-resource"],
				  "authorized_grant_types": [ "client_credentials", "password", "token" ],
				  "authorities": [ "some-authority", "another-authority", "some-extra-authority" ],
				  "name": "some-name",
				  "lastModified": 1588809891186,
				  "required_user_groups": [ ]
				}`
				createHandler.RespondsWith(http.StatusCreated, createJsonResponse)
			})

			It("doesn't go to uaa when client definition is not provided", func() {
				uaaConfig.ClientDefinition = config.ClientDefinition{}
				uaaClient, err := uaa.New(uaaConfig, trustedCert)
				Expect(err).NotTo(HaveOccurred())

				actualClient, err := uaaClient.CreateClient("some-client-id", "some-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(actualClient).To(BeNil())

				Expect(createHandler.RequestsReceived()).To(Equal(0))
			})

			It("creates and returns a client map", func() {
				uaaClient.RandFunc = func() string {
					return "superrandomsecret"
				}

				actualClient, err := uaaClient.CreateClient("some-client-id", "some-name")
				Expect(err).NotTo(HaveOccurred())

				By("injecting some properties", func() {
					Expect(actualClient["client_id"]).To(Equal("some-client-id"))
					Expect(actualClient["client_secret"]).To(Equal("superrandomsecret"))
					Expect(actualClient["name"]).To(Equal("some-name"))
				})

				By("using the configured properties", func() {
					Expect(actualClient["scopes"]).To(Equal(uaaConfig.ClientDefinition.Scopes + ",extra-scope"))
					Expect(actualClient["resource_ids"]).To(Equal(uaaConfig.ClientDefinition.ResourceIDs + ",some-extra-resource"))
					Expect(actualClient["authorities"]).To(Equal(uaaConfig.ClientDefinition.Authorities + ",some-extra-authority"))
					Expect(actualClient["authorized_grant_types"]).To(Equal(uaaConfig.ClientDefinition.AuthorizedGrantTypes + ",token"))
				})

				By("creating a client on UAA", func() {
					Expect(createHandler.RequestsReceived()).To(Equal(1))
					request := createHandler.GetRequestForCall(0)
					Expect(request.Body).To(MatchJSON(`
						{
                          "scope": [ "admin", "read", "write" ],
						  "client_id": "some-client-id",
						  "client_secret": "superrandomsecret",
						  "resource_ids": ["resource1", "resource2"],
						  "authorized_grant_types": [ "client_credentials", "password" ],
						  "authorities": [ "some-authority", "another-authority" ],
						  "name": "some-name"
						}`,
					), "Expected request body mismatch")
				})
			})

			It("generates a new password every time it is called", func() {
				c1, _ := uaaClient.CreateClient("foo", "foo")
				c2, _ := uaaClient.CreateClient("foo", "foo")

				Expect(c1["client_secret"]).NotTo(Equal(c2["client_secret"]))
			})

			It("generates unique but reproducible ids and names", func() {
				_, err := uaaClient.CreateClient("client1", "name1")
				Expect(err).NotTo(HaveOccurred())
				_, err = uaaClient.CreateClient("client2", "name2")
				Expect(err).NotTo(HaveOccurred())

				_, err = uaaClient.CreateClient("client1", "name1")
				Expect(err).NotTo(HaveOccurred())

				c1ReqBody := toMap(createHandler.GetRequestForCall(0).Body)
				c2ReqBody := toMap(createHandler.GetRequestForCall(1).Body)
				anotherC1ReqBody := toMap(createHandler.GetRequestForCall(2).Body)

				Expect(c1ReqBody["name"]).NotTo(Equal(c2ReqBody["name"]), "names are not unique")
				Expect(c1ReqBody["client_id"]).NotTo(Equal(c2ReqBody["client_id"]), "client_ids are not unique")
				Expect(c1ReqBody["name"]).To(Equal(anotherC1ReqBody["name"]), "name are not reproducible")
				Expect(c1ReqBody["client_id"]).To(Equal(anotherC1ReqBody["client_id"]), "client_ids are not reproducible")
			})

			It("does not generate a name if not passed", func() {
				_, err := uaaClient.CreateClient("client1", "")
				Expect(err).NotTo(HaveOccurred())

				c1ReqBody := toMap(createHandler.GetRequestForCall(0).Body)
				Expect(c1ReqBody).NotTo(HaveKey("name"))
			})

			It("fails when UAA responds with error", func() {
				createHandler.RespondsOnCall(0, 500, "")
				_, err := uaaClient.CreateClient("some-client-id", "some-name")
				Expect(err).To(HaveOccurred())

				errorMsg := fmt.Sprintf("An error occurred while calling %s/oauth/clients", server.URL())
				Expect(err).To(MatchError(ContainSubstring(errorMsg)))
			})
		})

		Describe("#UpdateClient", func() {
			var updateHandler *helpers.FakeHandler

			BeforeEach(func() {
				updateHandler = new(helpers.FakeHandler)

				server.RouteToHandler(
					http.MethodPut, regexp.MustCompile(`/oauth/clients/some-client-id`),
					ghttp.CombineHandlers(
						updateHandler.Handle,
					),
				)

				updateJsonResponse := `{
				  "scope": [ "admin", "read", "write", "extra-scope" ],
				  "client_id": "some-client-id",
				  "resource_ids": ["resource1", "resource2", "some-extra-resource"],
				  "authorized_grant_types": [ "client_credentials", "password", "token" ],
				  "authorities": [ "some-authority", "another-authority", "some-extra-authority" ],
                  "redirect_uri": ["https://example.com/dashboard/some-client-id/response"],
				  "name": "some-name",
				  "lastModified": 1588809891186,
				  "required_user_groups": [ ]
				}`
				updateHandler.RespondsWith(http.StatusCreated, updateJsonResponse)
			})

			It("doesn't go to uaa when client definition is not provided", func() {
				uaaConfig.ClientDefinition = config.ClientDefinition{}
				uaaClient, err := uaa.New(uaaConfig, trustedCert)
				Expect(err).NotTo(HaveOccurred())

				actualClient, err := uaaClient.UpdateClient("some-client-id", "some-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(actualClient).To(BeNil())

				Expect(updateHandler.RequestsReceived()).To(Equal(0))
			})

			It("updates and returns a client map", func() {
				actualClient, err := uaaClient.UpdateClient("some-client-id", "https://example.com/dashboard/some-client-id")
				Expect(err).NotTo(HaveOccurred())

				By("updating the client on UAA", func() {
					Expect(updateHandler.RequestsReceived()).To(Equal(1))
					request := updateHandler.GetRequestForCall(0)
					Expect(request.Body).To(MatchJSON(`
						{
                          "scope": [ "admin", "read", "write" ],
						  "client_id": "some-client-id",
						  "resource_ids": ["resource1", "resource2"],
                          "redirect_uri": ["https://example.com/dashboard/some-client-id"],
						  "authorized_grant_types": [ "client_credentials", "password" ],
						  "authorities": [ "some-authority", "another-authority" ]
						}`,
					), "Expected request body mismatch")
				})

				By("injecting some properties", func() {
					Expect(actualClient["client_id"]).To(Equal("some-client-id"))
				})

				By("using the configured and returned properties", func() {
					Expect(actualClient["scopes"]).To(Equal(uaaConfig.ClientDefinition.Scopes + ",extra-scope"))
					Expect(actualClient["resource_ids"]).To(Equal(uaaConfig.ClientDefinition.ResourceIDs + ",some-extra-resource"))
					Expect(actualClient["authorities"]).To(Equal(uaaConfig.ClientDefinition.Authorities + ",some-extra-authority"))
					Expect(actualClient["authorized_grant_types"]).To(Equal(uaaConfig.ClientDefinition.AuthorizedGrantTypes + ",token"))
				})

			})

			It("does not send redirect_uri when not passed", func() {
				_, err := uaaClient.UpdateClient("some-client-id", "")
				Expect(err).NotTo(HaveOccurred())
				Expect(updateHandler.RequestsReceived()).To(Equal(1))
				request := updateHandler.GetRequestForCall(0)
				Expect(request.Body).To(MatchJSON(`
						{
                          "scope": [ "admin", "read", "write" ],
						  "client_id": "some-client-id",
						  "resource_ids": ["resource1", "resource2"],
						  "authorized_grant_types": [ "client_credentials", "password" ],
						  "authorities": [ "some-authority", "another-authority" ]
						}`,
				), "Expected request body mismatch")
			})

			It("fails when UAA responds with error", func() {
				updateHandler.RespondsOnCall(0, 500, "")
				_, err := uaaClient.UpdateClient("some-client-id", "some-dashboard")
				Expect(err).To(HaveOccurred())

				errorMsg := fmt.Sprintf("An error occurred while calling %s/oauth/clients/some-client-id", server.URL())
				Expect(err).To(MatchError(ContainSubstring(errorMsg)))
			})
		})

		Describe("#DeleteClient", func() {
			var (
				deleteHandler *helpers.FakeHandler
			)

			BeforeEach(func() {
				deleteHandler = new(helpers.FakeHandler)

				server.RouteToHandler(
					http.MethodDelete, regexp.MustCompile(`/oauth/clients/some-client-id`),
					ghttp.CombineHandlers(
						deleteHandler.Handle,
					),
				)

				deleteJsonResponse := `{
				  "scope": [ "admin", "read", "write", "extra-scope" ],
				  "client_id": "some-client-id",
				  "resource_ids": ["resource1", "resource2", "some-extra-resource"],
				  "authorized_grant_types": [ "client_credentials", "password", "token" ],
				  "authorities": [ "some-authority", "another-authority", "some-extra-authority" ],
                  "redirect_uri": ["https://example.com/dashboard/some-client-id/response"],
				  "name": "some-name",
				  "lastModified": 1588809891186,
				  "required_user_groups": [ ]
				}`
				deleteHandler.RespondsWith(http.StatusOK, deleteJsonResponse)
			})

			It("deletes the client successfully", func() {
				err := uaaClient.DeleteClient("some-client-id")
				Expect(err).NotTo(HaveOccurred())

				By("deleting the client on UAA", func() {
					Expect(deleteHandler.RequestsReceived()).To(Equal(1))
				})
			})

			It("fails when UAA responds with error", func() {
				deleteHandler.RespondsOnCall(0, http.StatusNotFound, "")
				err := uaaClient.DeleteClient("some-client-id")
				Expect(err).To(HaveOccurred())

				errorMsg := fmt.Sprintf("An error occurred while calling %s/oauth/clients/some-client-id", server.URL())
				Expect(err).To(MatchError(ContainSubstring(errorMsg)))
			})
		})

		Describe("#GetClient", func() {
			var (
				listHandler *helpers.FakeHandler
				query       []string
			)

			BeforeEach(func() {
				listHandler = new(helpers.FakeHandler)

				server.RouteToHandler(http.MethodGet, regexp.MustCompile(`/oauth/clients`), ghttp.CombineHandlers(
					listHandler.Handle,
				))

				query = []string{`count=1`, `filter=client_id+eq+%22some-client-id%22`, `startIndex=1`}
				listHandler.
					WithQueryParams(query...).
					RespondsWith(http.StatusOK, `{"resources":[{"client_id":"some-client-id"}]}`)
			})

			It("returns a client when the client exists", func() {
				client, err := uaaClient.GetClient("some-client-id")
				Expect(err).NotTo(HaveOccurred())
				Expect(client).NotTo(BeNil())
				Expect(client["client_id"]).To(Equal("some-client-id"))
			})

			It("returns nil when the client does not exist", func() {
				listHandler.
					WithQueryParams(query...).
					RespondsWith(http.StatusOK, `{"resources":[]}`)

				client, err := uaaClient.GetClient("some-client-id")
				Expect(err).NotTo(HaveOccurred())
				Expect(client).To(BeNil())
			})

			It("fails when cannot query list of clients", func() {
				listHandler.
					WithQueryParams(query...).
					RespondsWith(http.StatusBadRequest, `{"resources":[]}`)

				_, err := uaaClient.GetClient("some-client-id")
				Expect(err).To(HaveOccurred())
				errorMsg := fmt.Sprintf("An error occurred while calling %s/oauth/clients", server.URL())
				Expect(err).To(MatchError(ContainSubstring(errorMsg)))
			})
		})

		Describe("#HasClientDefinition", func() {
			It("returns true when at least one property is set", func() {
				c := config.UAAConfig{ClientDefinition: config.ClientDefinition{AuthorizedGrantTypes: "123"}}

				client, err := uaa.New(c, "")
				Expect(err).ToNot(HaveOccurred())
				Expect(client.HasClientDefinition()).To(BeTrue())

				c = config.UAAConfig{ClientDefinition: config.ClientDefinition{Authorities: "asd"}}
				client, err = uaa.New(c, "")
				Expect(err).ToNot(HaveOccurred())
				Expect(client.HasClientDefinition()).To(BeTrue())

				c = config.UAAConfig{ClientDefinition: config.ClientDefinition{ResourceIDs: "fff"}}
				client, err = uaa.New(c, "")
				Expect(err).ToNot(HaveOccurred())
				Expect(client.HasClientDefinition()).To(BeTrue())

				c = config.UAAConfig{ClientDefinition: config.ClientDefinition{Scopes: "admin"}}
				client, err = uaa.New(c, "")
				Expect(err).ToNot(HaveOccurred())
				Expect(client.HasClientDefinition()).To(BeTrue())
			})

			It("returns false when no property is set", func() {
				client, err := uaa.New(config.UAAConfig{}, "")
				Expect(err).ToNot(HaveOccurred())
				Expect(client.HasClientDefinition()).To(BeFalse())
			})
		})
	})
})

func toMap(body string) map[string]interface{} {
	var m map[string]interface{}
	err := json.Unmarshal([]byte(body), &m)
	Expect(err).NotTo(HaveOccurred())
	return m
}

func setupUAARoutes(uaaAPI *ghttp.Server, uaaConfig config.UAAConfig) {
	uaaAuthenticationHandler := new(helpers.FakeHandler)
	secret := uaaConfig.Authentication.ClientCredentials.Secret
	id := uaaConfig.Authentication.ClientCredentials.ID
	uaaAPI.RouteToHandler(http.MethodPost, regexp.MustCompile(`/oauth/token`), ghttp.CombineHandlers(
		ghttp.VerifyBasicAuth(id, secret),
		uaaAuthenticationHandler.Handle,
	))
	authenticationResponse := `{ "access_token": " some-token", "expires_in": 3600, "token_type":"bearer"}`
	uaaAuthenticationHandler.RespondsWith(http.StatusOK, authenticationResponse)
}
