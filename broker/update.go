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
	"net/http"

	"github.com/pivotal-cf/brokerapi/v7/domain"
	"github.com/pivotal-cf/brokerapi/v7/domain/apiresponses"
	"github.com/pivotal-cf/on-demand-service-broker/config"

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

func (b *Broker) Update(
	ctx context.Context,
	instanceID string,
	details domain.UpdateDetails,
	asyncAllowed bool,
) (domain.UpdateServiceSpec, error) {
	requestID := uuid.New()
	ctx = brokercontext.New(ctx, string(OperationTypeUpdate), requestID, b.serviceOffering.Name, instanceID)
	logger := b.loggerFactory.NewWithContext(ctx)

	if !asyncAllowed {
		return domain.UpdateServiceSpec{}, b.processError(apiresponses.ErrAsyncRequired, logger)
	}

	detailsWithRawParameters := domain.DetailsWithRawParameters(details)
	detailsMap, err := convertDetailsToMap(detailsWithRawParameters)
	if err != nil {
		return domain.UpdateServiceSpec{}, b.processError(NewGenericError(ctx, err), logger)
	}

	if err := b.validateMaintenanceInfo(details.PlanID, details.MaintenanceInfo, logger); err != nil {
		return domain.UpdateServiceSpec{}, b.processError(err, logger)
	}

	if b.isUpgrade(details, detailsMap) {
		operationData, err := b.Upgrade(ctx, instanceID, details, logger)
		if err != nil {
			if _, ok := err.(OperationAlreadyCompletedError); ok {
				return domain.UpdateServiceSpec{IsAsync: false}, nil
			}
			return domain.UpdateServiceSpec{}, err
		}
		operationDataJSON, err := json.Marshal(operationData)
		if err != nil {
			return domain.UpdateServiceSpec{}, b.processError(NewGenericError(ctx, err), logger)
		}
		return domain.UpdateServiceSpec{IsAsync: true, OperationData: string(operationDataJSON)}, nil
	}

	if err := b.validateMaintenanceInfo(details.PreviousValues.PlanID, details.PreviousValues.MaintenanceInfo, logger); err != nil {
		if err == apiresponses.ErrMaintenanceInfoConflict {
			return domain.UpdateServiceSpec{}, b.processError(
				apiresponses.NewFailureResponseBuilder(errors.New("service instance needs to be upgraded before updating"), http.StatusUnprocessableEntity, "previous-maintenance-info-check").Build(),
				logger)
		}
		return domain.UpdateServiceSpec{}, b.processError(err, logger)
	}

	b.deploymentLock.Lock()
	defer b.deploymentLock.Unlock()

	plan, err := b.checkPlanExists(details, logger)
	if err != nil {
		return domain.UpdateServiceSpec{}, b.processError(err, logger)
	}

	var boshContextID string
	if len(plan.PostDeployErrands()) > 0 {
		boshContextID = uuid.New()
	}

	var boshTaskID int
	var operationType OperationType

	err = b.validateQuotasForUpdate(ctx, plan, details, logger)
	if err != nil {
		return domain.UpdateServiceSpec{}, b.processError(err, logger)
	}

	err = b.validatePlanSchemas(plan, details, logger)
	if err != nil {
		return domain.UpdateServiceSpec{}, b.processError(err, logger)
	}

	var secretMap map[string]string
	secretMap, err = b.getSecretMap(instanceID, logger)
	if err != nil {
		return domain.UpdateServiceSpec{}, b.processError(NewGenericError(ctx, err), logger)
	}

	logger.Printf("updating instance %s", instanceID)

	operationType = OperationTypeUpdate
	boshTaskID, _, err = b.deployer.Update(
		deploymentName(instanceID),
		details.PlanID,
		detailsMap,
		&details.PreviousValues.PlanID,
		boshContextID,
		secretMap,
		logger,
	)

	if err != nil {
		return b.handleUpdateError(ctx, err, logger)
	}

	operationData, err := json.Marshal(OperationData{
		BoshTaskID:    boshTaskID,
		OperationType: operationType,
		BoshContextID: boshContextID,
		Errands:       plan.PostDeployErrands(),
	})
	if err != nil {
		return domain.UpdateServiceSpec{}, b.processError(NewGenericError(brokercontext.WithBoshTaskID(ctx, boshTaskID), err), logger)
	}

	return domain.UpdateServiceSpec{IsAsync: true, OperationData: string(operationData)}, nil
}

