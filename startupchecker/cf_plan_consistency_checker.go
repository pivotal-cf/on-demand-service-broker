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

package startupchecker

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

type CFPlanConsistencyChecker struct {
	cfClient        ServiceInstanceCounter
	serviceOffering config.ServiceOffering
	logger          *log.Logger
}

func NewCFPlanConsistencyChecker(cfClient ServiceInstanceCounter, serviceOffering config.ServiceOffering, logger *log.Logger) *CFPlanConsistencyChecker {
	return &CFPlanConsistencyChecker{
		cfClient:        cfClient,
		serviceOffering: serviceOffering,
		logger:          logger,
	}
}

func (c *CFPlanConsistencyChecker) Check() error {
	instanceCountByPlanID, err := c.cfClient.CountInstancesOfServiceOffering(c.serviceOffering.ID, c.logger)
	if err != nil {
		return err
	}

	for plan, count := range instanceCountByPlanID {
		_, found := c.serviceOffering.Plans.FindByID(plan.ServicePlanEntity.UniqueID)

		if !found && count > 0 {
			return fmt.Errorf(
				"plan %s (%s) was expected but is now missing. You cannot remove or change the plan_id of a plan which has existing service instances",
				plan.ServicePlanEntity.Name,
				plan.ServicePlanEntity.UniqueID,
			)
		}
	}

	return nil
}

//go:generate counterfeiter -o fakes/fake_service_instance_counter.go . ServiceInstanceCounter
type ServiceInstanceCounter interface {
	CountInstancesOfServiceOffering(serviceOfferingID string, logger *log.Logger) (instanceCountByPlanID map[cf.ServicePlan]int, err error)
}
