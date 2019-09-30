package cf_test

import (
	"bytes"
	"github.com/pivotal-cf/on-demand-service-broker/integration_tests/helpers"
	"io"
	"log"
	"net/http"
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/cf/fakes"
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
		serviceGUID           = "06df08f9-5a58-4d33-8097-32d0baf3ce1e"
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

	Describe("GetServiceInstances", func() {
		It("returns a list of instances filtered by the service offering", func() {
			offeringID := "8F3E8998-5FD0-4F32-924A-5478DC390A5F"

			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans("34c08156-5b5d-4cc1-9af1-29cda9ec056f").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			instances, err := client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: offeringID}, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(instances).To(ConsistOf(
				cf.Instance{GUID: "520f8566-b727-4c67-8be8-d9285645e936", PlanUniqueID: "11789210-D743-4C65-9D38-C80B29F4D9C8"},
				cf.Instance{GUID: "f897f40d-0b2d-474a-a5c9-98426a2cb4b8", PlanUniqueID: "22789210-D743-4C65-9D38-C80B29F4D9C8"},
				cf.Instance{GUID: "2f759033-04a4-426b-bccd-01722036c152", PlanUniqueID: "22789210-D743-4C65-9D38-C80B29F4D9C8"},
			))
		})

		Context("when the list of services spans multiple pages", func() {
			It("returns a list of instance IDs", func() {
				offeringID := "D94A086D-203D-4966-A6F1-60A9E2300F72"

				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response_page_1.json")),
					mockcfapi.ListServiceOfferingsForPage(2).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response_page_2.json")),
					mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
					mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
					mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				instances, err := client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: offeringID}, testLogger)
				Expect(err).NotTo(HaveOccurred())
				Expect(instances).To(ConsistOf(
					cf.Instance{GUID: "520f8566-b727-4c67-8be8-d9285645e936", PlanUniqueID: "11789210-D743-4C65-9D38-C80B29F4D9C8"},
					cf.Instance{GUID: "f897f40d-0b2d-474a-a5c9-98426a2cb4b8", PlanUniqueID: "22789210-D743-4C65-9D38-C80B29F4D9C8"},
					cf.Instance{GUID: "2f759033-04a4-426b-bccd-01722036c152", PlanUniqueID: "22789210-D743-4C65-9D38-C80B29F4D9C8"},
				))
			})
		})

		Context("when the list of plans spans multiple pages", func() {
			It("returns a list of instance IDs", func() {
				offeringID := "D94A086D-203D-4966-A6F1-60A9E2300F72"

				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
					mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_1.json")),
					mockcfapi.ListServicePlansForPage(serviceGUID, 2).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_2.json")),
					mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
					mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				instances, err := client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: offeringID}, testLogger)
				Expect(err).NotTo(HaveOccurred())
				Expect(instances).To(ConsistOf(
					cf.Instance{GUID: "520f8566-b727-4c67-8be8-d9285645e936", PlanUniqueID: "11789210-D743-4C65-9D38-C80B29F4D9C8"},
					cf.Instance{GUID: "f897f40d-0b2d-474a-a5c9-98426a2cb4b8", PlanUniqueID: "22789210-D743-4C65-9D38-C80B29F4D9C8"},
					cf.Instance{GUID: "2f759033-04a4-426b-bccd-01722036c152", PlanUniqueID: "22789210-D743-4C65-9D38-C80B29F4D9C8"},
				))
			})
		})

		Context("when the list of instances spans multiple pages", func() {
			It("returns a list of instance IDs", func() {
				offeringID := "D94A086D-203D-4966-A6F1-60A9E2300F72"

				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
					mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
					mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
					mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_page_1.json")),
					mockcfapi.ListServiceInstancesForPage("2777ad05-8114-4169-8188-2ef5f39e0c6b", 2).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_page_2.json")),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				instances, err := client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: offeringID}, testLogger)
				Expect(err).NotTo(HaveOccurred())
				Expect(instances).To(ConsistOf(
					cf.Instance{GUID: "520f8566-b727-4c67-8be8-d9285645e936", PlanUniqueID: "11789210-D743-4C65-9D38-C80B29F4D9C8"},
					cf.Instance{GUID: "f897f40d-0b2d-474a-a5c9-98426a2cb4b8", PlanUniqueID: "22789210-D743-4C65-9D38-C80B29F4D9C8"},
					cf.Instance{GUID: "2f759033-04a4-426b-bccd-01722036c152", PlanUniqueID: "22789210-D743-4C65-9D38-C80B29F4D9C8"},
				))
			})
		})

		It("when there are no instances, returns an empty list of instances", func() {
			offeringID := "D94A086D-203D-4966-A6F1-60A9E2300F72"

			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_empty_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_empty_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			instances, err := client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: offeringID}, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(instances).To(Equal([]cf.Instance{}))
		})

		Context("when the list of services cannot be retrieved", func() {
			It("returns an error", func() {
				offeringID := "8F3E8998-5FD0-4F32-924A-5478DC390A5F"

				server.VerifyAndMock(

					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("oops"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: offeringID}, testLogger)
				Expect(err).To(MatchError(ContainSubstring("oops")))
			})
		})

		Context("when the list of plans for the service cannot be retrieved", func() {
			It("returns an error", func() {
				offeringID := "8F3E8998-5FD0-4F32-924A-5478DC390A5F"

				server.VerifyAndMock(

					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
					mockcfapi.ListServicePlans("34c08156-5b5d-4cc1-9af1-29cda9ec056f").WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("oops"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: offeringID}, testLogger)
				Expect(err).To(MatchError(ContainSubstring("oops")))
			})
		})

		Context("when the list of instances for a plan cannot be retrieved", func() {
			It("returns an error", func() {
				offeringID := "8F3E8998-5FD0-4F32-924A-5478DC390A5F"

				server.VerifyAndMock(

					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
					mockcfapi.ListServicePlans("34c08156-5b5d-4cc1-9af1-29cda9ec056f").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
					mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("oops"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: offeringID}, testLogger)
				Expect(err).To(MatchError(ContainSubstring("oops")))
			})
		})

		Context("filtering by service offering, org and space", func() {
			var (
				orgName, spaceName, orgGuid, spaceGuid string
			)

			BeforeEach(func() {
				orgName = "cf-org"
				spaceName = "cf-space"
				orgGuid = "an-org-guid"
				spaceGuid = "a-space-guid"
			})

			It("returns a list of instances, filtered by org and spaces", func() {
				offeringID := "8F3E8998-5FD0-4F32-924A-5478DC390A5F"
				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
					mockcfapi.ListServicePlans("34c08156-5b5d-4cc1-9af1-29cda9ec056f").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
					mockcfapi.ListOrg(orgName).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("org_response.json")),
					mockcfapi.ListOrgSpace(orgGuid, spaceName).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("space_response.json")),
					mockcfapi.ListServiceInstancesBySpace("ff717e7c-afd5-4d0a-bafe-16c7eff546ec", spaceGuid).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(
						fixture("list_service_instances_for_plan_1_response.json"),
					),
					mockcfapi.ListServiceInstancesBySpace("2777ad05-8114-4169-8188-2ef5f39e0c6b", spaceGuid).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(
						fixture("list_service_instances_for_plan_2_response.json"),
					),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				instances, err := client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: offeringID, OrgName: orgName, SpaceName: spaceName}, testLogger)
				Expect(err).NotTo(HaveOccurred())
				Expect(instances).To(ConsistOf(
					cf.Instance{GUID: "520f8566-b727-4c67-8be8-d9285645e936", PlanUniqueID: "11789210-D743-4C65-9D38-C80B29F4D9C8"},
					cf.Instance{GUID: "f897f40d-0b2d-474a-a5c9-98426a2cb4b8", PlanUniqueID: "22789210-D743-4C65-9D38-C80B29F4D9C8"},
					cf.Instance{GUID: "2f759033-04a4-426b-bccd-01722036c152", PlanUniqueID: "22789210-D743-4C65-9D38-C80B29F4D9C8"},
				))
			})

			It("returns an empty list when the org doesn't exist", func() {
				offeringID := "8F3E8998-5FD0-4F32-924A-5478DC390A5F"

				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
					mockcfapi.ListServicePlans("34c08156-5b5d-4cc1-9af1-29cda9ec056f").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
					mockcfapi.ListOrg(orgName).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(`{"resources":[]}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				instances, err := client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: offeringID, OrgName: orgName, SpaceName: spaceName}, testLogger)
				Expect(err).NotTo(HaveOccurred())
				Expect(instances).To(Equal([]cf.Instance{}))
			})

			It("returns an empty list when the space doesn't exist", func() {
				offeringID := "8F3E8998-5FD0-4F32-924A-5478DC390A5F"

				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
					mockcfapi.ListServicePlans("34c08156-5b5d-4cc1-9af1-29cda9ec056f").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
					mockcfapi.ListOrg(orgName).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("org_response.json")),
					mockcfapi.ListOrgSpace(orgGuid, spaceName).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(`{"resources":[]}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				instances, err := client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: offeringID, OrgName: orgName, SpaceName: spaceName}, testLogger)
				Expect(err).NotTo(HaveOccurred())
				Expect(instances).To(Equal([]cf.Instance{}))
			})

			Context("error cases", func() {
				It("errors when the space url cannot be retrieved", func() {
					offeringID := "8F3E8998-5FD0-4F32-924A-5478DC390A5F"

					server.VerifyAndMock(
						mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
						mockcfapi.ListServicePlans("34c08156-5b5d-4cc1-9af1-29cda9ec056f").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
						mockcfapi.ListOrg(orgName).WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("oops"),
					)

					client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
					Expect(err).NotTo(HaveOccurred())

					_, err = client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: offeringID, OrgName: orgName, SpaceName: spaceName}, testLogger)
					Expect(err).To(MatchError(ContainSubstring("oops")))
				})

				It("errors when the space guid cannot be retrieved", func() {
					offeringID := "8F3E8998-5FD0-4F32-924A-5478DC390A5F"

					server.VerifyAndMock(
						mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
						mockcfapi.ListServicePlans("34c08156-5b5d-4cc1-9af1-29cda9ec056f").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
						mockcfapi.ListOrg(orgName).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("org_response.json")),
						mockcfapi.ListOrgSpace(orgGuid, spaceName).WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("oops"),
					)

					client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
					Expect(err).NotTo(HaveOccurred())

					_, err = client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: offeringID, OrgName: orgName, SpaceName: spaceName}, testLogger)
					Expect(err).To(MatchError(ContainSubstring("oops")))
				})
			})
		})
	})
})
