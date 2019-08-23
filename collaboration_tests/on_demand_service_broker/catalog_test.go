// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package on_demand_service_broker_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/pivotal-cf/brokerapi/domain"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Catalog", func() {
	var schemaParameters = map[string]interface{}{
		"$schema": "http://json-schema.org/draft-04/schema#",
		"properties": map[string]interface{}{
			"flibbles": map[string]interface{}{
				"description": "Number of flibbles to spawn",
				"type":        "integer",
			},
		},
		"type":     "object",
		"required": []interface{}{"flibbles"},
	}
	var defaultSchemas = domain.ServiceSchemas{
		Instance: domain.ServiceInstanceSchema{
			Create: domain.Schema{Parameters: schemaParameters},
			Update: domain.Schema{Parameters: schemaParameters},
		},
		Binding: domain.ServiceBindingSchema{
			Create: domain.Schema{Parameters: schemaParameters},
		},
	}

	var (
		defaultSchemasJSON []byte
		zero               int
	)

	BeforeEach(func() {
		var err error
		defaultSchemasJSON, err = json.Marshal(defaultSchemas)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("without optional fields", func() {
		BeforeEach(func() {
			serviceCatalogConfig := defaultServiceCatalogConfig()
			serviceCatalogConfig.DashboardClient = nil
			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
					EnablePlanSchemas: true,
				},
				ServiceCatalog: serviceCatalogConfig,
			}

			StartServer(conf)
		})

		It("returns catalog", func() {
			fakeCommandRunner.RunWithInputParamsReturns(defaultSchemasJSON, []byte{}, &zero, nil)

			response, bodyContent := doCatalogRequest()

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct catalog response")
			catalog := make(map[string][]domain.Service)
			Expect(json.Unmarshal(bodyContent, &catalog)).To(Succeed())
			Expect(catalog).To(Equal(map[string][]domain.Service{
				"services": {
					{
						ID:            serviceID,
						Name:          serviceName,
						Description:   serviceDescription,
						Bindable:      serviceBindable,
						PlanUpdatable: servicePlanUpdatable,
						Metadata: &domain.ServiceMetadata{
							DisplayName:         serviceMetadataDisplayName,
							ImageUrl:            serviceMetadataImageURL,
							LongDescription:     serviceMetaDataLongDescription,
							ProviderDisplayName: serviceMetaDataProviderDisplayName,
							DocumentationUrl:    serviceMetaDataDocumentationURL,
							SupportUrl:          serviceMetaDataSupportURL,
							Shareable:           &trueVar,
						},
						DashboardClient: nil,
						Tags:            serviceTags,
						Plans: []domain.ServicePlan{
							{
								ID:          dedicatedPlanID,
								Name:        dedicatedPlanName,
								Description: dedicatedPlanDescription,
								Free:        &trueVar,
								Bindable:    &trueVar,
								Schemas:     &defaultSchemas,
								Metadata: &domain.ServicePlanMetadata{
									Bullets:     dedicatedPlanBullets,
									DisplayName: dedicatedPlanDisplayName,
									Costs: []domain.ServicePlanCost{
										{
											Unit:   dedicatedPlanCostUnit,
											Amount: dedicatedPlanCostAmount,
										},
									},
									AdditionalMetadata: map[string]interface{}{
										"foo": "bar",
									},
								},
								MaintenanceInfo: nil,
							},
							{
								ID:          highMemoryPlanID,
								Name:        highMemoryPlanName,
								Description: highMemoryPlanDescription,
								Metadata: &domain.ServicePlanMetadata{
									Bullets:     highMemoryPlanBullets,
									DisplayName: highMemoryPlanDisplayName,
								},
								Schemas:         &defaultSchemas,
								MaintenanceInfo: nil,
							},
						},
					},
				},
			}))
		})

		It("can deal with concurrent requests", func() {
			fakeCommandRunner.RunWithInputParamsReturns(defaultSchemasJSON, []byte{}, &zero, nil)

			var wg sync.WaitGroup
			const threads = 2
			wg.Add(threads)

			for i := 0; i < threads; i++ {
				go func() {
					defer wg.Done()
					defer GinkgoRecover()

					response, _ := doCatalogRequest()

					By("returning the correct HTTP status")
					Expect(response.StatusCode).To(Equal(http.StatusOK))

				}()
			}
			wg.Wait()
		})
	})

	Context("with optional fields", func() {
		BeforeEach(func() {
			fakeMapHasher.HashStub = func(m map[string]string) string {
				var s string
				for key, value := range m {
					s += "hashed-" + key + "-" + value + ";"
				}
				return s
			}
			serviceCatalogConfig := defaultServiceCatalogConfig()
			serviceCatalogConfig.Requires = []string{"syslog_drain", "route_forwarding"}
			serviceCatalogConfig.Plans[0].Metadata.AdditionalMetadata = map[string]interface{}{
				"yo": "bill",
			}
			serviceCatalogConfig.Metadata.AdditionalMetadata = map[string]interface{}{
				"random": "george",
			}
			serviceCatalogConfig.MaintenanceInfo = &brokerConfig.MaintenanceInfo{
				Public: map[string]string{
					"name": "jorge",
				},
				Private: map[string]string{
					"secret": "global_value",
				},
				Version:     "1.2.4+global",
				Description: "Global description",
			}
			serviceCatalogConfig.Plans[0].MaintenanceInfo = &brokerConfig.MaintenanceInfo{
				Public: map[string]string{
					"stemcell_version": "1234",
					"name":             "gloria",
				},
				Private: map[string]string{
					"secret": "plan_value",
				},
				Version:     "1.2.3+plan",
				Description: "Plan description",
			}
			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
					EnablePlanSchemas: true,
				},
				ServiceCatalog: serviceCatalogConfig,
			}

			StartServer(conf)
		})

		It("returns catalog", func() {
			fakeCommandRunner.RunWithInputParamsReturns(defaultSchemasJSON, []byte{}, &zero, nil)

			response, bodyContent := doCatalogRequest()

			By("returning the correct HTTP status")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct catalog response")
			catalog := make(map[string][]domain.Service)
			Expect(json.Unmarshal(bodyContent, &catalog)).To(Succeed())
			Expect(catalog).To(Equal(map[string][]domain.Service{
				"services": {
					{
						ID:            serviceID,
						Name:          serviceName,
						Description:   serviceDescription,
						Bindable:      serviceBindable,
						PlanUpdatable: servicePlanUpdatable,
						Metadata: &domain.ServiceMetadata{
							DisplayName:         serviceMetadataDisplayName,
							ImageUrl:            serviceMetadataImageURL,
							LongDescription:     serviceMetaDataLongDescription,
							ProviderDisplayName: serviceMetaDataProviderDisplayName,
							DocumentationUrl:    serviceMetaDataDocumentationURL,
							SupportUrl:          serviceMetaDataSupportURL,
							Shareable:           &trueVar,
							AdditionalMetadata: map[string]interface{}{
								"random": "george",
							},
						},
						DashboardClient: &domain.ServiceDashboardClient{
							ID:          "client-id-1",
							Secret:      "secret-1",
							RedirectURI: "https://dashboard.url",
						},
						Requires: []domain.RequiredPermission{"syslog_drain", "route_forwarding"},
						Tags:     serviceTags,
						Plans: []domain.ServicePlan{
							{
								ID:          dedicatedPlanID,
								Name:        dedicatedPlanName,
								Description: dedicatedPlanDescription,
								Free:        &trueVar,
								Bindable:    &trueVar,
								Metadata: &domain.ServicePlanMetadata{
									Bullets:     dedicatedPlanBullets,
									DisplayName: dedicatedPlanDisplayName,
									Costs: []domain.ServicePlanCost{
										{
											Unit:   dedicatedPlanCostUnit,
											Amount: dedicatedPlanCostAmount,
										},
									},
									AdditionalMetadata: map[string]interface{}{
										"yo": "bill",
									},
								},
								Schemas: &defaultSchemas,
								MaintenanceInfo: &domain.MaintenanceInfo{
									Public: map[string]string{
										"name":             "gloria",
										"stemcell_version": "1234",
									},
									Private:     "hashed-secret-plan_value;",
									Version:     "1.2.3+plan",
									Description: "Plan description",
								},
							},
							{
								ID:          highMemoryPlanID,
								Name:        highMemoryPlanName,
								Description: highMemoryPlanDescription,
								Metadata: &domain.ServicePlanMetadata{
									Bullets:     highMemoryPlanBullets,
									DisplayName: highMemoryPlanDisplayName,
								},
								Schemas: &defaultSchemas,
								MaintenanceInfo: &domain.MaintenanceInfo{
									Public: map[string]string{
										"name": "jorge",
									},
									Private:     "hashed-secret-global_value;",
									Version:     "1.2.4+global",
									Description: "Global description",
								},
							},
						},
					},
				},
			}))
		})
	})

	When("GeneratePlanSchemas returns an error", func() {
		var (
			serviceCatalogConfig brokerConfig.ServiceOffering
		)

		BeforeEach(func() {
			serviceCatalogConfig = defaultServiceCatalogConfig()
			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
					EnablePlanSchemas: true,
				},
				ServiceCatalog: serviceCatalogConfig,
			}

			StartServer(conf)
		})

		It("fails with 500 status code", func() {
			fakeCommandRunner.RunWithInputParamsReturns(nil, nil, nil, errors.New("oops"))
			response, bodyContent := doCatalogRequest()

			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			Expect(bodyContent).To(ContainSubstring("oops"))
		})

		It("fails with a proper message if not implemented", func() {
			errCode := sdk.NotImplementedExitCode
			fakeCommandRunner.RunWithInputParamsReturns([]byte{}, []byte{}, &errCode, nil)

			response, bodyContent := doCatalogRequest()

			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			Expect(string(bodyContent)).To(ContainSubstring("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas"))
		})
	})

	Context("without version header", func() {
		BeforeEach(func() {
			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
					EnablePlanSchemas: true,
				},
			}

			StartServer(conf)
		})

		It("fails with 412 status code", func() {
			response, bodyContent := doRequestWithAuthAndHeaderSet(
				http.MethodGet,
				fmt.Sprintf("http://%s/v2/catalog", serverURL),
				nil,
				func(r *http.Request) {
					r.Header.Del("X-Broker-API-Version")
				})

			Expect(response.StatusCode).To(Equal(http.StatusPreconditionFailed))
			Expect(string(bodyContent)).To(ContainSubstring(`{"Description":"X-Broker-API-Version Header not set"}`))
		})
	})

	Context("without authentication", func() {
		BeforeEach(func() {
			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
					EnablePlanSchemas: true,
				},
			}

			StartServer(conf)
		})

		It("fails with 401 status code", func() {
			response, _ := doRequestWithoutAuth(
				http.MethodGet,
				fmt.Sprintf("http://%s/v2/catalog", serverURL),
				nil,
				func(r *http.Request) {
					r.Header.Set("X-Broker-API-Version", "2.14")
				})

			Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
		})
	})
})

