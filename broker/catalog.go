// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"context"

	"github.com/pivotal-cf/brokerapi"
)

func (b *Broker) Services(_ context.Context) []brokerapi.Service {
	var servicePlans []brokerapi.ServicePlan
	for _, plan := range b.serviceOffering.Plans {
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
				DisplayName: plan.Metadata.DisplayName,
				Bullets:     plan.Metadata.Bullets,
				Costs:       planCosts,
			},
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
			},
			DashboardClient: dashboardClient,
			Requires:        requiredPermissions(b.serviceOffering.Requires),
			Tags:            b.serviceOffering.Tags,
		},
	}
}

func requiredPermissions(permissions []string) []brokerapi.RequiredPermission {
	var brokerPermissions []brokerapi.RequiredPermission
	for _, permission := range permissions {
		brokerPermissions = append(brokerPermissions, brokerapi.RequiredPermission(permission))
	}
	return brokerPermissions
}
