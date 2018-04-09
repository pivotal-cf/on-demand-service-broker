// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/task"
)

func (b *Broker) Update(
	ctx context.Context,
	instanceID string,
	details brokerapi.UpdateDetails,
	asyncAllowed bool,
) (brokerapi.UpdateServiceSpec, error) {
	b.deploymentLock.Lock()
	defer b.deploymentLock.Unlock()

	requestID := uuid.New()
	ctx = brokercontext.New(ctx, string(OperationTypeUpdate), requestID, b.serviceOffering.Name, instanceID)
	logger := b.loggerFactory.NewWithContext(ctx)

	if !asyncAllowed {
		return brokerapi.UpdateServiceSpec{}, b.processError(brokerapi.ErrAsyncRequired, logger)
	}

	plan, found := b.serviceOffering.FindPlanByID(details.PlanID)
	if !found {
		message := fmt.Sprintf("Plan %s not found", details.PlanID)
		return brokerapi.UpdateServiceSpec{IsAsync: true}, b.processError(errors.New(message), logger)
	}

	if details.PreviousValues.PlanID != plan.ID {
		if err := b.validatePlanQuota(ctx, details.ServiceID, plan, logger); err != nil {
			return brokerapi.UpdateServiceSpec{IsAsync: true}, b.processError(err, logger)
		}
	}

	logger.Printf("updating instance %s", instanceID)
	detailsWithRawParameters := brokerapi.DetailsWithRawParameters(details)
	detailsMap, err := convertDetailsToMap(detailsWithRawParameters)
	if err != nil {
		return brokerapi.UpdateServiceSpec{IsAsync: true}, b.processError(NewGenericError(ctx, err), logger)
	}

	if b.EnablePlanSchemas {
		var schemas brokerapi.ServiceSchemas
		schemas, err = b.adapterClient.GeneratePlanSchema(plan.AdapterPlan(b.serviceOffering.GlobalProperties), logger)
		if err != nil {
			if _, ok := err.(serviceadapter.NotImplementedError); !ok {
				return brokerapi.UpdateServiceSpec{}, b.processError(err, logger)
			}
			logger.Println("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas")
			return brokerapi.UpdateServiceSpec{}, b.processError(fmt.Errorf("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas"), logger)
		}

		instanceUpdateSchema := schemas.Instance.Update

		validator := NewValidator(instanceUpdateSchema.Parameters)
		err = validator.ValidateSchema()
		if err != nil {
			return brokerapi.UpdateServiceSpec{}, err
		}

		params := make(map[string]interface{})
		err = json.Unmarshal(details.RawParameters, &params)
		if err != nil && len(details.RawParameters) > 0 {
			return brokerapi.UpdateServiceSpec{}, fmt.Errorf("update request params are malformed: %s", details.RawParameters)
		}

		err = validator.ValidateParams(params)
		if err != nil {
			return brokerapi.UpdateServiceSpec{}, err
		}
	}

	var boshContextID string

	if plan.PostDeployErrand() != "" {
		boshContextID = uuid.New()
	}

	boshTaskID, _, err := b.deployer.Update(
		deploymentName(instanceID),
		details.PlanID,
		detailsMap,
		&details.PreviousValues.PlanID,
		boshContextID,
		logger,
	)

	switch err := err.(type) {
	case task.ServiceError:
		return brokerapi.UpdateServiceSpec{IsAsync: true}, b.processError(NewBoshRequestError("update", fmt.Errorf("error deploying instance: %s", err)), logger)
	case task.PendingChangesNotAppliedError:
		return brokerapi.UpdateServiceSpec{IsAsync: true}, b.processError(brokerapi.NewFailureResponse(
			errors.New(PendingChangesErrorMessage),
			http.StatusUnprocessableEntity,
			UpdateLoggerAction,
		), logger)
	case task.TaskInProgressError:
		return brokerapi.UpdateServiceSpec{IsAsync: true}, b.processError(errors.New(OperationInProgressMessage), logger)
	case task.PlanNotFoundError:
		return brokerapi.UpdateServiceSpec{IsAsync: true}, b.processError(err, logger)
	case serviceadapter.UnknownFailureError:
		return brokerapi.UpdateServiceSpec{IsAsync: true}, b.processError(adapterToAPIError(ctx, err), logger)
	case error:
		return brokerapi.UpdateServiceSpec{IsAsync: true}, b.processError(NewGenericError(ctx, fmt.Errorf("error deploying instance: %s", err)), logger)
	}

	operationData, err := json.Marshal(OperationData{
		BoshTaskID:    boshTaskID,
		OperationType: OperationTypeUpdate,
		BoshContextID: boshContextID,
		Errands:       plan.PostDeployErrands(),
	})
	if err != nil {
		return brokerapi.UpdateServiceSpec{IsAsync: true}, b.processError(NewGenericError(brokercontext.WithBoshTaskID(ctx, boshTaskID), err), logger)
	}

	return brokerapi.UpdateServiceSpec{IsAsync: true, OperationData: string(operationData)}, nil
}
