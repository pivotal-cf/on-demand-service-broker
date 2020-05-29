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

		Describe("Constructor", func() {
			BeforeEach(func() {
				uaaConfig = config.UAAConfig{
					URL: "some-url",
				}
			})

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
				_, err := uaa.New(config.UAAConfig{}, trustedCert)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("the target is missing"))
			})
		})

		Describe("#CreateClient", func() {
			var (
				createHandler *helpers.FakeHandler
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

				createHandler = new(helpers.FakeHandler)
				setupUAARoutes(server, uaaConfig)

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

			AfterEach(func() {
				server.Close()
			})

			It("creates and returns a client map", func() {
				uaaClient.RandFunc = func() string {
					return "superrandomsecret"
				}

				actualClient, err := uaaClient.CreateClient("some-client-id", "some-name")
				Expect(err).NotTo(HaveOccurred())

				By("generating some properties", func() {
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
			var (
				updateHandler *helpers.FakeHandler
			)

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

			It("updates and returns a client map", func() {
				uaaClient.RandFunc = func() string {
					return "a-new-updated-secret"
				}

				actualClient, err := uaaClient.UpdateClient("some-client-id", "https://example.com/dashboard/some-client-id")
				Expect(err).NotTo(HaveOccurred())

				By("updating the client on UAA", func() {
					Expect(updateHandler.RequestsReceived()).To(Equal(1))
					request := updateHandler.GetRequestForCall(0)
					Expect(request.Body).To(MatchJSON(`
						{
                          "scope": [ "admin", "read", "write" ],
						  "client_id": "some-client-id",
						  "client_secret": "a-new-updated-secret",
						  "resource_ids": ["resource1", "resource2"],
                          "redirect_uri": ["https://example.com/dashboard/some-client-id"],
						  "authorized_grant_types": [ "client_credentials", "password" ],
						  "authorities": [ "some-authority", "another-authority" ]
						}`,
					), "Expected request body mismatch")
				})

				By("generating some properties", func() {
					Expect(actualClient["client_id"]).To(Equal("some-client-id"))
					Expect(actualClient["client_secret"]).To(Equal("a-new-updated-secret"))
				})

				By("using the configured and returned properties", func() {
					Expect(actualClient["scopes"]).To(Equal(uaaConfig.ClientDefinition.Scopes + ",extra-scope"))
					Expect(actualClient["resource_ids"]).To(Equal(uaaConfig.ClientDefinition.ResourceIDs + ",some-extra-resource"))
					Expect(actualClient["authorities"]).To(Equal(uaaConfig.ClientDefinition.Authorities + ",some-extra-authority"))
					Expect(actualClient["authorized_grant_types"]).To(Equal(uaaConfig.ClientDefinition.AuthorizedGrantTypes + ",token"))
				})

			})

			It("fails when UAA responds with error", func() {
				updateHandler.RespondsOnCall(0, 500, "")
				_, err := uaaClient.UpdateClient("some-client-id", "some-dashboard")
				Expect(err).To(HaveOccurred())

				errorMsg := fmt.Sprintf("An error occurred while calling %s/oauth/clients/some-client-id", server.URL())
				Expect(err).To(MatchError(ContainSubstring(errorMsg)))
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
