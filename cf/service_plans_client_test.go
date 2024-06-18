package cf_test

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/cf/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/integration_tests/helpers"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
)

var _ = Describe("ServicePlansClient", func() {
	var server *mockhttp.Server
	var testLogger *log.Logger
	var logBuffer *bytes.Buffer
	var authHeaderBuilder *fakes.FakeAuthHeaderBuilder

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
		testLogger = log.New(io.MultiWriter(logBuffer, GinkgoWriter), "my-app", log.LstdFlags)
	})

	AfterEach(func() {
		server.VerifyMocks()
	})
	Describe("GetPlanByServiceInstanceGUID", func() {
		It("returns service plan", func() {
			cfApi := ghttp.NewServer()
			servicePlanHandler := new(helpers.FakeHandler)

			expectedServiceGUID := "plan-unique-id"
			cfApi.RouteToHandler(http.MethodGet, regexp.MustCompile(`/v2/service_plans`), ghttp.CombineHandlers(
				ghttp.VerifyRequest(http.MethodGet, ContainSubstring("/v2/service_plans"), "q=service_instance_guid:"+expectedServiceGUID),
				servicePlanHandler.Handle,
			))
			servicePlanResponse := `{ "resources":[{ "entity": { "maintenance_info": { "version": "0.31.0" }}}]}`
			servicePlanHandler.RespondsWith(http.StatusOK, servicePlanResponse)

			client, err := cf.New(cfApi.URL(), authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			actualPlan, err := client.GetPlanByServiceInstanceGUID(expectedServiceGUID, testLogger)

			Expect(servicePlanHandler.RequestsReceived()).To(Equal(1), "should call service plan handler")
			Expect(actualPlan.ServicePlanEntity.MaintenanceInfo).To(Equal(cf.MaintenanceInfo{Version: "0.31.0"}))
			Expect(err).To(Not(HaveOccurred()))
		})

		It("returns error when CF endpoint errors", func() {
			cfApi := ghttp.NewServer()
			servicePlanHandler := new(helpers.FakeHandler)

			expectedServiceGUID := "plan-unique-id"
			cfApi.RouteToHandler(http.MethodGet, regexp.MustCompile(`/v2/service_plans`), ghttp.CombineHandlers(
				ghttp.VerifyRequest(http.MethodGet, "/v2/service_plans", "q=service_instance_guid:"+expectedServiceGUID),
				servicePlanHandler.Handle,
			))
			servicePlanHandler.RespondsWith(http.StatusBadRequest, `{}`)

			client, err := cf.New(cfApi.URL(), authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.GetPlanByServiceInstanceGUID(expectedServiceGUID, testLogger)

			Expect(servicePlanHandler.RequestsReceived()).To(Equal(1), "should call service plan handler")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("failed to retrieve plan for service")))
		})
	})
})
