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

package broker_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/noopservicescontroller"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

var _ = Describe("Catalog", func() {
	var (
		createSchema, updateSchema, bindingSchema                brokerapi.Schema
		invalidTypeSchema, invalidVersionSchema, noVersionSchema brokerapi.Schema
	)

	setSchemas := func() {
		createSchema = brokerapi.Schema{
			Parameters: map[string]interface{}{
				"$schema": "http://json-schema.org/draft-04/schema#",
				"type":    "object",
				"properties": map[string]interface{}{
					"create-prop": map[string]interface{}{
						"description": "create prop",
						"type":        "integer",
					},
				},
			},
		}
		updateSchema = brokerapi.Schema{
			Parameters: map[string]interface{}{
				"$schema": "http://json-schema.org/draft-04/schema#",
				"type":    "object",
				"properties": map[string]interface{}{
					"some-update-prop": map[string]interface{}{
						"description": "some update prop create topics",
						"type":        "boolean",
					},
				},
			},
		}
		bindingSchema = brokerapi.Schema{
			Parameters: map[string]interface{}{
				"$schema": "http://json-schema.org/draft-04/schema#",
				"type":    "object",
				"properties": map[string]interface{}{
					"binding-prop": map[string]interface{}{
						"description": "binding",
						"type":        "boolean",
					},
				},
			},
		}
		noVersionSchema = brokerapi.Schema{
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"some-prop": map[string]interface{}{
						"description": "create prop",
						"type":        "string",
					},
				},
			},
		}
		invalidVersionSchema = brokerapi.Schema{
			Parameters: map[string]interface{}{
				"$schema": "http://json-schema.org/draft-03/schema#",
				"type":    "object",
				"properties": map[string]interface{}{
					"some-prop": map[string]interface{}{
						"description": "create prop",
						"type":        "string",
					},
				},
			},
		}
		invalidTypeSchema = brokerapi.Schema{
			Parameters: map[string]interface{}{
				"$schema": "http://json-schema.org/draft-04/schema#",
				"type":    "object",
				"properties": map[string]interface{}{
					"some-prop": map[string]interface{}{
						"description": "create prop",
						"type":        "fool",
					},
				},
			},
		}
	}

	setSchemas()

	BeforeEach(func() {
		setSchemas()
	})

	It("generates the catalog response", func() {
		serviceAdapter.GeneratePlanSchemaReturns(brokerapi.ServiceSchemas{}, serviceadapter.NewNotImplementedError("not implemented"))
		b, brokerCreationErr = createBroker([]broker.StartupChecker{}, noopservicescontroller.New())
		Expect(brokerCreationErr).NotTo(HaveOccurred())

		contextWithoutRequestID := context.Background()
		services, err := b.Services(contextWithoutRequestID)
		Expect(err).ToNot(HaveOccurred())

		plans := getPlansFromCatalog(serviceCatalog)

		Expect(services).To(Equal([]brokerapi.Service{
			{
				ID:            serviceCatalog.ID,
				Name:          serviceCatalog.Name,
				Description:   serviceCatalog.Description,
				Bindable:      serviceCatalog.Bindable,
				PlanUpdatable: serviceCatalog.PlanUpdatable,
				Metadata: &brokerapi.ServiceMetadata{
					DisplayName:         serviceCatalog.Metadata.DisplayName,
					ImageUrl:            serviceCatalog.Metadata.DisplayName,
					LongDescription:     serviceCatalog.Metadata.LongDescription,
					ProviderDisplayName: serviceCatalog.Metadata.ProviderDisplayName,
					DocumentationUrl:    serviceCatalog.Metadata.DocumentationURL,
					SupportUrl:          serviceCatalog.Metadata.SupportURL,
					Shareable:           &serviceCatalog.Metadata.Shareable,
				},
				DashboardClient: nil,
				Tags:            serviceCatalog.Tags,
				Plans:           plans,
			},
		}))

		Expect(serviceAdapter.GeneratePlanSchemaCallCount()).To(BeZero())
	})

	It("includes the plan cost", func() {
		serviceCatalog.Plans[0].Metadata.Costs = []config.PlanCost{
			{Unit: "dogecoins", Amount: map[string]float64{"value": 1.65}},
		}
		b, brokerCreationErr = createBroker([]broker.StartupChecker{}, noopservicescontroller.New())
		Expect(brokerCreationErr).NotTo(HaveOccurred())

		contextWithoutRequestID := context.Background()
		services, err := b.Services(contextWithoutRequestID)
		Expect(err).ToNot(HaveOccurred())

		Expect(services[0].Plans[0].Metadata.Costs).To(Equal(
			[]brokerapi.ServicePlanCost{
				{Amount: map[string]float64{"value": 1.65}, Unit: "dogecoins"},
			},
		))
	})

	It("includes the plan dashboard", func() {
		serviceCatalog.DashboardClient = &config.DashboardClient{
			ID:          "super-id",
			Secret:      "super-secret",
			RedirectUri: "super-uri",
		}

		b, brokerCreationErr = createBroker([]broker.StartupChecker{}, noopservicescontroller.New())
		Expect(brokerCreationErr).NotTo(HaveOccurred())

		contextWithoutRequestID := context.Background()
		services, err := b.Services(contextWithoutRequestID)
		Expect(err).ToNot(HaveOccurred())

		Expect(*services[0].DashboardClient).To(Equal(
			brokerapi.ServiceDashboardClient{
				ID:          "super-id",
				Secret:      "super-secret",
				RedirectURI: "super-uri",
			},
		))
	})

	It("includes arbitrary fields", func() {
		serviceCatalog.Metadata.AdditionalMetadata = map[string]interface{}{
			"random": "george",
		}
		serviceCatalog.Plans[0].Metadata.AdditionalMetadata = map[string]interface{}{
			"arbitrary": "bill",
		}
		b, brokerCreationErr = createBroker([]broker.StartupChecker{}, noopservicescontroller.New())
		Expect(brokerCreationErr).NotTo(HaveOccurred())

		contextWithoutRequestID := context.Background()
		services, err := b.Services(contextWithoutRequestID)
		Expect(err).ToNot(HaveOccurred())

		Expect(services[0].Plans[0].Metadata.AdditionalMetadata).To(Equal(
			map[string]interface{}{
				"arbitrary": "bill",
			},
		))
		Expect(services[0].Metadata.AdditionalMetadata).To(Equal(
			map[string]interface{}{
				"random": "george",
			},
		))
	})

	It("for each plan, includes maintenance_info", func() {

		serviceCatalog = config.ServiceOffering{
			ID: serviceOfferingID,
			MaintenanceInfo: &config.MaintenanceInfo{
				Public: map[string]string{
					"name":    "yuliana",
					"vm_type": "small",
				},
			},
			Plans: []config.Plan{
				{
					ID: "1",
					MaintenanceInfo: &config.MaintenanceInfo{
						Public: map[string]string{
							"name":             "alberto",
							"stemcell_version": "1234",
						},
					},
				}, {
					ID: "2",
				},
			},
		}

		b, brokerCreationErr = createBroker([]broker.StartupChecker{}, noopservicescontroller.New())
		Expect(brokerCreationErr).NotTo(HaveOccurred())

		contextWithoutRequestID := context.Background()
		services, err := b.Services(contextWithoutRequestID)
		Expect(err).ToNot(HaveOccurred())

		Expect(services[0].Plans[0].MaintenanceInfo.Public).To(SatisfyAll(
			HaveKeyWithValue("name", "alberto"),
			HaveKeyWithValue("vm_type", "small"),
			HaveKeyWithValue("stemcell_version", "1234"),
		))

		Expect(services[0].Plans[1].MaintenanceInfo.Public).To(SatisfyAll(
			HaveKeyWithValue("name", "yuliana"),
			HaveKeyWithValue("vm_type", "small"),
		))
	})

	It("for each plan, calls the adapter to generate the plan schemas", func() {
		planSchema := brokerapi.ServiceSchemas{
			Instance: brokerapi.ServiceInstanceSchema{
				Create: createSchema,
				Update: updateSchema,
			},
			Binding: brokerapi.ServiceBindingSchema{
				Create: bindingSchema,
			},
		}

		serviceAdapter.GeneratePlanSchemaReturns(planSchema, nil)
		b, brokerCreationErr = createBroker([]broker.StartupChecker{}, noopservicescontroller.New())
		b.EnablePlanSchemas = true

		Expect(brokerCreationErr).NotTo(HaveOccurred())

		contextWithoutRequestID := context.Background()
		services, err := b.Services(contextWithoutRequestID)
		Expect(err).ToNot(HaveOccurred())

		Expect(serviceAdapter.GeneratePlanSchemaCallCount()).To(Equal(len(services[0].Plans)))
		for _, service := range services {
			for _, plan := range service.Plans {
				Expect(*plan.Schemas).To(Equal(brokerapi.ServiceSchemas{
					Instance: brokerapi.ServiceInstanceSchema{
						Create: brokerapi.Schema{Parameters: createSchema.Parameters},
						Update: brokerapi.Schema{Parameters: updateSchema.Parameters},
					},
					Binding: brokerapi.ServiceBindingSchema{
						Create: brokerapi.Schema{Parameters: bindingSchema.Parameters},
					},
				}))
			}
		}
	})

	DescribeTable("when the generated schema is invalid",
		func(create, update, binding brokerapi.Schema, errorLabel string) {
			planSchema := brokerapi.ServiceSchemas{
				Instance: brokerapi.ServiceInstanceSchema{
					Create: create,
					Update: update,
				},
				Binding: brokerapi.ServiceBindingSchema{
					Create: binding,
				},
			}

			serviceAdapter.GeneratePlanSchemaReturns(planSchema, nil)
			b, brokerCreationErr = createBroker([]broker.StartupChecker{}, noopservicescontroller.New())
			b.EnablePlanSchemas = true

			Expect(brokerCreationErr).NotTo(HaveOccurred())

			contextWithoutRequestID := context.Background()
			_, err := b.Services(contextWithoutRequestID)
			Expect(err).To(MatchError(
				SatisfyAll(
					ContainSubstring("Invalid JSON Schema for plan"),
					ContainSubstring(errorLabel),
				),
			))
			Expect(logBuffer.String()).To(
				SatisfyAll(
					ContainSubstring("Invalid JSON Schema for plan"),
					ContainSubstring(errorLabel),
				),
			)
		},

		Entry("invalid type on instance.create", invalidTypeSchema, updateSchema, bindingSchema, "instance create"),
		Entry("invalid type on instance.update", createSchema, invalidTypeSchema, bindingSchema, "instance update"),
		Entry("invalid type on binding.create", createSchema, updateSchema, invalidTypeSchema, "binding create"),
		Entry("invalid version", invalidVersionSchema, updateSchema, bindingSchema, "instance create"),
		Entry("no version specified", noVersionSchema, updateSchema, bindingSchema, "instance create"),
		Entry("missing schemas", createSchema, nil, bindingSchema, "instance update"),
	)

	It("fails if the adapter returns an error when generating plan schemas", func() {
		serviceAdapter.GeneratePlanSchemaReturns(brokerapi.ServiceSchemas{}, serviceadapter.NewNotImplementedError("not implemented"))
		b, brokerCreationErr = createBroker([]broker.StartupChecker{}, noopservicescontroller.New())
		b.EnablePlanSchemas = true
		Expect(brokerCreationErr).NotTo(HaveOccurred())

		contextWithoutRequestID := context.Background()
		_, err := b.Services(contextWithoutRequestID)
		Expect(err).To(MatchError(ContainSubstring("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas")))
		Expect(logBuffer.String()).To(ContainSubstring("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas"))
	})
})

func getPlansFromCatalog(serviceCatalog config.ServiceOffering) []brokerapi.ServicePlan {
	var servicePlans []brokerapi.ServicePlan
	for _, plan := range serviceCatalog.Plans {
		servicePlans = append(servicePlans, brokerapi.ServicePlan{
			ID:          plan.ID,
			Name:        plan.Name,
			Description: plan.Description,
			Free:        plan.Free,
			Bindable:    plan.Bindable,
			Metadata: &brokerapi.ServicePlanMetadata{
				Bullets:     plan.Metadata.Bullets,
				DisplayName: plan.Metadata.DisplayName,
			},
		})
	}
	return servicePlans
}
