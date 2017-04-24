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

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

func (b *Broker) Instances(logger *log.Logger) ([]string, error) {
	instanceIDs, err := b.cfClient.GetInstancesOfServiceOffering(b.serviceOffering.ID, logger)
	if err != nil {
		logger.Printf("error listing instances: %s", err)
		return nil, err
	}

	return instanceIDs, nil
}

func (b *Broker) validatePlanQuota(ctx context.Context, serviceID string, plan config.Plan, logger *log.Logger) DisplayableError {
	if plan.Quotas.ServiceInstanceLimit == nil {
		return NilError
	}
	limit := *plan.Quotas.ServiceInstanceLimit

	count, err := b.cfClient.CountInstancesOfPlan(serviceID, plan.ID, logger)
	if err != nil {
		return NewGenericError(ctx, fmt.Errorf("error counting instances of plan: %s", err))
	}

	if count >= limit {
		return NewDisplayableError(brokerapi.ErrPlanQuotaExceeded, fmt.Errorf("plan quota exceeded for plan ID %s", plan.ID))
	}

	return NilError
}
