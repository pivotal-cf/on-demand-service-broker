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
	"log"

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
		return brokerapi.UpdateServiceSpec{}, brokerapi.ErrAsyncRequired
	}

	errs := func(err DisplayableError) (brokerapi.UpdateServiceSpec, error) {
		logger.Println(err)
		return brokerapi.UpdateServiceSpec{IsAsync: true}, err.ErrorForCFUser()
	}

	plan, found := b.serviceOffering.FindPlanByID(details.PlanID)
	if !found {
		message := fmt.Sprintf("Plan %s not found", details.PlanID)
		logger.Println(message)
		return brokerapi.UpdateServiceSpec{IsAsync: true}, errors.New(message)
	}

	if details.PreviousValues.PlanID != plan.ID {
		if err := b.validatePlanQuota(ctx, details.ServiceID, plan, logger); err != NilError {
			return errs(err)
		}
	}

	logger.Printf("updating instance %s", instanceID)
	detailsWithRawParameters := brokerapi.DetailsWithRawParameters(details)
	detailsMap, err := convertDetailsToMap(detailsWithRawParameters)
	if err != nil {
		return errs(NewGenericError(ctx, err))
	}

	var boshContextID string
	var operationPostDeployErrandName string
	if plan.PostDeployErrand() != "" {
		boshContextID = uuid.New()
		operationPostDeployErrandName = plan.PostDeployErrand()
	}

	parameters := parametersFromRequest(detailsMap)
	applyingChanges, err := b.validatedApplyChanges(parameters)
	if err != nil {
		logger.Println(err)
		return brokerapi.UpdateServiceSpec{IsAsync: true}, b.asUpdateError(err)
	}

	if applyingChanges {
		if err := b.assertCanApplyChanges(parameters, details.PlanID, &details.PreviousValues.PlanID, logger); err != nil {
			return brokerapi.UpdateServiceSpec{IsAsync: true}, err
		}
	}
	boshTaskID, _, err := b.deployer.Update(
		deploymentName(instanceID),
		details.PlanID,
		applyingChanges,
		detailsMap,
		&details.PreviousValues.PlanID,
		boshContextID,
		logger,
	)

	switch err := err.(type) {
	case task.ServiceError:
		return errs(NewBoshRequestError("update", fmt.Errorf("error deploying instance: %s", err)))
	case task.PendingChangesNotAppliedError:
		return brokerapi.UpdateServiceSpec{IsAsync: true}, b.asPendingChangesError()
	case task.TaskInProgressError:
		return brokerapi.UpdateServiceSpec{IsAsync: true}, errors.New(OperationInProgressMessage)
	case task.PlanNotFoundError:
		return brokerapi.UpdateServiceSpec{IsAsync: true}, err
	case serviceadapter.UnknownFailureError:
		return brokerapi.UpdateServiceSpec{IsAsync: true}, adapterToAPIError(ctx, err)
	case error:
		return errs(NewGenericError(ctx, fmt.Errorf("error deploying instance: %s", err)))
	}

	operationData, err := json.Marshal(OperationData{
		BoshTaskID:           boshTaskID,
		OperationType:        OperationTypeUpdate,
		BoshContextID:        boshContextID,
		PostDeployErrandName: operationPostDeployErrandName,
	})
	if err != nil {
		return errs(NewGenericError(brokercontext.WithBoshTaskID(ctx, boshTaskID), err))
	}

	return brokerapi.UpdateServiceSpec{IsAsync: true, OperationData: string(operationData)}, nil
}

func (b *Broker) asPendingChangesError() error {
	if b.featureFlags.CFUserTriggeredUpgrades() {
		return errors.New(PendingChangesErrorMessage)
	}
	return errors.New(ApplyChangesDisabledMessage)
}

func (b *Broker) asUpdateError(err error) error {
	if b.featureFlags.CFUserTriggeredUpgrades() {
		return errors.New(PendingChangesErrorMessage)
	}
	return errors.New(ApplyChangesNotPermittedMessage)
}

func parametersFromRequest(requestParams map[string]interface{}) map[string]interface{} {
	parameters, ok := requestParams["parameters"].(map[string]interface{})
	if !ok {
		return nil
	}

	return parameters
}

func (b *Broker) validatedApplyChanges(parameters map[string]interface{}) (bool, error) {
	const applyChangesKey = "apply-changes"

	value := parameters[applyChangesKey]
	if value == nil {
		return false, nil
	}

	applyChanges, ok := value.(bool)
	if !ok {
		return false, applyChangesNotABooleanError(value)
	}

	delete(parameters, applyChangesKey)

	return applyChanges, nil
}

func (b *Broker) assertCanApplyChanges(parameters map[string]interface{}, planID string, previousPlanID *string, logger *log.Logger) error {
	if !b.featureFlags.CFUserTriggeredUpgrades() {
		logger.Println("'cf_user_triggered_upgrades' feature is disabled")
		return errors.New(ApplyChangesNotPermittedMessage)
	}

	if previousPlanID != nil && planID != *previousPlanID {
		logger.Println("update called with apply-changes and a plan change")
		return errors.New(PendingChangesErrorMessage)
	}

	if len(parameters) > 0 {
		logger.Println("update called with apply-changes and arbitrary parameters set")
		return errors.New(PendingChangesErrorMessage)
	}

	return nil
}
