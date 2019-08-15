package cf_test

import (
	"bytes"
	"fmt"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
	"io"
	"log"
	"net/http"
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/cf/fakes"
)

var _ = Describe("ServiceInstancesClient", func() {
	var (
		server            *mockhttp.Server
		testLogger        *log.Logger
		logBuffer         *bytes.Buffer
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
		logBuffer = new(bytes.Buffer)
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
})
