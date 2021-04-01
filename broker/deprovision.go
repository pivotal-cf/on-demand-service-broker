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
	"github.com/pivotal-cf/brokerapi/v8/domain"
	"github.com/pivotal-cf/brokerapi/v8/domain/apiresponses"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

func (b *Broker) Deprovision(
	ctx context.Context,
	instanceID string,
	deprovisionDetails domain.DeprovisionDetails,
	asyncAllowed bool,
) (domain.DeprovisionServiceSpec, error) {

	b.deploymentLock.Lock()
	defer b.deploymentLock.Unlock()
	requestID := uuid.New()
	ctx = brokercontext.New(ctx, string(OperationTypeDelete), requestID, b.serviceOffering.Name, instanceID)
	logger := b.loggerFactory.NewWithContext(ctx)

	if !asyncAllowed {
		return domain.DeprovisionServiceSpec{}, b.processError(apiresponses.ErrAsyncRequired, logger)
	}

	_, err := b.boshClient.GetInfo(logger)
	if err != nil {
		return domain.DeprovisionServiceSpec{IsAsync: true}, b.processError(NewBoshRequestError("delete", err), logger)
	}

	deploymentExists, err := b.assertDeploymentExists(ctx, instanceID, logger)
	if err != nil {
		return domain.DeprovisionServiceSpec{IsAsync: true}, b.processError(err, logger)
	}

	if !deploymentExists {
		var err error
		err = NewDisplayableError(
			apiresponses.ErrInstanceDoesNotExist,
			fmt.Errorf("error deprovisioning: instance %s, not found", instanceID),
		)

		if !b.DisableBoshConfigs {
			deleteConfigsErr := b.deleteConfigsForNotFoundInstance(ctx, instanceID, logger)
			if deleteConfigsErr != nil {
				return domain.DeprovisionServiceSpec{IsAsync: true}, b.processError(deleteConfigsErr, logger)
			}
		}

		secretsErr := b.clearSecretsForNotFoundInstance(ctx, instanceID, logger)
		if secretsErr != nil {
			err = secretsErr
		}

		return domain.DeprovisionServiceSpec{IsAsync: true}, b.processError(err, logger)
	}

	if err := b.assertNoOperationsInProgress(ctx, instanceID, logger); err != nil {
		return domain.DeprovisionServiceSpec{IsAsync: true}, b.processError(err, logger)
	}

	operationType := b.getOperationType(deprovisionDetails.Force)
	plan, found := b.serviceOffering.FindPlanByID(deprovisionDetails.PlanID)
	if found {
		if errands := plan.PreDeleteErrands(); len(errands) != 0 {
			serviceSpec, err := b.runPreDeleteErrands(ctx, instanceID, errands, operationType, logger)
			return serviceSpec, b.processError(err, logger)
		}
	}

	serviceSpec, err := b.deleteInstance(ctx, instanceID, plan, operationType, logger)

	clientErr := b.uaaClient.DeleteClient(instanceID)
	if clientErr != nil {
		logger.Printf("failed to delete UAA client associated with service instance %s\n", instanceID)
	}

	return serviceSpec, b.processError(err, logger)
}

func (b *Broker) deleteConfigsForNotFoundInstance(ctx context.Context, instanceID string, logger *log.Logger) error {
	if err := b.boshClient.DeleteConfigs(deploymentName(instanceID), logger); err != nil {
		operatorError := NewGenericError(
			ctx,
			fmt.Errorf("error deprovisioning: failed to delete configs for instance %s: %s", deploymentName(instanceID), err),
		)
		return NewDisplayableError(errors.New("Unable to delete service. Please try again later or contact your operator."), operatorError)
	}

	return nil
}

func (b *Broker) clearSecretsForNotFoundInstance(ctx context.Context, instanceID string, logger *log.Logger) error {
	if err := b.secretManager.DeleteSecretsForInstance(instanceID, logger); err != nil {
		userError := errors.New("Unable to delete service. Please try again later or contact your operator.")
		operatorError := NewGenericError(
			ctx,
			fmt.Errorf("error deprovisioning: failed to delete secrets for instance %s: %s", deploymentName(instanceID), err),
		)
		return NewDisplayableError(userError, operatorError)
	}
	return nil
}

