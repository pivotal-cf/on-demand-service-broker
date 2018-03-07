package broker_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/noopservicescontroller"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

var _ = Describe("Catalog", func() {
	var (
		createSchema, updateSchema, bindingSchema brokerapi.Schema
	)

	BeforeEach(func() {
		createSchema = brokerapi.Schema{
			Parameters: map[string]interface{}{
				"$schema": "http://json-schema.org/draft-04/schema#",
				"type":    "object",
				"properties": map[string]interface{}{
					"create-prop": map[string]interface{}{
						"description": "create prop",
						"type":        "integer",
						"required":    false,
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
						"type":        "bool",
						"required":    true,
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
						"type":        "bool",
						"required":    false,
					},
				},
			},
		}
	})

	It("generates the catalog response if the adapter does not implement generate-plan-schemas", func() {
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

		Expect(logBuffer.String()).To(ContainSubstring("the service adapter does not implement generate-plan-schemas"))
	})

	It("fails if the adapter returns an error when generating plan schemas", func() {
		serviceAdapter.GeneratePlanSchemaReturns(brokerapi.ServiceSchemas{}, errors.New("oops"))
		b, brokerCreationErr = createBroker([]broker.StartupChecker{}, noopservicescontroller.New())
		Expect(brokerCreationErr).NotTo(HaveOccurred())

		contextWithoutRequestID := context.Background()
		_, err := b.Services(contextWithoutRequestID)
		Expect(err).To(MatchError("oops"))
		Expect(logBuffer.String()).NotTo(ContainSubstring("the service adapter does not implement generate-plan-schemas"))
	})

	Context("a plan includes a cost", func() {
		It("includes the cost in the catalog", func() {
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
	})

	Context("a plan includes a dashboard", func() {
		It("includes the dashboard in the catalog", func() {
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
