// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_tests

import (
	"encoding/json"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
)

var _ = Describe("Catalog", func() {
	var (
		runningBroker *gexec.Session
		config        config.Config
		boshDirector  *mockhttp.Server
		boshUAA       *mockuaa.ClientCredentialsServer
		cfAPI         *mockhttp.Server
		cfUAA         *mockuaa.ClientCredentialsServer
	)

	BeforeEach(func() {
		boshDirector = mockbosh.New()
		boshUAA = mockuaa.NewClientCredentialsServer(boshClientID, boshClientSecret, "bosh uaa token")
		boshDirector.ExpectedAuthorizationHeader(boshUAA.ExpectedAuthorizationHeader())
		cfAPI = mockcfapi.New()
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")
	})

	JustBeforeEach(func() {
		runningBroker = startBrokerWithPassingStartupChecks(config, cfAPI, boshDirector)
	})

	AfterEach(func() {
		killBrokerAndCheckForOpenConnections(runningBroker, cfAPI.URL)
		boshDirector.VerifyMocks()
		boshDirector.Close()
		boshUAA.Close()
		cfAPI.VerifyMocks()
		cfAPI.Close()
		cfUAA.Close()
	})

	Context("without optional fields", func() {
		BeforeEach(func() {
			config = defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)
			config.ServiceCatalog.DashboardClient = nil
		})

		It("returns catalog metadata", func() {
			req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/v2/catalog", brokerPort), nil)
			Expect(err).NotTo(HaveOccurred())
			req = basicAuthBrokerRequest(req)

			response, err := http.DefaultClient.Do(req)

			Expect(err).NotTo(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			defer response.Body.Close()

			catalog := make(map[string][]brokerapi.Service)
			Expect(json.NewDecoder(response.Body).Decode(&catalog)).To(Succeed())
			Expect(catalog).To(Equal(map[string][]brokerapi.Service{
				"services": {
					{
						ID:            serviceID,
						Name:          serviceName,
						Description:   serviceDescription,
						Bindable:      serviceBindable,
						PlanUpdatable: servicePlanUpdatable,
						Metadata: &brokerapi.ServiceMetadata{
							DisplayName:         serviceMetadataDisplayName,
							ImageUrl:            serviceMetadataImageURL,
							LongDescription:     serviceMetaDataLongDescription,
							ProviderDisplayName: serviceMetaDataProviderDisplayName,
							DocumentationUrl:    serviceMetaDataDocumentationURL,
							SupportUrl:          serviceMetaDataSupportURL,
						},
						DashboardClient: nil,
						Tags:            serviceTags,
						Plans: []brokerapi.ServicePlan{
							{
								ID:          dedicatedPlanID,
								Name:        dedicatedPlanName,
								Description: dedicatedPlanDescription,
								Free:        booleanPointer(true),
								Bindable:    booleanPointer(true),
								Metadata: &brokerapi.ServicePlanMetadata{
									Bullets:     dedicatedPlanBullets,
									DisplayName: dedicatedPlanDisplayName,
									Costs: []brokerapi.ServicePlanCost{
										{
											Unit:   dedicatedPlanCostUnit,
											Amount: dedicatedPlanCostAmount,
										},
									},
								},
							},
							{
								ID:          highMemoryPlanID,
								Name:        highMemoryPlanName,
								Description: highMemoryPlanDescription,
								Metadata: &brokerapi.ServicePlanMetadata{
									Bullets:     highMemoryPlanBullets,
									DisplayName: highMemoryPlanDisplayName,
								},
							},
						},
					},
				},
			}))
		})
	})

	Context("with optional 'requires' field", func() {
		BeforeEach(func() {
			config = defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)
			config.ServiceCatalog.Requires = []string{"syslog_drain", "route_forwarding"}
		})

		It("returns catalog metadata", func() {
			req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/v2/catalog", brokerPort), nil)
			Expect(err).NotTo(HaveOccurred())
			req = basicAuthBrokerRequest(req)

			response, err := http.DefaultClient.Do(req)

			Expect(err).NotTo(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			defer response.Body.Close()

			catalog := make(map[string][]brokerapi.Service)
			Expect(json.NewDecoder(response.Body).Decode(&catalog)).To(Succeed())
			Expect(catalog).To(Equal(map[string][]brokerapi.Service{
				"services": {
					{
						ID:            serviceID,
						Name:          serviceName,
						Description:   serviceDescription,
						Bindable:      serviceBindable,
						PlanUpdatable: servicePlanUpdatable,
						Metadata: &brokerapi.ServiceMetadata{
							DisplayName:         serviceMetadataDisplayName,
							ImageUrl:            serviceMetadataImageURL,
							LongDescription:     serviceMetaDataLongDescription,
							ProviderDisplayName: serviceMetaDataProviderDisplayName,
							DocumentationUrl:    serviceMetaDataDocumentationURL,
							SupportUrl:          serviceMetaDataSupportURL,
						},
						DashboardClient: &brokerapi.ServiceDashboardClient{
							ID:          "client-id-1",
							Secret:      "secret-1",
							RedirectURI: "https://dashboard.url",
						},
						Requires: []brokerapi.RequiredPermission{"syslog_drain", "route_forwarding"},
						Tags:     serviceTags,
						Plans: []brokerapi.ServicePlan{
							{
								ID:          dedicatedPlanID,
								Name:        dedicatedPlanName,
								Description: dedicatedPlanDescription,
								Free:        booleanPointer(true),
								Bindable:    booleanPointer(true),
								Metadata: &brokerapi.ServicePlanMetadata{
									Bullets:     dedicatedPlanBullets,
									DisplayName: dedicatedPlanDisplayName,
									Costs: []brokerapi.ServicePlanCost{
										{
											Unit:   dedicatedPlanCostUnit,
											Amount: dedicatedPlanCostAmount,
										},
									},
								},
							},
							{
								ID:          highMemoryPlanID,
								Name:        highMemoryPlanName,
								Description: highMemoryPlanDescription,
								Metadata: &brokerapi.ServicePlanMetadata{
									Bullets:     highMemoryPlanBullets,
									DisplayName: highMemoryPlanDisplayName,
								},
							},
						},
					},
				},
			}))
		})
	})
})