func doCatalogRequest() (*http.Response, []byte) {
	return doRequestWithAuthAndHeaderSet(http.MethodGet, fmt.Sprintf("http://%s/v2/catalog", serverURL), nil)
}

func defaultServiceCatalogConfig() brokerConfig.ServiceOffering {
	return brokerConfig.ServiceOffering{
		ID:            serviceID,
		Name:          serviceName,
		Description:   serviceDescription,
		Bindable:      serviceBindable,
		PlanUpdatable: servicePlanUpdatable,
		Metadata: brokerConfig.ServiceMetadata{
			DisplayName:         serviceMetadataDisplayName,
			ImageURL:            serviceMetadataImageURL,
			LongDescription:     serviceMetaDataLongDescription,
			ProviderDisplayName: serviceMetaDataProviderDisplayName,
			DocumentationURL:    serviceMetaDataDocumentationURL,
			SupportURL:          serviceMetaDataSupportURL,
			Shareable:           serviceMetaDataShareable,
		},
		DashboardClient: &brokerConfig.DashboardClient{
			ID:          "client-id-1",
			Secret:      "secret-1",
			RedirectUri: "https://dashboard.url",
		},
		Tags: serviceTags,
		GlobalProperties: sdk.Properties{
			"global_property": "global_value",
		},
		GlobalQuotas: brokerConfig.Quotas{},
		Plans: []brokerConfig.Plan{
			{
				Name:        dedicatedPlanName,
				ID:          dedicatedPlanID,
				Description: dedicatedPlanDescription,
				Free:        &trueVar,
				Bindable:    &trueVar,
				Update:      dedicatedPlanUpdateBlock,
				Metadata: brokerConfig.PlanMetadata{
					DisplayName: dedicatedPlanDisplayName,
					Bullets:     dedicatedPlanBullets,
					Costs: []brokerConfig.PlanCost{
						{
							Amount: dedicatedPlanCostAmount,
							Unit:   dedicatedPlanCostUnit,
						},
					},
					AdditionalMetadata: map[string]interface{}{
						"foo": "bar",
					},
				},
				Quotas: brokerConfig.Quotas{
					ServiceInstanceLimit: &dedicatedPlanQuota,
				},
				Properties: sdk.Properties{
					"type": "dedicated-plan-property",
				},
				InstanceGroups: []sdk.InstanceGroup{
					{
						Name:               "instance-group-name",
						VMType:             dedicatedPlanVMType,
						VMExtensions:       dedicatedPlanVMExtensions,
						PersistentDiskType: dedicatedPlanDisk,
						Instances:          dedicatedPlanInstances,
						Networks:           dedicatedPlanNetworks,
						AZs:                dedicatedPlanAZs,
					},
					{
						Name:               "instance-group-errand",
						Lifecycle:          "errand",
						VMType:             dedicatedPlanVMType,
						PersistentDiskType: dedicatedPlanDisk,
						Instances:          dedicatedPlanInstances,
						Networks:           dedicatedPlanNetworks,
						AZs:                dedicatedPlanAZs,
					},
				},
			},
			{
				Name:        highMemoryPlanName,
				ID:          highMemoryPlanID,
				Description: highMemoryPlanDescription,
				Metadata: brokerConfig.PlanMetadata{
					DisplayName: highMemoryPlanDisplayName,
					Bullets:     highMemoryPlanBullets,
				},
				Properties: sdk.Properties{
					"type":            "high-memory-plan-property",
					"global_property": "overrides_global_value",
				},
				InstanceGroups: []sdk.InstanceGroup{
					{
						Name:         "instance-group-name",
						VMType:       highMemoryPlanVMType,
						VMExtensions: highMemoryPlanVMExtensions,
						Instances:    highMemoryPlanInstances,
						Networks:     highMemoryPlanNetworks,
						AZs:          highMemoryPlanAZs,
					},
				},
			},
		},
	}
}
