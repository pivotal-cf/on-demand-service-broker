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
	"reflect"

	"github.com/pivotal-cf/on-demand-service-broker/config"

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
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

	plan, err := b.checkPlanExists(details, logger, ctx)
	if err != nil {
		return brokerapi.UpdateServiceSpec{}, b.processError(err, logger)
	}

	var boshContextID string
	if len(plan.PostDeployErrands()) > 0 {
		boshContextID = uuid.New()
	}

	var boshTaskID int
	var operationType OperationType

	err = b.validateMaintenanceInfo(details, ctx)
	if err != nil {
		return brokerapi.UpdateServiceSpec{}, b.processError(err, logger)
	}

	detailsWithRawParameters := brokerapi.DetailsWithRawParameters(details)
	detailsMap, err := convertDetailsToMap(detailsWithRawParameters)
	if err != nil {
		return brokerapi.UpdateServiceSpec{}, b.processError(NewGenericError(ctx, err), logger)
	}

	if b.isUpgrade(details, detailsMap) {
		logger.Printf("upgrading instance %s", instanceID)

		operationType = OperationTypeUpgrade
		boshTaskID, _, err = b.deployer.Upgrade(
			deploymentName(instanceID),
			details.PlanID,
			&details.PreviousValues.PlanID,
			boshContextID,
			logger,
		)
	} else {
		err = b.validateQuotasForUpdate(plan, details, logger, ctx)
		if err != nil {
			return brokerapi.UpdateServiceSpec{}, b.processError(err, logger)
		}

		err = b.validatePlanSchemas(plan, details, logger)
		if err != nil {
			return brokerapi.UpdateServiceSpec{}, b.processError(err, logger)
		}

		var secretMap map[string]string
		secretMap, err = b.getSecretMap(instanceID, logger)
		if err != nil {
			return brokerapi.UpdateServiceSpec{}, b.processError(NewGenericError(ctx, err), logger)
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
	}

	if err != nil {
		return b.handleUpdateError(err, logger, ctx)
	}

	operationData, err := json.Marshal(OperationData{
		BoshTaskID:    boshTaskID,
		OperationType: operationType,
		BoshContextID: boshContextID,
		Errands:       plan.PostDeployErrands(),
	})
	if err != nil {
		return brokerapi.UpdateServiceSpec{}, b.processError(NewGenericError(brokercontext.WithBoshTaskID(ctx, boshTaskID), err), logger)
	}

	return brokerapi.UpdateServiceSpec{IsAsync: true, OperationData: string(operationData)}, nil
}

func (b *Broker) handleUpdateError(err error, logger *log.Logger, ctx context.Context) (brokerapi.UpdateServiceSpec, error) {
	switch err := err.(type) {
	case ServiceError:
		return brokerapi.UpdateServiceSpec{}, b.processError(NewBoshRequestError("update", fmt.Errorf("error deploying instance: %s", err)), logger)
	case PendingChangesNotAppliedError:
		return brokerapi.UpdateServiceSpec{}, b.processError(brokerapi.NewFailureResponse(
			errors.New(PendingChangesErrorMessage),
			http.StatusUnprocessableEntity,
			UpdateLoggerAction,
		), logger)
	case TaskInProgressError:
		return brokerapi.UpdateServiceSpec{}, b.processError(errors.New(OperationInProgressMessage), logger)
	case PlanNotFoundError:
		return brokerapi.UpdateServiceSpec{}, b.processError(err, logger)
	case serviceadapter.UnknownFailureError:
		return brokerapi.UpdateServiceSpec{}, b.processError(adapterToAPIError(ctx, err), logger)
	case error:
		return brokerapi.UpdateServiceSpec{}, b.processError(NewGenericError(ctx, fmt.Errorf("error deploying instance: %s", err)), logger)
	}
	return brokerapi.UpdateServiceSpec{}, nil
}

func (b *Broker) checkPlanExists(details brokerapi.UpdateDetails, logger *log.Logger, ctx context.Context) (config.Plan, error) {
	plan, found := b.serviceOffering.FindPlanByID(details.PlanID)
	if !found {
		message := fmt.Sprintf("Plan %s not found", details.PlanID)
		return config.Plan{}, errors.New(message)
	}

	return plan, nil
}

func (b *Broker) isUpgrade(details brokerapi.UpdateDetails, detailsMap map[string]interface{}) bool {
	if details.MaintenanceInfo.Private != "" || details.MaintenanceInfo.Public != nil {
		params := detailsMap["parameters"]
		return details.PlanID == details.PreviousValues.PlanID && len(params.(map[string]interface{})) == 0
	}
	return false
}

func (b *Broker) getMaintenanceInfoForPlan(id string) (*brokerapi.MaintenanceInfo, error) {
	services, err := b.Services(context.Background())
	if err != nil {
		return nil, err
	}

	for _, plan := range services[0].Plans {
		if plan.ID == id {
			return plan.MaintenanceInfo, nil
		}
	}

	return nil, fmt.Errorf("plan %s not found", id)
}

func (b *Broker) validateMaintenanceInfo(details brokerapi.UpdateDetails, ctx context.Context) error {
	if details.MaintenanceInfo.Private != "" || details.MaintenanceInfo.Public != nil {
		brokerMaintenanceInfo, err := b.getMaintenanceInfoForPlan(details.PlanID)
		if err != nil {
			return NewGenericError(ctx, err)
		}

		if brokerMaintenanceInfo != nil && !reflect.DeepEqual(*brokerMaintenanceInfo, details.MaintenanceInfo) {
			return brokerapi.ErrMaintenanceInfoConflict
		}

		if brokerMaintenanceInfo == nil {
			return brokerapi.ErrMaintenanceInfoNilConflict
		}
	}
	return nil
}

func (b *Broker) validateQuotasForUpdate(plan config.Plan, details brokerapi.UpdateDetails, logger *log.Logger, ctx context.Context) error {
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

func (b *Broker) validatePlanSchemas(plan config.Plan, details brokerapi.UpdateDetails, logger *log.Logger) error {

	if b.EnablePlanSchemas {
		var schemas brokerapi.ServiceSchemas
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
			return err
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
