// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"context"
	"fmt"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pkg/errors"
)

func (b *Broker) Services(ctx context.Context) ([]brokerapi.Service, error) {
	logger := b.loggerFactory.NewWithContext(ctx)
	var servicePlans []brokerapi.ServicePlan

	for _, plan := range b.serviceOffering.Plans {
		mergedMaintenanceInfo := mergeMaintenanceInfo(b.serviceOffering.MaintenanceInfo, plan.MaintenanceInfo)

		var planCosts []brokerapi.ServicePlanCost
		for _, cost := range plan.Metadata.Costs {
			planCosts = append(planCosts, brokerapi.ServicePlanCost{Amount: cost.Amount, Unit: cost.Unit})
		}

		servicePlan := brokerapi.ServicePlan{
			ID:          plan.ID,
			Name:        plan.Name,
			Description: plan.Description,
			Free:        plan.Free,
			Bindable:    plan.Bindable,
			Metadata: &brokerapi.ServicePlanMetadata{
				DisplayName:        plan.Metadata.DisplayName,
				Bullets:            plan.Metadata.Bullets,
				Costs:              planCosts,
				AdditionalMetadata: plan.Metadata.AdditionalMetadata,
			},
			MaintenanceInfo: mergedMaintenanceInfo,
		}

		if b.EnablePlanSchemas {
			planSchema, err := b.adapterClient.GeneratePlanSchema(plan.AdapterPlan(b.serviceOffering.GlobalProperties), logger)
			if err != nil {
				if _, ok := err.(serviceadapter.NotImplementedError); !ok {
					return []brokerapi.Service{}, err
				}
				logger.Println("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas")
				return []brokerapi.Service{}, fmt.Errorf("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas")
			}

			err = validatePlanSchemas(planSchema)
			if err != nil {
				logger.Println(fmt.Sprintf("Invalid JSON Schema for plan %s: %s\n", plan.Name, err.Error()))
				return []brokerapi.Service{}, errors.Wrap(err, "Invalid JSON Schema for plan "+plan.Name)
			}

			servicePlan.Schemas = &planSchema
		}

		servicePlans = append(servicePlans, servicePlan)
	}

	var dashboardClient *brokerapi.ServiceDashboardClient
	if b.serviceOffering.DashboardClient != nil {
		dashboardClient = &brokerapi.ServiceDashboardClient{
			ID:          b.serviceOffering.DashboardClient.ID,
			Secret:      b.serviceOffering.DashboardClient.Secret,
			RedirectURI: b.serviceOffering.DashboardClient.RedirectUri,
		}
	}

	return []brokerapi.Service{
		{
			ID:            b.serviceOffering.ID,
			Name:          b.serviceOffering.Name,
			Description:   b.serviceOffering.Description,
			Bindable:      b.serviceOffering.Bindable,
			PlanUpdatable: b.serviceOffering.PlanUpdatable,
			Plans:         servicePlans,
			Metadata: &brokerapi.ServiceMetadata{
				DisplayName:         b.serviceOffering.Metadata.DisplayName,
				ImageUrl:            b.serviceOffering.Metadata.ImageURL,
				LongDescription:     b.serviceOffering.Metadata.LongDescription,
				ProviderDisplayName: b.serviceOffering.Metadata.ProviderDisplayName,
				DocumentationUrl:    b.serviceOffering.Metadata.DocumentationURL,
				SupportUrl:          b.serviceOffering.Metadata.SupportURL,
				Shareable:           &b.serviceOffering.Metadata.Shareable,
				AdditionalMetadata:  b.serviceOffering.Metadata.AdditionalMetadata,
			},
			DashboardClient: dashboardClient,
			Requires:        requiredPermissions(b.serviceOffering.Requires),
			Tags:            b.serviceOffering.Tags,
		},
	}, nil
}

func mergeMaintenanceInfo(globalInfo config.MaintenanceInfo, planInfo config.MaintenanceInfo) brokerapi.MaintenanceInfo {
	ret := brokerapi.MaintenanceInfo{
		Public: make(map[string]string),
	}

	for key, value := range globalInfo.Public {
		ret.Public[key] = value
	}

	// Duplicates will overwrite the global info
	for key, value := range planInfo.Public {
		ret.Public[key] = value
	}

	if len(ret.Public) == 0 {
		ret.Public = nil
	}
	return ret
}

func requiredPermissions(permissions []string) []brokerapi.RequiredPermission {
	var brokerPermissions []brokerapi.RequiredPermission
	for _, permission := range permissions {
		brokerPermissions = append(brokerPermissions, brokerapi.RequiredPermission(permission))
	}
	return brokerPermissions
}

func validatePlanSchemas(planSchema brokerapi.ServiceSchemas) error {
	labels := []string{"instance create", "instance update", "binding create"}
	for i, schema := range []map[string]interface{}{
		planSchema.Instance.Create.Parameters,
		planSchema.Instance.Update.Parameters,
		planSchema.Binding.Create.Parameters,
	} {
		validator := NewValidator(schema)
		err := validator.ValidateSchema()
		if err != nil {
			return fmt.Errorf("Error validating plan schemas for %s - %s", labels[i], err.Error())
		}

	}
	return nil
}
