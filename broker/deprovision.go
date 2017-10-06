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
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

func (b *Broker) Deprovision(
	ctx context.Context,
	instanceID string,
	deprovisionDetails brokerapi.DeprovisionDetails,
	asyncAllowed bool,
) (brokerapi.DeprovisionServiceSpec, error) {
	b.deploymentLock.Lock()
	defer b.deploymentLock.Unlock()

	requestID := uuid.New()
	ctx = brokercontext.New(ctx, string(OperationTypeDelete), requestID, b.serviceOffering.Name, instanceID)
	logger := b.loggerFactory.NewWithContext(ctx)

	if !asyncAllowed {
		return brokerapi.DeprovisionServiceSpec{}, brokerapi.ErrAsyncRequired
	}

	if err := b.assertDeploymentExists(ctx, instanceID, logger); err != NilError {
		return deprovisionErr(err, logger)
	}

	if err := b.assertNoOperationsInProgress(ctx, instanceID, logger); err != NilError {
		return deprovisionErr(err, logger)
	}

	plan, found := b.serviceOffering.FindPlanByID(deprovisionDetails.PlanID)
	if found {
		if errand := plan.PreDeleteErrand(); errand != "" {
			return b.runPreDeleteErrand(ctx, instanceID, errand, logger)
		}
	}

	return b.deleteInstance(ctx, instanceID, plan, logger)
}

func (b *Broker) assertDeploymentExists(ctx context.Context, instanceID string, logger *log.Logger) DisplayableError {
	_, deploymentFound, err := b.boshClient.GetDeployment(deploymentName(instanceID), logger)

	switch err.(type) {
	case boshdirector.RequestError:
		return NewBoshRequestError("delete", err)
	case error:
		return NewGenericError(
			ctx,
			fmt.Errorf("error deprovisioning: cannot get deployment %s: %s", deploymentName(instanceID), err),
		)
	}

	if !deploymentFound {
		return NewDisplayableError(
			brokerapi.ErrInstanceDoesNotExist,
			fmt.Errorf("error deprovisioning: instance %s, not found", instanceID),
		)
	}

	return NilError
}

func (b *Broker) assertNoOperationsInProgress(ctx context.Context, instanceID string, logger *log.Logger) DisplayableError {

	tasks, err := b.boshClient.GetTasks(deploymentName(instanceID), logger)
	switch err.(type) {
	case boshdirector.RequestError:
		return NewBoshRequestError("delete", err)
	case error:
		return NewGenericError(
			ctx,
			fmt.Errorf("error deprovisioning: cannot get tasks for deployment %s: %s\n", deploymentName(instanceID), err),
		)
	}

	incompleteTasks := tasks.IncompleteTasks()
	if len(incompleteTasks) > 0 {
		userError := errors.New("An operation is in progress for your service instance. Please try again later.")
		operatorError := NewOperationInProgressError(
			fmt.Errorf("error deprovisioning: deployment %s is still in progress: tasks %s\n",
				deploymentName(instanceID),
				incompleteTasks.ToLog()),
		)
		return NewDisplayableError(userError, operatorError)
	}

	return NilError
}

func (b *Broker) runPreDeleteErrand(
	ctx context.Context,
	instanceID string,
	preDeleteErrand string,
	logger *log.Logger,
) (brokerapi.DeprovisionServiceSpec, error) {
	logger.Printf("running pre-delete errand for instance %s\n", instanceID)

	boshContextID := uuid.New()

	taskID, err := b.boshClient.RunErrand(
		deploymentName(instanceID),
		preDeleteErrand,
		[]string{},
		boshContextID,
		logger,
	)
	if err != nil {
		return deprovisionErr(NewGenericError(ctx, err), logger)
	}

	operationData, err := json.Marshal(OperationData{
		OperationType: OperationTypeDelete,
		BoshTaskID:    taskID,
		BoshContextID: boshContextID,
	})

	if err != nil {
		return deprovisionErr(NewGenericError(ctx, err), logger)
	}

	return brokerapi.DeprovisionServiceSpec{IsAsync: true, OperationData: string(operationData)}, nil
}

func (b *Broker) deleteInstance(
	ctx context.Context,
	instanceID string,
	planConfig config.Plan,
	logger *log.Logger,
) (brokerapi.DeprovisionServiceSpec, error) {
	logger.Printf("deleting deployment for instance %s\n", instanceID)
	taskID, err := b.boshClient.DeleteDeployment(deploymentName(instanceID), "", logger)
	switch err.(type) {
	case boshdirector.RequestError:
		return deprovisionErr(NewBoshRequestError("delete", err), logger)
	case error:
		return deprovisionErr(NewGenericError(
			ctx,
			fmt.Errorf("error deprovisioning: deleting bosh deployment: %s", err),
		), logger)
	}

	logger.Printf("Bosh task id for Delete instance %s was %d\n", instanceID, taskID)
	ctx = brokercontext.WithBoshTaskID(ctx, taskID)

	operationData, err := json.Marshal(OperationData{
		OperationType: OperationTypeDelete,
		BoshTaskID:    taskID,
	})

	if err != nil {
		return deprovisionErr(NewGenericError(ctx, err), logger)
	}

	return brokerapi.DeprovisionServiceSpec{
		IsAsync:       true,
		OperationData: string(operationData),
	}, nil
}

func deprovisionErr(
	err DisplayableError,
	logger *log.Logger,
) (brokerapi.DeprovisionServiceSpec, error) {
	logger.Println(err)
	return brokerapi.DeprovisionServiceSpec{IsAsync: true}, err.ErrorForCFUser()
}
