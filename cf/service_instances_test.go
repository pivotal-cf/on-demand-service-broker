package cf_test

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"

	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/cf/fakes"
)

var _ = Describe("ServiceInstancesClient", func() {
	var (
		server            *mockhttp.Server
		testLogger        *log.Logger
		logBuffer         *gbytes.Buffer
		authHeaderBuilder *fakes.FakeAuthHeaderBuilder
		cfApi             *ghttp.Server
	)

	const (
		cfAuthorizationHeader = "auth-header"
	)

	BeforeEach(func() {
		authHeaderBuilder = new(fakes.FakeAuthHeaderBuilder)
		authHeaderBuilder.AddAuthHeaderStub = func(req *http.Request, logger *log.Logger) error {
			req.Header.Set("Authorization", cfAuthorizationHeader)
			return nil
		}
		server = mockcfapi.New()
		logBuffer = gbytes.NewBuffer()
		testLogger = log.New(io.MultiWriter(logBuffer, GinkgoWriter), "service-instances-test", log.LstdFlags)
		cfApi = ghttp.NewServer()

	})

	AfterEach(func() {
		server.VerifyMocks()
	})

	Describe("GetServiceInstance", func() {
		It("successfully gets a service instance", func() {
			cfApi.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest(http.MethodGet, "/v2/service_instances/fake-service-instance-guid"),
				ghttp.RespondWith(
					http.StatusOK,
					`{
						"metadata": {
							"guid": "fake-service-instance-guid"
						},
						"entity": {
							"service_plan_url": "fake-url",
							"maintenance_info": {
								"version": "1.2.3"
							},
							"last_operation": {
								"type": "fake-type",
								"state": "fake-state"
							}
						}
					}`,
				),
			))

			client, err := cf.New(cfApi.URL(), authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			instance, err := client.GetServiceInstance("fake-service-instance-guid", testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(instance).To(Equal(cf.ServiceInstanceResource{
				Metadata: cf.Metadata{
					GUID: "fake-service-instance-guid",
				},
				Entity: cf.ServiceInstanceEntity{
					ServicePlanURL: "fake-url",
					MaintenanceInfo: cf.MaintenanceInfo{
						Version: "1.2.3",
					},
					LastOperation: cf.LastOperation{
						Type:  "fake-type",
						State: "fake-state",
					},
				},
			}))
		})

		It("returns an error when getting the service instance failed", func() {
			cfApi.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest(http.MethodGet, "/v2/service_instances/fake-service-instance-guid"),
				ghttp.RespondWith(http.StatusInternalServerError, ""),
			))

			client, err := cf.New(cfApi.URL(), authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.GetServiceInstance("fake-service-instance-guid", testLogger)
			Expect(err).To(MatchError(ContainSubstring("Unexpected reponse status 500")))
		})
	})

	Describe("UpgradeServiceInstance", func() {
		It("returns last operation", func() {
			expectedServiceInstanceGUID := "service-instance-guid"

			serviceInstanceResponse := `
			{
				"entity": {
					"last_operation": {
						"type": "update",
						"state": "in progress"
					}
				}
			}`

			cfApi.RouteToHandler(http.MethodPut, regexp.MustCompile(`/v2/service_instances/*`), ghttp.CombineHandlers(
				ghttp.VerifyRequest(http.MethodPut, fmt.Sprintf(`/v2/service_instances/%s`, expectedServiceInstanceGUID), "accepts_incomplete=true"),
				ghttp.VerifyBody([]byte(`{"maintenance_info":{"version":"1.2.3"}}`)),
				ghttp.RespondWith(http.StatusAccepted, serviceInstanceResponse),
			))

			client, err := cf.New(cfApi.URL(), authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			actualResponse, err := client.UpgradeServiceInstance(expectedServiceInstanceGUID, cf.MaintenanceInfo{Version: "1.2.3"}, testLogger)

			Expect(err).NotTo(HaveOccurred())
			Expect(actualResponse.State).To(Equal(cf.OperationStateInProgress))
		})

		It("returns error when CF endpoint returns an unexpected HTTP response code", func() {
			expectedServiceInstanceGUID := "service-instance-guid"

			cfApi.RouteToHandler(http.MethodPut, regexp.MustCompile(`/v2/service_instances/*`), ghttp.CombineHandlers(
				ghttp.VerifyRequest(http.MethodPut, fmt.Sprintf(`/v2/service_instances/%s`, expectedServiceInstanceGUID), "accepts_incomplete=true"),
				ghttp.VerifyBody([]byte(`{"maintenance_info":{"version":"1.2.3"}}`)),
				ghttp.RespondWith(http.StatusInternalServerError, ""),
			))

			client, err := cf.New(cfApi.URL(), authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.UpgradeServiceInstance(expectedServiceInstanceGUID, cf.MaintenanceInfo{Version: "1.2.3"}, testLogger)

			Expect(err).To(MatchError(`unexpected response status 500 when upgrading service instance "service-instance-guid"; response body ""`))
		})

		It("returns error when CF endpoint returns invalid JSON", func() {
			cfApi := ghttp.NewServer()
			expectedServiceInstanceGUID := "service-instance-guid"

			cfApi.RouteToHandler(http.MethodPut, regexp.MustCompile(`/v2/service_instances/*`), ghttp.CombineHandlers(
				ghttp.VerifyRequest(http.MethodPut, fmt.Sprintf(`/v2/service_instances/%s`, expectedServiceInstanceGUID), "accepts_incomplete=true"),
				ghttp.VerifyBody([]byte(`{"maintenance_info":{"version":"1.2.3"}}`)),
				ghttp.RespondWith(http.StatusAccepted, "this-is-not-json"),
			))

			client, err := cf.New(cfApi.URL(), authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.UpgradeServiceInstance(expectedServiceInstanceGUID, cf.MaintenanceInfo{Version: "1.2.3"}, testLogger)

			Expect(err.Error()).To(ContainSubstring(`failed to de-serialise the response body`))
		})
	})

	Describe("DeleteServiceInstance", func() {
		const serviceInstanceGUID = "596736f1-eee4-4249-a201-e21f00a55209"

		Context("when the request is accepted", func() {
			var err error

			BeforeEach(func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceInstance(serviceInstanceGUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsAcceptedWith(""),
				)

				var client cf.Client
				client, err = cf.New(server.URL, authHeaderBuilder, nil, true, nil)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteServiceInstance(serviceInstanceGUID, testLogger)
			})

			It("does not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("logs the delete request", func() {
				Expect(logBuffer).To(gbytes.Say("DELETE %s/v2/service_instances/%s\\?accepts_incomplete\\=true", server.URL, serviceInstanceGUID))
			})
		})

		Context("when the response is 404 Not Found", func() {
			It("does not return an error", func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceInstance(serviceInstanceGUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsNotFoundWith(`{"foo":"bar"}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteServiceInstance(serviceInstanceGUID, testLogger)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the response has an unexpected status code", func() {
			It("return the error", func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceInstance(serviceInstanceGUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsForbiddenWith(`{"foo":"bar"}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteServiceInstance(serviceInstanceGUID, testLogger)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unexpected reponse status 403"))
				Expect(err.Error()).To(ContainSubstring(`"{\"foo\":\"bar\"}"`))
			})
		})

		Context("when the auth header builder returns an error", func() {
			It("returns the error", func() {
				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				authHeaderBuilder.AddAuthHeaderReturns(errors.New("no header for you"))

				err = client.DeleteServiceInstance(serviceInstanceGUID, testLogger)
				Expect(err).To(MatchError("no header for you"))
			})
		})
	})

	Describe("GetBindingsForInstance", func() {
		const serviceInstanceGUID = "92d707ce-c06c-421a-a1d2-ed1e750af650"

		It("returns a list of bindings", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceBindings(serviceInstanceGUID).
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_bindings_response_page_1.json")),
				mockcfapi.ListServiceBindingsForPage(serviceInstanceGUID, 2).
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_bindings_response_page_2.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			bindings, err := client.GetBindingsForInstance(serviceInstanceGUID, testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(bindings).To(HaveLen(2))
			Expect(bindings[0].GUID).To(Equal("83a87158-92b2-46ea-be66-9dad6b2cb116"))
			Expect(bindings[0].AppGUID).To(Equal("31809eda-4bdd-44fc-b804-eefe662b3a98"))

			Expect(bindings[1].GUID).To(Equal("9dad-6b2cb116-83a87158-92b2-46ea-be66"))
			Expect(bindings[1].AppGUID).To(Equal("eefe-662b3a98-31809eda-4bdd-44fcb804"))
		})

		Context("when the list bindings request fails", func() {
			It("returns an error", func() {
				server.VerifyAndMock(
					mockcfapi.ListServiceBindings(serviceInstanceGUID).RespondsInternalServerErrorWith("no bindings for you"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetBindingsForInstance(serviceInstanceGUID, testLogger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(`Unexpected reponse status 500, "no bindings for you"`))
			})
		})
	})

	Describe("GetServiceKeysForInstance", func() {
		const serviceInstanceGUID = "92d707ce-c06c-421a-a1d2-ed1e750af650"

		It("return a list of service keys", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceKeys(serviceInstanceGUID).
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_service_keys_response_page_1.json")),
				mockcfapi.ListServiceKeysForPage(serviceInstanceGUID, 2).
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_service_keys_response_page_2.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			serviceKeys, err := client.GetServiceKeysForInstance(serviceInstanceGUID, testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(serviceKeys).To(HaveLen(2))
			Expect(serviceKeys[0].GUID).To(Equal("3c8076a6-0d85-11e7-811e-685b3585cc4e"))
			Expect(serviceKeys[1].GUID).To(Equal("23549ec8-0d85-11e7-82e1-685b3585cc4e"))
		})

		Context("when the list service keys request fails", func() {
			It("returns an error", func() {
				server.VerifyAndMock(
					mockcfapi.ListServiceKeys(serviceInstanceGUID).RespondsInternalServerErrorWith("no service keys for you"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetServiceKeysForInstance(serviceInstanceGUID, testLogger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(`Unexpected reponse status 500, "no service keys for you"`))
			})
		})
	})
})
