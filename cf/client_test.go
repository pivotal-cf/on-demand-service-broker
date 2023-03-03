// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package cf_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/cf/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
)

var _ = Describe("Client", func() {
	var server *mockhttp.Server
	var testLogger *log.Logger
	var logBuffer *gbytes.Buffer
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
		logBuffer = gbytes.NewBuffer()
		testLogger = log.New(io.MultiWriter(logBuffer, GinkgoWriter), "my-app", log.LstdFlags)
	})

	AfterEach(func() {
		server.VerifyMocks()
		server.Close()
	})

	Describe("GetServiceOfferingGUID", func() {
		It("returns the broker guid", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceBrokers().
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_brokers_page_1_response.json")),
				mockcfapi.ListServiceBrokersForPage(2).
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_brokers_page_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			var brokerGUID string
			brokerGUID, err = client.GetServiceOfferingGUID("service-broker-name-2", testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(brokerGUID).To(Equal("service-broker-guid-2-guid"))

		})

		It("returns an error if it fails to get service brokers", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceBrokers().
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsInternalServerErrorWith("failed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.GetServiceOfferingGUID("service-broker-name-2", testLogger)
			Expect(err).To(MatchError(ContainSubstring("failed")))
		})

		It("logs if it fails to find a broker with the correct name", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceBrokers().
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_brokers_page_1_response.json")),
				mockcfapi.ListServiceBrokersForPage(2).
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_brokers_page_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.GetServiceOfferingGUID("not-a-real-broker", testLogger)
			Expect(err).ToNot(HaveOccurred())
			Expect(logBuffer).To(gbytes.Say("No service broker found with name: not-a-real-broker"))
		})
	})

	Describe("DisableServiceAccessForAllPlans", func() {
		const offeringID = "D94A086D-203D-4966-A6F1-60A9E2300F72"

		It("disables all the plans across pages", func() {

			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_1.json")),
				mockcfapi.ListServicePlansForPage(serviceGUID, 2).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_2.json")),
				mockcfapi.DisablePlanAccess("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsCreated(),
				mockcfapi.DisablePlanAccess("2777ad05-8114-4169-8188-2ef5f39e0c6b").RespondsCreated(),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.DisableServiceAccessForAllPlans(offeringID, testLogger)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error if it fails to get plans for service offering", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("failed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.DisableServiceAccessForAllPlans(offeringID, testLogger)
			Expect(err).To(MatchError(ContainSubstring("failed")))
		})

		It("returns an error if it fails to update the service plan", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_1.json")),
				mockcfapi.ListServicePlansForPage(serviceGUID, 2).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_2.json")),
				mockcfapi.DisablePlanAccess("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsInternalServerErrorWith("failed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.DisableServiceAccessForAllPlans(offeringID, testLogger)
			Expect(err).To(MatchError(ContainSubstring("failed")))
		})
	})

	Describe("EnableServiceAccess", func() {
		It("enables access for plan", func() {
			serviceID := "redis-test"
			serviceGUID := "34c08156-5b5d-4cc1-9af1-29cda9ec056f"
			planID := "dedicated-vm"
			planGUID := "11789210-D743-4C65-9D38-C80B29F4D9C8"

			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": { "name": %q },
							"metadata": { "guid": %q }
						}, {
							"entity": { "name": "other-plan" },
							"metadata": { "guid": "some-guid" }
						}]
					}
					`, planID, planGUID)),
				mockcfapi.EnablePlanAccess(planGUID).RespondsCreated(),
				mockcfapi.ListServicePlanVisibilities(planGUID).RespondsOKWith(`{"resources": null}`),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.EnableServiceAccess(serviceID, planID, testLogger)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error if it fails to get plans for service offering", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("failed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.EnableServiceAccess("redis-test", "plan-id", testLogger)
			Expect(err).To(MatchError(ContainSubstring("failed")))
		})

		It("returns an error if it fails to set service access", func() {
			serviceID := "redis-test"
			serviceGUID := "34c08156-5b5d-4cc1-9af1-29cda9ec056f"
			planID := "dedicated-vm"
			planGUID := "11789210-D743-4C65-9D38-C80B29F4D9C8"

			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`
					{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": { "name": %q },
							"metadata": { "guid": %q }
						}, {
							"entity": { "name": "other-plan" },
							"metadata": { "guid": "some-guid" }
						}]
					}
					`, planID, planGUID)),
				mockcfapi.EnablePlanAccess(planGUID).RespondsInternalServerErrorWith("failed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.EnableServiceAccess(serviceID, planID, testLogger)
			Expect(err).To(MatchError(SatisfyAll(
				ContainSubstring("failed"),
				ContainSubstring("500"),
				ContainSubstring(planGUID),
			)))
		})

		It("returns an error if it fails to find the plan for service", func() {
			serviceID := "redis-test"
			serviceGUID := "34c08156-5b5d-4cc1-9af1-29cda9ec056f"
			planID := "the-plan-i-am-looking-for"

			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`
					{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(`{
						"resources": [{
							"entity": { "name": "not-the-plan-you-are-looking-for" },
							"metadata": { "guid": "some-guid" }
						}]
					}
					`),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.EnableServiceAccess(serviceID, planID, testLogger)
			Expect(err).To(MatchError(ContainSubstring(`planID "the-plan-i-am-looking-for" not found while updating plan access`)))
		})

		When("the plan has visibilities", func() {
			It("deletes the visibilities", func() {
				serviceID := "redis-test"
				serviceGUID := "34c08156-5b5d-4cc1-9af1-29cda9ec056f"
				planID := "dedicated-vm"
				planGUID := "11789210-D743-4C65-9D38-C80B29F4D9C8"

				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
					mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": { "name": %q },
							"metadata": { "guid": %q }
						}, {
							"entity": { "name": "other-plan" },
							"metadata": { "guid": "some-guid" }
						}]
					}
					`, planID, planGUID)),
					mockcfapi.EnablePlanAccess(planGUID).RespondsCreated(),
					mockcfapi.ListServicePlanVisibilities(planGUID).RespondsOKWith(`{"resources": [
						{ "metadata": { "guid": "d1b5ea55-f354-4f43-b52e-53045747adb9" } },
						{ "metadata": { "guid": "some-plan-visibility-guid" } }
					]}`),
					mockcfapi.DeleteServicePlanVisibility("d1b5ea55-f354-4f43-b52e-53045747adb9").RespondsNoContent(),
					mockcfapi.DeleteServicePlanVisibility("some-plan-visibility-guid").RespondsNoContent(),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				err = client.EnableServiceAccess(serviceID, planID, testLogger)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error if it fails to get the service plan visibilities", func() {
				serviceID := "redis-test"
				serviceGUID := "34c08156-5b5d-4cc1-9af1-29cda9ec056f"
				planID := "dedicated-vm"
				planGUID := "11789210-D743-4C65-9D38-C80B29F4D9C8"

				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
					mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": { "name": %q },
							"metadata": { "guid": %q }
						}, {
							"entity": { "name": "other-plan" },
							"metadata": { "guid": "some-guid" }
						}]
					}
					`, planID, planGUID)),
					mockcfapi.EnablePlanAccess(planGUID).RespondsCreated(),
					mockcfapi.ListServicePlanVisibilities(planGUID).RespondsInternalServerErrorWith("nope"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				err = client.EnableServiceAccess(serviceID, planID, testLogger)
				Expect(err.Error()).To(SatisfyAll(
					ContainSubstring("nope"),
					ContainSubstring("failed to get plan visibilities for plan %s", planGUID),
				))
			})

			It("returns an error if it fails to delete the service plan visibilities", func() {
				serviceID := "redis-test"
				serviceGUID := "34c08156-5b5d-4cc1-9af1-29cda9ec056f"
				planID := "dedicated-vm"
				planGUID := "11789210-D743-4C65-9D38-C80B29F4D9C8"

				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
					mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": { "name": %q },
							"metadata": { "guid": %q }
						}, {
							"entity": { "name": "other-plan" },
							"metadata": { "guid": "some-guid" }
						}]
					}
					`, planID, planGUID)),
					mockcfapi.EnablePlanAccess(planGUID).RespondsCreated(),
					mockcfapi.ListServicePlanVisibilities(planGUID).RespondsOKWith(`{"resources": [
						{ "metadata": { "guid": "d1b5ea55-f354-4f43-b52e-53045747adb9" } },
						{ "metadata": { "guid": "some-plan-visibility-guid" } }
					]}`),
					mockcfapi.DeleteServicePlanVisibility("d1b5ea55-f354-4f43-b52e-53045747adb9").RespondsInternalServerErrorWith("nope"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				err = client.EnableServiceAccess(serviceID, planID, testLogger)
				Expect(err.Error()).To(SatisfyAll(
					ContainSubstring("nope"),
					ContainSubstring("failed to delete plan visibility for plan %s", planGUID),
				))
			})

		})
	})

	Describe("DisableServiceAccess", func() {
		It("disables access for plan", func() {
			serviceID := "service-id"
			serviceGUID := "service-guid"
			planID := "disabled-plan-id"
			planGUID := "disabled-plan-guid"

			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": { "name": %q },
							"metadata": { "guid": %q }
						}, {
							"entity": { "name": "other-plan" },
							"metadata": { "guid": "some-guid" }
						}]
					}
					`, planID, planGUID)),
				mockcfapi.DisablePlanAccess(planGUID).RespondsCreated(),
				mockcfapi.ListServicePlanVisibilities(planGUID).RespondsOKWith(`{"resources": null}`),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.DisableServiceAccess(serviceID, planID, testLogger)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error if it fails to get plans for service offering", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("failed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.DisableServiceAccess("a-plan", "plan-id", testLogger)
			Expect(err).To(MatchError(ContainSubstring("failed")))
		})

		It("returns an error if it fails to find the plan for service", func() {
			serviceID := "service-id"
			serviceGUID := "service-guid"
			planID := "inexistent-plan-id"

			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`
					{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(`{
						"resources": [{
							"entity": { "name": "not-the-plan-you-are-looking-for" },
							"metadata": { "guid": "some-guid" }
						}]
					}
					`),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.DisableServiceAccess(serviceID, planID, testLogger)
			Expect(err).To(MatchError(ContainSubstring("planID %q not found while updating plan access", planID)))
		})

		It("returns an error if it fails to set service access", func() {
			serviceID := "service-id"
			serviceGUID := "service-guid"
			planID := "disabled-plan-id"
			planGUID := "disabled-plan-guid"

			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`
					{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": { "name": %q },
							"metadata": { "guid": %q }
						}, {
							"entity": { "name": "other-plan" },
							"metadata": { "guid": "some-guid" }
						}]
					}
					`, planID, planGUID)),
				mockcfapi.DisablePlanAccess(planGUID).RespondsInternalServerErrorWith("failed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.DisableServiceAccess(serviceID, planID, testLogger)
			Expect(err).To(MatchError(SatisfyAll(
				ContainSubstring("failed"),
				ContainSubstring("500"),
				ContainSubstring(planGUID),
			)))
		})

		When("the plan has visibilities", func() {
			It("deletes the visibilities", func() {
				serviceID := "service-id"
				serviceGUID := "service-guid"
				planID := "disabled-plan-id"
				planGUID := "disabled-plan-guid"

				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
					mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": { "name": %q },
							"metadata": { "guid": %q }
						}, {
							"entity": { "name": "other-plan" },
							"metadata": { "guid": "some-guid" }
						}]
					}
					`, planID, planGUID)),
					mockcfapi.DisablePlanAccess(planGUID).RespondsCreated(),
					mockcfapi.ListServicePlanVisibilities(planGUID).RespondsOKWith(`{"resources": [
						{ "metadata": { "guid": "d1b5ea55-f354-4f43-b52e-53045747adb9" } },
						{ "metadata": { "guid": "some-plan-visibility-guid" } }
					]}`),
					mockcfapi.DeleteServicePlanVisibility("d1b5ea55-f354-4f43-b52e-53045747adb9").RespondsNoContent(),
					mockcfapi.DeleteServicePlanVisibility("some-plan-visibility-guid").RespondsNoContent(),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				err = client.DisableServiceAccess(serviceID, planID, testLogger)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error if it fails to get the service plan visibilities", func() {
				serviceID := "service-id"
				serviceGUID := "service-guid"
				planID := "disabled-plan-id"
				planGUID := "disabled-plan-guid"

				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
					mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": { "name": %q },
							"metadata": { "guid": %q }
						}, {
							"entity": { "name": "other-plan" },
							"metadata": { "guid": "some-guid" }
						}]
					}
					`, planID, planGUID)),
					mockcfapi.DisablePlanAccess(planGUID).RespondsCreated(),
					mockcfapi.ListServicePlanVisibilities(planGUID).RespondsInternalServerErrorWith("nope"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				err = client.DisableServiceAccess(serviceID, planID, testLogger)
				Expect(err.Error()).To(SatisfyAll(
					ContainSubstring("nope"),
					ContainSubstring("failed to get plan visibilities for plan %s", planGUID),
				))
			})

			It("returns an error if it fails to delete the service plan visibilities", func() {
				serviceID := "service-id"
				serviceGUID := "service-guid"
				planID := "disabled-plan-id"
				planGUID := "disabled-plan-guid"

				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
					mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": { "name": %q },
							"metadata": { "guid": %q }
						}, {
							"entity": { "name": "other-plan" },
							"metadata": { "guid": "some-guid" }
						}]
					}
					`, planID, planGUID)),
					mockcfapi.DisablePlanAccess(planGUID).RespondsCreated(),
					mockcfapi.ListServicePlanVisibilities(planGUID).RespondsOKWith(`{"resources": [
						{ "metadata": { "guid": "d1b5ea55-f354-4f43-b52e-53045747adb9" } },
						{ "metadata": { "guid": "some-plan-visibility-guid" } }
					]}`),
					mockcfapi.DeleteServicePlanVisibility("d1b5ea55-f354-4f43-b52e-53045747adb9").RespondsInternalServerErrorWith("nope"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				err = client.DisableServiceAccess(serviceID, planID, testLogger)
				Expect(err.Error()).To(SatisfyAll(
					ContainSubstring("nope"),
					ContainSubstring("failed to delete plan visibility for plan %s", planGUID),
				))
			})
		})
	})

	Describe("Deregister", func() {
		const brokerGUID = "broker-guid"

		It("does not return an error", func() {
			server.VerifyAndMock(
				mockcfapi.DeregisterBroker(brokerGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsNoContent(),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.DeregisterBroker(brokerGUID, testLogger)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error when the deregister fails", func() {
			server.VerifyAndMock(
				mockcfapi.DeregisterBroker(brokerGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("failed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.DeregisterBroker(brokerGUID, testLogger)
			Expect(err).To(MatchError(ContainSubstring("failed")))

		})
	})

	Describe("CountInstancesOfServiceOffering", func() {
		It("fetches instance counts per plan", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)).To(Equal(map[cf.ServicePlan]int{
				servicePlan(
					"ff717e7c-afd5-4d0a-bafe-16c7eff546ec",
					"11789210-D743-4C65-9D38-C80B29F4D9C8",
					"/v2/service_plans/ff717e7c-afd5-4d0a-bafe-16c7eff546ec/service_instances",
					"small",
				): 1,
				servicePlan(
					"2777ad05-8114-4169-8188-2ef5f39e0c6b",
					"22789210-D743-4C65-9D38-C80B29F4D9C8",
					"/v2/service_plans/2777ad05-8114-4169-8188-2ef5f39e0c6b/service_instances",
					"big",
				): 2,
			}))
		})

		It("finds no instances when the service is not registered with cf", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_empty_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)).To(Equal(map[cf.ServicePlan]int{}))
		})

		It("fails if getting a new token fails", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsUnauthorizedWith(`{"code": 1000,"description": "Invalid Auth Token","error_code": "CF-InvalidAuthToken"}`),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("Invalid Auth Token")))
		})

		It("reuses tokens", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)).To(Equal(map[cf.ServicePlan]int{
				servicePlan(
					"ff717e7c-afd5-4d0a-bafe-16c7eff546ec",
					"11789210-D743-4C65-9D38-C80B29F4D9C8",
					"/v2/service_plans/ff717e7c-afd5-4d0a-bafe-16c7eff546ec/service_instances",
					"small",
				): 1,
				servicePlan(
					"2777ad05-8114-4169-8188-2ef5f39e0c6b",
					"22789210-D743-4C65-9D38-C80B29F4D9C8",
					"/v2/service_plans/2777ad05-8114-4169-8188-2ef5f39e0c6b/service_instances",
					"big",
				): 2,
			}))

			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
			)

			Expect(client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)).To(Equal(map[cf.ServicePlan]int{
				servicePlan(
					"ff717e7c-afd5-4d0a-bafe-16c7eff546ec",
					"11789210-D743-4C65-9D38-C80B29F4D9C8",
					"/v2/service_plans/ff717e7c-afd5-4d0a-bafe-16c7eff546ec/service_instances",
					"small",
				): 1,
				servicePlan(
					"2777ad05-8114-4169-8188-2ef5f39e0c6b",
					"22789210-D743-4C65-9D38-C80B29F4D9C8",
					"/v2/service_plans/2777ad05-8114-4169-8188-2ef5f39e0c6b/service_instances",
					"big",
				): 2,
			}))
		})

		It("fetches instance counts per plan, across service pages", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response_page_1.json")),
				mockcfapi.ListServiceOfferingsForPage(2).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response_page_2.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)).To(Equal(map[cf.ServicePlan]int{
				servicePlan(
					"ff717e7c-afd5-4d0a-bafe-16c7eff546ec",
					"11789210-D743-4C65-9D38-C80B29F4D9C8",
					"/v2/service_plans/ff717e7c-afd5-4d0a-bafe-16c7eff546ec/service_instances",
					"small",
				): 1,
				servicePlan(
					"2777ad05-8114-4169-8188-2ef5f39e0c6b",
					"22789210-D743-4C65-9D38-C80B29F4D9C8",
					"/v2/service_plans/2777ad05-8114-4169-8188-2ef5f39e0c6b/service_instances",
					"big",
				): 2,
			}))
		})

		It("fetches instance counts per plan, across plan pages", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_1.json")),
				mockcfapi.ListServicePlansForPage(serviceGUID, 2).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_2.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)).To(Equal(map[cf.ServicePlan]int{
				servicePlan(
					"ff717e7c-afd5-4d0a-bafe-16c7eff546ec",
					"11789210-D743-4C65-9D38-C80B29F4D9C8",
					"/v2/service_plans/ff717e7c-afd5-4d0a-bafe-16c7eff546ec/service_instances",
					"small",
				): 1,
				servicePlan(
					"2777ad05-8114-4169-8188-2ef5f39e0c6b",
					"22789210-D743-4C65-9D38-C80B29F4D9C8",
					"/v2/service_plans/2777ad05-8114-4169-8188-2ef5f39e0c6b/service_instances",
					"big",
				): 2,
			}))
		})

		It("fails, if fetching auth token fails", func() {
			authHeaderBuilder.AddAuthHeaderReturns(errors.New("niet goed"))

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.GetServiceInstances(cf.GetInstancesFilter{ServiceOfferingID: "some-offering"}, testLogger)
			Expect(err).To(MatchError(ContainSubstring("niet goed")))
		})

		It("fails if fetching services fails", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsInternalServerErrorWith("niet goed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)
			Expect(err).To(MatchError(ContainSubstring("niet goed")))
		})

		It("fails if fetching services fails", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsInternalServerErrorWith("niet goed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)
			Expect(err).To(MatchError(ContainSubstring("niet goed")))
		})

		It("fails if fetching service plans fails", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).RespondsInternalServerErrorWith("niet goed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)
			Expect(err).To(MatchError(ContainSubstring("niet goed")))
		})

		It("fails if fetching service instances fails", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsInternalServerErrorWith("niet goed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)
			Expect(err).To(MatchError(ContainSubstring("niet goed")))
		})
	})

	Describe("CountInstancesOfPlan", func() {
		It("fetches instance counts for the plan", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.CountInstancesOfPlan("D94A086D-203D-4966-A6F1-60A9E2300F72", "22789210-D743-4C65-9D38-C80B29F4D9C8", testLogger)).To(Equal(2))
		})

		It("fail if service instance not found", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			count, err := client.CountInstancesOfPlan("D94A086D-203D-4966-A6F1-60A9E2300F72", "does-not-exist", testLogger)
			Expect(err).To(MatchError(ContainSubstring("service plan does-not-exist not found")))
			Expect(count).To(BeZero())
		})

		It("fails when it can't retrieve services", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("no services for you"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			count, err := client.CountInstancesOfPlan("D94A086D-203D-4966-A6F1-60A9E2300F72", "22789210-D743-4C65-9D38-C80B29F4D9C8", testLogger)
			Expect(err).To(MatchError(ContainSubstring("no services for you")))
			Expect(count).To(BeZero())
		})

		It("fails when it can't retrieve service plans", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).RespondsInternalServerErrorWith("no service plans for you"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			count, err := client.CountInstancesOfPlan("D94A086D-203D-4966-A6F1-60A9E2300F72", "22789210-D743-4C65-9D38-C80B29F4D9C8", testLogger)
			Expect(err).To(MatchError(ContainSubstring("no service plans for you")))
			Expect(count).To(BeZero())
		})

		It("fails when it can't retrieve service instances for the plan", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").RespondsInternalServerErrorWith("no instances for you"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			count, err := client.CountInstancesOfPlan("D94A086D-203D-4966-A6F1-60A9E2300F72", "22789210-D743-4C65-9D38-C80B29F4D9C8", testLogger)
			Expect(err).To(MatchError(ContainSubstring("no instances for you")))
			Expect(count).To(BeZero())
		})

		It("fails when it receives an empty json", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith("{}"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			count, err := client.CountInstancesOfPlan("D94A086D-203D-4966-A6F1-60A9E2300F72", "22789210-D743-4C65-9D38-C80B29F4D9C8", testLogger)
			Expect(count).To(BeZero())
			Expect(err).To(MatchError(ContainSubstring("Empty response body")))
		})
	})

	Describe("GetLastOperationForInstance", func() {
		It("fetches the instance", func() {
			server.VerifyAndMock(
				mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").
					RespondsOKWith(fixture("get_service_instance_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			lastOperation, err := client.GetLastOperationForInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(lastOperation.Type).To(Equal(cf.OperationType("create")))
			Expect(lastOperation.State).To(Equal(cf.OperationState("succeeded")))
		})

		Context("when the service instance does not exist", func() {
			It("returns a not found error with the API error description", func() {
				server.VerifyAndMock(
					mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").RespondsNotFoundWith(`{
						"code": 60004,
   					"description": "The service instance could not be found: 783f8645-1ded-4161-b457-73f59423f9eb",
   					"error_code": "CF-ServiceInstanceNotFound"
   				}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetLastOperationForInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
				Expect(err).To(BeAssignableToTypeOf(cf.ResourceNotFoundError{}))
				Expect(err).To(MatchError("The service instance could not be found: 783f8645-1ded-4161-b457-73f59423f9eb"))
			})
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				server.VerifyAndMock(
					mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").RespondsInternalServerErrorWith("er ma gerd"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetLastOperationForInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
				Expect(err).To(MatchError(ContainSubstring("er ma gerd")))
				Expect(err).NotTo(BeAssignableToTypeOf(cf.ResourceNotFoundError{}))
			})
		})

		Context("when the request is unauthorized", func() {
			Context("when the response is a CF API error response", func() {
				It("returns an unauthorized error with the API error description", func() {
					server.VerifyAndMock(
						mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").
							RespondsUnauthorizedWith(`{
								"code": 10002,
								"description": "Authentication error",
								"error_code": "CF-NotAuthenticated"
							}`),
					)

					client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
					Expect(err).NotTo(HaveOccurred())

					_, err = client.GetLastOperationForInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
					Expect(err).To(MatchError(ContainSubstring("Authentication error")))
					Expect(err).To(BeAssignableToTypeOf(cf.UnauthorizedError{}))
				})
			})

			Context("when the response is invalid json", func() {
				It("returns an unauthorized error with the response body", func() {
					server.VerifyAndMock(
						mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").
							RespondsUnauthorizedWith("not valid json"),
					)

					client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
					Expect(err).NotTo(HaveOccurred())

					_, err = client.GetLastOperationForInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
					Expect(err).To(MatchError(ContainSubstring("not valid json")))
					Expect(err).To(BeAssignableToTypeOf(cf.UnauthorizedError{}))
				})
			})
		})

		Context("when the request is forbidden", func() {
			Context("when the response is a CF API error response", func() {
				It("returns an unauthorized error with the API error description", func() {
					server.VerifyAndMock(
						mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").
							RespondsForbiddenWith(`{
								"code": 10003,
								"description": "You are not authorized to perform the requested action",
								"error_code": "CF-NotAuthorized"
							}`),
					)

					client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
					Expect(err).NotTo(HaveOccurred())

					_, err = client.GetLastOperationForInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
					Expect(err).To(MatchError(ContainSubstring("You are not authorized to perform the requested action")))
					Expect(err).To(BeAssignableToTypeOf(cf.ForbiddenError{}))
				})
			})

			Context("when the response is invalid json", func() {
				It("returns an unauthorized error with the response body", func() {
					server.VerifyAndMock(
						mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").
							RespondsForbiddenWith("not valid json"),
					)

					client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
					Expect(err).NotTo(HaveOccurred())

					_, err = client.GetLastOperationForInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
					Expect(err).To(MatchError(ContainSubstring("not valid json")))
					Expect(err).To(BeAssignableToTypeOf(cf.ForbiddenError{}))
				})
			})
		})

		Context("when the request succeeds with an invalid response body", func() {
			It("returns an invalid response error", func() {
				server.VerifyAndMock(
					mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").
						RespondsOKWith("not valid json"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetLastOperationForInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
				Expect(err).To(MatchError(ContainSubstring("Invalid response body")))
				Expect(err).To(MatchError(ContainSubstring("invalid character 'o'")))
				Expect(err).To(BeAssignableToTypeOf(cf.InvalidResponseError{}))
			})
		})
	})

	Describe("DeleteBinding", func() {
		var binding cf.Binding

		BeforeEach(func() {
			binding = cf.Binding{
				GUID:    "596736f1-eee4-4249-a201-e21f00a55209",
				AppGUID: "65bdd3a3-f471-4108-a7e8-67627ba76d6a",
			}
		})

		Context("when the response is 204 No Content", func() {
			var err error

			BeforeEach(func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceBinding(binding.AppGUID, binding.GUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsNoContent(),
				)

				var client cf.Client
				client, err = cf.New(server.URL, authHeaderBuilder, nil, true, nil)
				Expect(err).NotTo(HaveOccurred())
				err = client.DeleteBinding(binding, testLogger)
			})

			It("does not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("logs the delete request", func() {
				Expect(logBuffer).To(gbytes.Say("DELETE %s/v2/apps/%s/service_bindings/%s", server.URL, binding.AppGUID, binding.GUID))
			})
		})

		Context("when the response is 404 Not Found", func() {
			It("does not return an error", func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceBinding(binding.AppGUID, binding.GUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsNotFoundWith(`{"foo":"bar"}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteBinding(binding, testLogger)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the auth header builder returns an error", func() {
			It("returns the error", func() {
				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				authHeaderBuilder.AddAuthHeaderReturns(errors.New("no header for you"))

				err = client.DeleteBinding(binding, testLogger)
				Expect(err).To(MatchError("no header for you"))
			})
		})

		Context("when the response has an unexpected status code", func() {
			It("return the error", func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceBinding(binding.AppGUID, binding.GUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsForbiddenWith(`{"foo":"bar"}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteBinding(binding, testLogger)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unexpected reponse status 403"))
				Expect(err.Error()).To(ContainSubstring(`"{\"foo\":\"bar\"}"`))
			})
		})
	})

	Describe("DeleteServiceKey", func() {
		var serviceKey cf.ServiceKey

		BeforeEach(func() {
			serviceKey = cf.ServiceKey{
				GUID: "596736f1-eee4-4249-a201-e21f00a55209",
			}
		})

		Context("when the response is 204 No Content", func() {
			var err error

			BeforeEach(func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceKey(serviceKey.GUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsNoContent(),
				)

				var client cf.Client
				client, err = cf.New(server.URL, authHeaderBuilder, nil, true, nil)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteServiceKey(serviceKey, testLogger)
			})

			It("does not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("logs the delete request", func() {
				Expect(logBuffer).To(gbytes.Say("DELETE %s/v2/service_keys/%s", server.URL, serviceKey.GUID))
			})
		})

		Context("when the response is 404 Not Found", func() {
			It("does not return an error", func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceKey(serviceKey.GUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsNotFoundWith(`{"foo":"bar"}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteServiceKey(serviceKey, testLogger)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the auth header builder returns an error", func() {
			It("returns the error", func() {
				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				authHeaderBuilder.AddAuthHeaderReturns(errors.New("no header for you"))

				err = client.DeleteServiceKey(serviceKey, testLogger)
				Expect(err).To(MatchError("no header for you"))
			})
		})

		Context("when the response has an unexpected status code", func() {
			It("return the error", func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceKey(serviceKey.GUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsForbiddenWith(`{"foo":"bar"}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteServiceKey(serviceKey, testLogger)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unexpected reponse status 403"))
				Expect(err.Error()).To(ContainSubstring(`"{\"foo\":\"bar\"}"`))
			})
		})
	})

	Describe("ServiceBrokers", func() {
		It("returns the a list of brokers", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceBrokers().
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_brokers_page_1_response.json")),
				mockcfapi.ListServiceBrokersForPage(2).
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_brokers_page_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			serviceBrokers, err := client.ServiceBrokers()
			Expect(err).NotTo(HaveOccurred())
			Expect(serviceBrokers).To(HaveLen(2))
			Expect(serviceBrokers).To(ConsistOf(
				cf.ServiceBroker{GUID: "service-broker-guid-1", Name: "service-broker-name-1"},
				cf.ServiceBroker{GUID: "service-broker-guid-2-guid", Name: "service-broker-name-2"},
			))
		})

		It("returns an error if it fails to get service brokers", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceBrokers().
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsInternalServerErrorWith("failed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.ServiceBrokers()
			Expect(err).To(MatchError(ContainSubstring("failed")))
		})
	})

	Describe("CreateServiceBroker", func() {
		It("Creates a service broker", func() {
			server.VerifyAndMock(
				mockcfapi.CreateServiceBroker().
					WithAuthorizationHeader(cfAuthorizationHeader).
					WithJSONBody(`{
					  "name": "service-broker-name",
					  "broker_url": "https://broker.example.com",
					  "auth_username": "exampleUser",
					  "auth_password": "examplePassword"
					}`).
					RespondsCreated(),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.CreateServiceBroker("service-broker-name", "exampleUser", "examplePassword", "https://broker.example.com")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error if it fails to create", func() {
			server.VerifyAndMock(
				mockcfapi.CreateServiceBroker().
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsInternalServerErrorWith("failed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.CreateServiceBroker("service-broker-name", "exampleUser", "examplePassword", "https://broker.example.com")
			Expect(err).To(MatchError(ContainSubstring("failed")))
		})

		It("returns an error if creating the auth header fails", func() {
			authHeaderBuilder.AddAuthHeaderReturns(errors.New("failed building header"))

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.CreateServiceBroker("service-broker-name", "exampleUser", "examplePassword", "https://broker.example.com")
			Expect(err).To(MatchError(ContainSubstring("failed building header")))
		})

	})

	Describe("UpdateServiceBroker", func() {
		It("Updates a service broker", func() {
			guid := "a-guid"
			server.VerifyAndMock(
				mockcfapi.UpdateServiceBroker(guid).
					WithAuthorizationHeader(cfAuthorizationHeader).
					WithJSONBody(`{
					  "name": "service-broker-name",
					  "broker_url": "https://broker.example.com",
					  "auth_username": "exampleUser",
					  "auth_password": "examplePassword"
					}`).
					RespondsOKWith(""),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.UpdateServiceBroker(guid, "service-broker-name", "exampleUser", "examplePassword", "https://broker.example.com")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error if it fails to create", func() {
			guid := "a-guid"
			server.VerifyAndMock(
				mockcfapi.UpdateServiceBroker(guid).
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsInternalServerErrorWith("boo"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.UpdateServiceBroker(guid, "service-broker-name", "exampleUser", "examplePassword", "https://broker.example.com")
			Expect(err).To(MatchError(ContainSubstring("boo")))
		})

		It("returns an error if creating the auth header fails", func() {
			authHeaderBuilder.AddAuthHeaderReturns(errors.New("failed building header"))

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.UpdateServiceBroker("a-guid", "service-broker-name", "exampleUser", "examplePassword", "https://broker.example.com")
			Expect(err).To(MatchError(ContainSubstring("failed building header")))
		})

	})

	Describe("CreateServicePlanVisibility", func() {
		It("creates a service plan visibility", func() {
			serviceID := "service-name"
			serviceGUID := "service-guid"

			planID := "plan-name"
			planGUID := "plan-guid"

			orgID := "org-name"
			orgGUID := "org-guid"

			server.VerifyAndMock(
				mockcfapi.ListOrg(orgID).RespondsOKWith(fmt.Sprintf(`{
					"resources": [{
						"metadata": { "guid": "%s" }
					}]
				}`, orgGUID)),
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": { "name": %q },
							"metadata": { "guid": %q }
						}, {
							"entity": { "name": "other-plan" },
							"metadata": { "guid": "some-guid" }
						}]
					}
					`, planID, planGUID)),
				mockcfapi.CreateServicePlanVisibility(orgGUID, planGUID).RespondsCreated(),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.CreateServicePlanVisibility(orgID, serviceID, planID, testLogger)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error if it fails to get list of orgs", func() {
			serviceID := "service-name"
			planID := "plan-name"
			orgID := "org-name"

			server.VerifyAndMock(
				mockcfapi.ListOrg(orgID).RespondsInternalServerErrorWith("failed to list orgs"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.CreateServicePlanVisibility(orgID, serviceID, planID, testLogger)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("failed to list orgs")))
		})

		It("returns an error if it fails to find the org", func() {
			serviceID := "service-name"
			planID := "plan-name"
			orgID := "org-name"

			server.VerifyAndMock(
				mockcfapi.ListOrg(orgID).RespondsOKWith(`{"resources": []}`),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.CreateServicePlanVisibility(orgID, serviceID, planID, testLogger)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("failed to find org with name %q", orgID)))
		})

		It("returns an error when cannot get list of plans", func() {
			serviceID := "service-name"
			planID := "plan-name"
			orgID := "org-name"
			orgGUID := "org-guid"

			server.VerifyAndMock(
				mockcfapi.ListOrg(orgID).RespondsOKWith(fmt.Sprintf(`{ "resources": [{ "metadata": { "guid": "%s" } }] }`, orgGUID)),
				mockcfapi.ListServiceOfferings().RespondsInternalServerErrorWith("failed to get plans"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.CreateServicePlanVisibility(orgID, serviceID, planID, testLogger)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("failed to get plans")))
		})

		It("returns an error when cannot list of plans does not include the target plan", func() {
			serviceID := "service-name"
			planID := "plan-name"
			orgID := "org-name"
			orgGUID := "org-guid"

			server.VerifyAndMock(
				mockcfapi.ListOrg(orgID).RespondsOKWith(fmt.Sprintf(`{ "resources": [{ "metadata": { "guid": "%s" } }] }`, orgGUID)),
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(`{
						"resources": [{
							"entity": { "name": "other-plan" },
							"metadata": { "guid": "some-guid" }
						}]
					}`),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.CreateServicePlanVisibility(orgID, serviceID, planID, testLogger)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("planID %q not found while updating plan access", planID)))
		})

		It("returns an error when creation returns a non-201 status code", func() {
			serviceID := "service-name"
			serviceGUID := "service-guid"

			planID := "plan-name"
			planGUID := "plan-guid"

			orgID := "org-name"
			orgGUID := "org-guid"

			server.VerifyAndMock(
				mockcfapi.ListOrg(orgID).RespondsOKWith(fmt.Sprintf(`{
					"resources": [{
						"metadata": { "guid": "%s" }
					}]
				}`, orgGUID)),
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": {
								"unique_id": %q,
								"service_plans_url": "/v2/services/%s/service_plans"
							}
						}]
					}`, serviceID, serviceGUID)),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fmt.Sprintf(`{
						"resources": [{
							"entity": { "name": %q },
							"metadata": { "guid": %q }
						}, {
							"entity": { "name": "other-plan" },
							"metadata": { "guid": "some-guid" }
						}]
					}
					`, planID, planGUID)),
				mockcfapi.CreateServicePlanVisibility(orgGUID, planGUID).RespondsNoContent(),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true, testLogger)
			Expect(err).NotTo(HaveOccurred())

			err = client.CreateServicePlanVisibility(orgID, serviceID, planID, testLogger)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("unexpected status code 204 when creating service plan visibility")))
		})
	})
})

func servicePlan(guid, uniqueID, servicePlanUrl, name string) cf.ServicePlan {
	return cf.ServicePlan{
		Metadata: cf.Metadata{
			GUID: guid,
		},
		ServicePlanEntity: cf.ServicePlanEntity{
			UniqueID:            uniqueID,
			ServiceInstancesUrl: servicePlanUrl,
			Name:                name,
		},
	}
}

func fixture(filename string) string {
	file, err := os.Open(path.Join("fixtures", filename))
	Expect(err).NotTo(HaveOccurred())
	content, err := ioutil.ReadAll(file)
	Expect(err).NotTo(HaveOccurred())
	return string(content)
}