func (b *Broker) assertDeploymentExists(ctx context.Context, instanceID string, logger *log.Logger) (bool, error) {
	_, deploymentFound, err := b.boshClient.GetDeployment(deploymentName(instanceID), logger)

	switch err.(type) {
	case boshdirector.RequestError:
		return false, NewBoshRequestError("delete", err)
	case error:
		return false, NewGenericError(
			ctx,
			fmt.Errorf("error deprovisioning: cannot get deployment %s: %s", deploymentName(instanceID), err),
		)
	}

	return deploymentFound, nil
}

func (b *Broker) assertNoOperationsInProgress(ctx context.Context, instanceID string, logger *log.Logger) error {

	incompleteTasks, err := b.boshClient.GetTasksInProgress(deploymentName(instanceID), logger)
	switch err.(type) {
	case boshdirector.RequestError:
		return NewBoshRequestError("delete", err)
	case error:
		return NewGenericError(
			ctx,
			fmt.Errorf("error deprovisioning: cannot get tasks for deployment %s: %s\n", deploymentName(instanceID), err),
		)
	}

	if len(incompleteTasks) > 0 {
		userError := errors.New("An operation is in progress for your service instance. Please try again later.")
		operatorError := NewOperationInProgressError(
			fmt.Errorf("error deprovisioning: deployment %s is still in progress: tasks %s\n",
				deploymentName(instanceID),
				incompleteTasks.ToLog()),
		)
		return NewDisplayableError(userError, operatorError)
	}

	return nil
}

func (b *Broker) runPreDeleteErrands(ctx context.Context, instanceID string, preDeleteErrands []config.Errand, operationType OperationType, logger *log.Logger) (domain.DeprovisionServiceSpec, error) {
	logger.Printf("running pre-delete errand for instance %s\n", instanceID)

	boshContextID := uuid.New()

	taskID, err := b.boshClient.RunErrand(
		deploymentName(instanceID),
		preDeleteErrands[0].Name,
		preDeleteErrands[0].Instances,
		boshContextID,
		logger,
		boshdirector.NewAsyncTaskReporter(),
	)
	if err != nil {
		return domain.DeprovisionServiceSpec{IsAsync: true}, NewGenericError(ctx, err)
	}

	operationData, err := json.Marshal(OperationData{
		OperationType: operationType,
		BoshTaskID:    taskID,
		BoshContextID: boshContextID,
		Errands:       preDeleteErrands,
	})

	if err != nil {
		return domain.DeprovisionServiceSpec{IsAsync: true}, NewGenericError(ctx, err)
	}

	return domain.DeprovisionServiceSpec{IsAsync: true, OperationData: string(operationData)}, nil
}

func (b *Broker) deleteInstance(ctx context.Context, instanceID string, planConfig config.Plan, operationType OperationType, logger *log.Logger) (domain.DeprovisionServiceSpec, error) {
	logger.Printf("removing deployment for instance %s as part of operation %q\n", instanceID, operationType)

	taskID, err := b.boshClient.DeleteDeployment(
		deploymentName(instanceID),
		fmt.Sprintf("delete-%s", instanceID),
		operationType == OperationTypeForceDelete,
		boshdirector.NewAsyncTaskReporter(),
		logger,
	)
	switch err.(type) {
	case boshdirector.RequestError:
		return domain.DeprovisionServiceSpec{IsAsync: true}, NewBoshRequestError("delete", err)
	case error:
		return domain.DeprovisionServiceSpec{IsAsync: true}, NewGenericError(
			ctx,
			fmt.Errorf("error deprovisioning: deleting bosh deployment: %s", err),
		)
	}

	logger.Printf("Bosh task id is %d for operation %q of instance %s\n", taskID, operationType, instanceID)
	ctx = brokercontext.WithBoshTaskID(ctx, taskID)

	operationData, err := b.generateOperationData(operationType, err, taskID)
	if err != nil {
		return domain.DeprovisionServiceSpec{IsAsync: true}, NewGenericError(ctx, err)
	}

	return domain.DeprovisionServiceSpec{
		IsAsync:       true,
		OperationData: operationData,
	}, nil
}

func (b *Broker) generateOperationData(operationType OperationType, err error, taskID int) (string, error) {
	operationData, err := json.Marshal(OperationData{
		OperationType: operationType,
		BoshTaskID:    taskID,
	})
	if err != nil {
		return "", err
	}

	return string(operationData), nil
}

func (b *Broker) getOperationType(forceDelete bool) OperationType {
	operationType := OperationTypeDelete
	if forceDelete {
		operationType = OperationTypeForceDelete
	}
	return operationType
}
