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
)

var descriptions = map[brokerapi.LastOperationState]map[OperationType]string{
	brokerapi.InProgress: {
		OperationTypeCreate:  "Instance provisioning in progress",
		OperationTypeUpdate:  "Instance update in progress",
		OperationTypeUpgrade: "Instance upgrade in progress",
		OperationTypeDelete:  "Instance deletion in progress",
	},
	brokerapi.Succeeded: {
		OperationTypeCreate:  "Instance provisioning completed",
		OperationTypeUpdate:  "Instance update completed",
		OperationTypeUpgrade: "Instance upgrade completed",
		OperationTypeDelete:  "Instance deletion completed",
	},
	brokerapi.Failed: {
		OperationTypeCreate:  "Instance provisioning failed",
		OperationTypeUpdate:  "Instance update failed",
		OperationTypeUpgrade: "Failed for bosh task",
		OperationTypeDelete:  "Instance deletion failed",
	},
}

func (b *Broker) LastOperation(ctx context.Context, instanceID, operationDataRaw string,
) (brokerapi.LastOperation, error) {

	requestID := uuid.New()
	ctx = brokercontext.New(ctx, "", requestID, b.serviceOffering.Name, instanceID)
	logger := b.loggerFactory.NewWithContext(ctx)

	errs := func(err DisplayableError) (brokerapi.LastOperation, error) {
		logger.Println(err)
		return brokerapi.LastOperation{}, err.ErrorForCFUser()
	}

	if operationDataRaw == "" {
		err := errors.New("Request missing operation data, please check your Cloud Foundry version is v238+")
		return errs(NewGenericError(ctx, err))
	}

	var operationData OperationData
	if err := json.Unmarshal([]byte(operationDataRaw), &operationData); err != nil {
		return errs(NewGenericError(
			ctx, fmt.Errorf("operation data cannot be parsed: %s", err),
		))
	}

	ctx = brokercontext.WithOperation(ctx, string(operationData.OperationType))

	if operationData.BoshTaskID == 0 {
		return errs(NewGenericError(
			ctx, errors.New("no task ID found in operation data"),
		))
	}

	ctx = brokercontext.WithBoshTaskID(ctx, operationData.BoshTaskID)

	lifeCycleRunner := NewLifeCycleRunner(b.boshClient, b.serviceOffering.Plans)

	// if the errand isn't already running, GetTask will start it!
	lastBoshTask, err := lifeCycleRunner.GetTask(deploymentName(instanceID), operationData, logger)
	if err != nil {
		return errs(NewGenericError(ctx, fmt.Errorf(
			"error retrieving tasks from bosh, for deployment '%s': %s",
			deploymentName(instanceID), err,
		)))
	}

	ctx = brokercontext.WithBoshTaskID(ctx, lastBoshTask.ID)

	lastOperation := constructLastOperation(ctx, lastBoshTask, operationData, logger)
	logLastOperation(instanceID, lastBoshTask, operationData, logger)

	return lastOperation, nil
}

func constructLastOperation(ctx context.Context, boshTask boshdirector.BoshTask, operationData OperationData, logger *log.Logger) brokerapi.LastOperation {
	taskState := lastOperationState(boshTask, logger)
	description := descriptionForOperationTask(ctx, taskState, operationData, boshTask.ID)

	return brokerapi.LastOperation{State: taskState, Description: description}
}

func lastOperationState(task boshdirector.BoshTask, logger *log.Logger) brokerapi.LastOperationState {
	switch task.StateType() {
	case boshdirector.TaskIncomplete:
		return brokerapi.InProgress
	case boshdirector.TaskComplete:
		return brokerapi.Succeeded
	case boshdirector.TaskFailed:
		return brokerapi.Failed
	default:
		logger.Printf("Unrecognised BOSH task state: %s", task.State)
		return brokerapi.Failed
	}
}

func descriptionForOperationTask(ctx context.Context, taskState brokerapi.LastOperationState, operationData OperationData, taskID int) string {
	description := descriptions[taskState][operationData.OperationType]

	if taskState == brokerapi.Failed {
		if operationData.OperationType == OperationTypeUpgrade {
			description = fmt.Sprintf(description+": %d", taskID) // Allows upgrader to log BOSH task ID when an upgrade fails
		} else {
			description = fmt.Sprintf(description+": %s", NewGenericError(ctx, nil).ErrorForCFUser())
		}
	}

	return description
}

func logLastOperation(instanceID string, boshTask boshdirector.BoshTask, operationData OperationData, logger *log.Logger) {
	logger.Printf(
		"BOSH task ID %d status: %s %s deployment for instance %s: Description: %s Result: %s\n",
		boshTask.ID,
		boshTask.State,
		operationData.OperationType,
		instanceID,
		boshTask.Description,
		boshTask.Result,
	)
}
