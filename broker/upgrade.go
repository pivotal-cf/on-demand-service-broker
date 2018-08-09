// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"context"
	"errors"
	"fmt"
	"log"

	"encoding/json"

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

func (b *Broker) Upgrade(ctx context.Context, instanceID string, details brokerapi.UpdateDetails, logger *log.Logger) (OperationData, error) {
	b.deploymentLock.Lock()
	defer b.deploymentLock.Unlock()

	logger.Printf("upgrading instance %s", instanceID)

	if details.PlanID == "" {
		return OperationData{}, b.processError(errors.New("no plan ID provided in upgrade request body"), logger)
	}

	plan, found := b.serviceOffering.FindPlanByID(details.PlanID)
	if !found {
		logger.Printf("error: finding plan ID %s", details.PlanID)
		return OperationData{}, b.processError(fmt.Errorf("plan %s not found", details.PlanID), logger)
	}

	var boshContextID string

	if plan.LifecycleErrands != nil {
		boshContextID = uuid.New()
	}

	if b.EnablePlanSchemas {
		schemas, _ := b.adapterClient.GeneratePlanSchema(plan.AdapterPlan(b.serviceOffering.GlobalProperties), logger)
		instanceUpgradeSchema := schemas.Instance.Update

		validator := NewValidator(instanceUpgradeSchema.Parameters)
		err := validator.ValidateSchema()
		if err != nil {
			return OperationData{}, err
		}

		params := make(map[string]interface{})
		err = json.Unmarshal(details.RawParameters, &params)
		if err != nil && len(details.RawParameters) > 0 {
			return OperationData{}, fmt.Errorf("provision request params are malformed: %s", details.RawParameters)
		}

		err = validator.ValidateParams(params)
		if err != nil {
			return OperationData{}, err
		}
	}

	taskID, _, err := b.deployer.Upgrade(
		deploymentName(instanceID),
		details.PlanID,
		&details.PlanID,
		boshContextID,
		logger,
	)

	if err != nil {
		logger.Printf("error upgrading instance %s: %s", instanceID, err)

		switch err := err.(type) {
		case serviceadapter.UnknownFailureError:
			return OperationData{}, b.processError(adapterToAPIError(ctx, err), logger)
		case TaskInProgressError:
			return OperationData{}, b.processError(NewOperationInProgressError(err), logger)
		default:
			return OperationData{}, b.processError(err, logger)
		}
	}

	return OperationData{
		BoshContextID: boshContextID,
		BoshTaskID:    taskID,
		OperationType: OperationTypeUpgrade,
		Errands:       plan.PostDeployErrands(),
	}, nil
}
