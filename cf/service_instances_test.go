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
		serviceGUID           = "06df08f9-5a58-4d33-8097-32d0baf3ce1e"
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
