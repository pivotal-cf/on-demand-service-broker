// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"context"
	"fmt"
	"log"

	"github.com/pivotal-cf/brokerapi/v9/domain"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pkg/errors"
)

func (b *Broker) Services(ctx context.Context) ([]domain.Service, error) {
	b.catalogLock.Lock()
	defer b.catalogLock.Unlock()

	if b.cachedCatalog != nil {
		return b.cachedCatalog, nil
	}

	logger := b.loggerFactory.NewWithContext(ctx)

	var servicePlans []domain.ServicePlan
	for _, plan := range b.serviceOffering.Plans {
		servicePlan, err := b.generateServicePlan(plan, logger)
		if err != nil {
			return []domain.Service{}, err
		}
		servicePlans = append(servicePlans, servicePlan)
	}

	b.cachedCatalog = []domain.Service{
		{
			ID:            b.serviceOffering.ID,
			Name:          b.serviceOffering.Name,
			Description:   b.serviceOffering.Description,
			Bindable:      b.serviceOffering.Bindable,
			PlanUpdatable: b.serviceOffering.PlanUpdatable,
			Plans:         servicePlans,
			Metadata: &domain.ServiceMetadata{
				DisplayName:         b.serviceOffering.Metadata.DisplayName,
				ImageUrl:            b.serviceOffering.Metadata.ImageURL,
				LongDescription:     b.serviceOffering.Metadata.LongDescription,
				ProviderDisplayName: b.serviceOffering.Metadata.ProviderDisplayName,
				DocumentationUrl:    b.serviceOffering.Metadata.DocumentationURL,
				SupportUrl:          b.serviceOffering.Metadata.SupportURL,
				Shareable:           &b.serviceOffering.Metadata.Shareable,
				AdditionalMetadata:  b.serviceOffering.Metadata.AdditionalMetadata,
			},
			DashboardClient: b.generateDashboardClient(),
			Requires:        requiredPermissions(b.serviceOffering.Requires),
			Tags:            b.serviceOffering.Tags,
		},
	}
	return b.cachedCatalog, nil
}

func (b *Broker) generateDashboardClient() *domain.ServiceDashboardClient {
	var dashboardClient *domain.ServiceDashboardClient
	if b.serviceOffering.DashboardClient != nil {
		dashboardClient = &domain.ServiceDashboardClient{
			ID:          b.serviceOffering.DashboardClient.ID,
			Secret:      b.serviceOffering.DashboardClient.Secret,
			RedirectURI: b.serviceOffering.DashboardClient.RedirectUri,
		}
	}
	return dashboardClient
}

func (b *Broker) generateServicePlan(plan config.Plan, logger *log.Logger) (domain.ServicePlan, error) {
	maintenanceInfo := b.generateMaintenanceInfo(plan)

	var planCosts []domain.ServicePlanCost
	for _, cost := range plan.Metadata.Costs {
		planCosts = append(planCosts, domain.ServicePlanCost{Amount: cost.Amount, Unit: cost.Unit})
	}

	planSchema, err := b.generatePlanSchemas(plan, logger)
	if err != nil {
		return domain.ServicePlan{}, err
	}

	return domain.ServicePlan{
		ID:          plan.ID,
		Name:        plan.Name,
		Description: plan.Description,
		Free:        plan.Free,
		Bindable:    plan.Bindable,
		Metadata: &domain.ServicePlanMetadata{
			DisplayName:        plan.Metadata.DisplayName,
			Bullets:            plan.Metadata.Bullets,
			Costs:              planCosts,
			AdditionalMetadata: plan.Metadata.AdditionalMetadata,
		},
		MaintenanceInfo: maintenanceInfo,
		Schemas:         planSchema,
	}, nil
}

func (b *Broker) generateMaintenanceInfo(plan config.Plan) *domain.MaintenanceInfo {
	var maintenanceInfo *domain.MaintenanceInfo
	mergedPublic, mergedPrivate := mergeMaintenanceInfo(b.serviceOffering.MaintenanceInfo, plan.MaintenanceInfo)
	version := getMaintenanceInfoVersion(b.serviceOffering.MaintenanceInfo, plan.MaintenanceInfo)
	description := getMaintenanceInfoDescription(b.serviceOffering.MaintenanceInfo, plan.MaintenanceInfo)

	if mergedPublic != nil || mergedPrivate != nil || version != "" {
		maintenanceInfo = &domain.MaintenanceInfo{
			Public:      mergedPublic,
			Private:     b.hasher.Hash(mergedPrivate),
			Version:     version,
			Description: description,
		}
	}
	return maintenanceInfo
}

func (b *Broker) generatePlanSchemas(plan config.Plan, logger *log.Logger) (*domain.ServiceSchemas, error) {
	if b.EnablePlanSchemas {
		planSchema, err := b.adapterClient.GeneratePlanSchema(plan.AdapterPlan(b.serviceOffering.GlobalProperties), logger)
		if err != nil {
			if _, ok := err.(serviceadapter.NotImplementedError); !ok {
				return nil, err
			}
			logger.Println("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas")
			return nil, fmt.Errorf("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas")
		}

		err = validatePlanSchemas(planSchema)
		if err != nil {
			logger.Println(fmt.Sprintf("Invalid JSON Schema for plan %s: %s\n", plan.Name, err.Error()))
			return nil, errors.Wrap(err, "Invalid JSON Schema for plan "+plan.Name)
		}
		return &planSchema, nil
	}
	return nil, nil
}

func mergeMaintenanceInfo(globalInfo *config.MaintenanceInfo, planInfo *config.MaintenanceInfo) (map[string]string, map[string]string) {
	if globalInfo == nil && planInfo == nil {
		return nil, nil
	}

	publicMap := make(map[string]string)
	privateMap := make(map[string]string)

	if globalInfo != nil {
		copyMap(publicMap, globalInfo.Public)
		copyMap(privateMap, globalInfo.Private)
	}

	if planInfo != nil {
		copyMap(publicMap, planInfo.Public)
		copyMap(privateMap, planInfo.Private)
	}

	return normalize(publicMap), normalize(privateMap)
}

func copyMap(dst, src map[string]string) {
	for key, value := range src {
		dst[key] = value
	}
}

func normalize(keyMap map[string]string) map[string]string {
	if len(keyMap) == 0 {
		return nil
	}
	return keyMap
}

func getMaintenanceInfoVersion(globalInfo *config.MaintenanceInfo, planInfo *config.MaintenanceInfo) string {
	if planInfo != nil && planInfo.Version != "" {
		return planInfo.Version
	}

	if globalInfo != nil {
		return globalInfo.Version
	}

	return ""
}

func getMaintenanceInfoDescription(globalInfo *config.MaintenanceInfo, planInfo *config.MaintenanceInfo) string {
	if planInfo != nil && planInfo.Description != "" {
		return planInfo.Description
	}

	if globalInfo != nil {
		return globalInfo.Description
	}

	return ""
}

func requiredPermissions(permissions []string) []domain.RequiredPermission {
	var brokerPermissions []domain.RequiredPermission
	for _, permission := range permissions {
		brokerPermissions = append(brokerPermissions, domain.RequiredPermission(permission))
	}
	return brokerPermissions
}

func validatePlanSchemas(planSchema domain.ServiceSchemas) error {
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