func (b *Broker) handleUpdateError(ctx context.Context, err error, logger *log.Logger) (domain.UpdateServiceSpec, error) {
	switch err := err.(type) {
	case ServiceError:
		return domain.UpdateServiceSpec{}, b.processError(NewBoshRequestError("update", fmt.Errorf("error deploying instance: %s", err)), logger)
	case PendingChangesNotAppliedError:
		return domain.UpdateServiceSpec{}, b.processError(apiresponses.NewFailureResponse(
			errors.New(PendingChangesErrorMessage),
			http.StatusUnprocessableEntity,
			UpdateLoggerAction,
		), logger)
	case TaskInProgressError:
		return domain.UpdateServiceSpec{}, b.processError(NewOperationInProgressError(errors.New(OperationInProgressMessage)), logger)
	case PlanNotFoundError, DeploymentNotFoundError, OperationAlreadyCompletedError:
		return domain.UpdateServiceSpec{}, b.processError(err, logger)
	case serviceadapter.UnknownFailureError:
		return domain.UpdateServiceSpec{}, b.processError(adapterToAPIError(ctx, err), logger)
	case error:
		return domain.UpdateServiceSpec{}, b.processError(NewGenericError(ctx, fmt.Errorf("error deploying instance: %s", err)), logger)
	}
	return domain.UpdateServiceSpec{}, nil
}

func (b *Broker) checkPlanExists(details domain.UpdateDetails, logger *log.Logger) (config.Plan, error) {
	plan, found := b.serviceOffering.FindPlanByID(details.PlanID)
	if !found {
		return config.Plan{}, PlanNotFoundError{PlanGUID: details.PlanID}
	}

	return plan, nil
}

func (b *Broker) isUpgrade(details domain.UpdateDetails, detailsMap map[string]interface{}) bool {
	if details.MaintenanceInfo != nil {
		params := detailsMap["parameters"]
		return details.PlanID == details.PreviousValues.PlanID && len(params.(map[string]interface{})) == 0
	}
	return false
}

func (b *Broker) validateMaintenanceInfo(planID string, maintenanceInfo *domain.MaintenanceInfo, logger *log.Logger) error {
	servicesCatalog, err := b.Services(context.Background())
	if err != nil {
		return err
	}

	return b.maintenanceInfoChecker.Check(planID, maintenanceInfo, servicesCatalog, logger)
}

func (b *Broker) validateQuotasForUpdate(ctx context.Context, plan config.Plan, details domain.UpdateDetails, logger *log.Logger) error {
	if details.PreviousValues.PlanID != plan.ID {
		cfPlanCounts, err := b.cfClient.CountInstancesOfServiceOffering(b.serviceOffering.ID, logger)
		if err != nil {
			return NewGenericError(ctx, err)
		}

		quotasErrors, ok := b.checkQuotas(ctx, plan, cfPlanCounts, b.serviceOffering.ID, logger)
		if !ok {
			return quotasErrors
		}
	}

	return nil
}

func (b *Broker) validatePlanSchemas(plan config.Plan, details domain.UpdateDetails, logger *log.Logger) error {

	if b.EnablePlanSchemas {
		var schemas domain.ServiceSchemas
		schemas, err := b.adapterClient.GeneratePlanSchema(plan.AdapterPlan(b.serviceOffering.GlobalProperties), logger)
		if err != nil {
			if _, ok := err.(serviceadapter.NotImplementedError); !ok {
				return err
			}
			logger.Println("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas")
			return fmt.Errorf("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas")
		}

		instanceUpdateSchema := schemas.Instance.Update

		validator := NewValidator(instanceUpdateSchema.Parameters)
		err = validator.ValidateSchema()
		if err != nil {
			return err
		}

		params := make(map[string]interface{})
		err = json.Unmarshal(details.RawParameters, &params)
		if err != nil && len(details.RawParameters) > 0 {
			return fmt.Errorf("update request params are malformed: %s", details.RawParameters)
		}

		err = validator.ValidateParams(params)
		if err != nil {
			return apiresponses.NewFailureResponseBuilder(err, http.StatusBadRequest, "params-validation-failed").Build()
		}
	}

	return nil
}

func (b *Broker) getSecretMap(instanceID string, logger *log.Logger) (map[string]string, error) {
	manifest, _, err := b.boshClient.GetDeployment(deploymentName(instanceID), logger)
	if err != nil {
		return nil, err
	}

	deploymentVariables, err := b.boshClient.Variables(deploymentName(instanceID), logger)
	if err != nil {
		return nil, err
	}

	secretMap, err := b.secretManager.ResolveManifestSecrets(manifest, deploymentVariables, logger)
	if err != nil {
		return nil, err
	}

	return secretMap, nil
}
