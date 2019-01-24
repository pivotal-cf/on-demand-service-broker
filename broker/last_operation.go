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
		OperationTypeCreate:   "Instance provisioning in progress",
		OperationTypeUpdate:   "Instance update in progress",
		OperationTypeUpgrade:  "Instance upgrade in progress",
		OperationTypeDelete:   "Instance deletion in progress",
		OperationTypeRecreate: "Instance recreate in progress",
	},
	brokerapi.Succeeded: {
		OperationTypeCreate:   "Instance provisioning completed",
		OperationTypeUpdate:   "Instance update completed",
		OperationTypeUpgrade:  "Instance upgrade completed",
		OperationTypeDelete:   "Instance deletion completed",
		OperationTypeRecreate: "Instance recreate completed",
	},
	brokerapi.Failed: {
		OperationTypeCreate:   "Instance provisioning failed",
		OperationTypeUpdate:   "Instance update failed",
		OperationTypeUpgrade:  "Failed for bosh task",
		OperationTypeDelete:   "Instance deletion failed",
		OperationTypeRecreate: "Instance recreate failed",
	},
}

func (b *Broker) LastOperation(ctx context.Context, instanceID string, pollDetails brokerapi.PollDetails,
) (brokerapi.LastOperation, error) {

	operationDataRaw := pollDetails.OperationData
	requestID := uuid.New()
	ctx = brokercontext.New(ctx, "", requestID, b.serviceOffering.Name, instanceID)
	logger := b.loggerFactory.NewWithContext(ctx)

	if operationDataRaw == "" {
		err := errors.New("Request missing operation data, please check your Cloud Foundry version is v238+")
		return brokerapi.LastOperation{}, b.processError(NewGenericError(ctx, err), logger)
	}

	var operationData OperationData
	if err := json.Unmarshal([]byte(operationDataRaw), &operationData); err != nil {
		return brokerapi.LastOperation{}, b.processError(NewGenericError(ctx, fmt.Errorf("operation data cannot be parsed: %s", err)), logger)
	}

	ctx = brokercontext.WithOperation(ctx, string(operationData.OperationType))

	if operationData.BoshTaskID == 0 {
		return brokerapi.LastOperation{}, b.processError(NewGenericError(ctx, errors.New("no task ID found in operation data")), logger)
	}

	ctx = brokercontext.WithBoshTaskID(ctx, operationData.BoshTaskID)

	lifeCycleRunner := NewLifeCycleRunner(b.boshClient, b.serviceOffering.Plans)

	// if the errand isn't already running, or delete deployment wasn't triggered, GetTask will start it!
	lastBoshTask, err := lifeCycleRunner.GetTask(deploymentName(instanceID), operationData, logger)
	if err != nil {
		return brokerapi.LastOperation{}, b.processError(
			NewGenericError(ctx, fmt.Errorf("error retrieving tasks from bosh, for deployment '%s': %s", deploymentName(instanceID), err)),
			logger,
		)
	}

	if operationData.OperationType == OperationTypeDelete && lastBoshTask.StateType() == boshdirector.TaskComplete {
		if !b.DisableBoshConfigs {
			if err = b.boshClient.DeleteConfigs(deploymentName(instanceID), logger); err != nil {
				ctx = brokercontext.WithBoshTaskID(ctx, 0)
				lastOperation := constructLastOperation(ctx, brokerapi.Failed, lastBoshTask, operationData, b.ExposeOperationalErrors)
				logger.Printf("Failed to delete configs for service instance %s: %s\n", instanceID, err.Error())
				return lastOperation, nil
			}
		}

		if err = b.secretManager.DeleteSecretsForInstance(instanceID, logger); err != nil {
			ctx = brokercontext.WithBoshTaskID(ctx, 0)
			lastOperation := constructLastOperation(ctx, brokerapi.Failed, lastBoshTask, operationData, b.ExposeOperationalErrors)
			logger.Printf("Failed to delete credhub secrets for service instance %s. Credhub error: %s\n", instanceID, err.Error())
			return lastOperation, nil
		}
	}

	ctx = brokercontext.WithBoshTaskID(ctx, lastBoshTask.ID)

	taskState := lastOperationState(lastBoshTask, logger)
	lastOperation := constructLastOperation(ctx, taskState, lastBoshTask, operationData, b.ExposeOperationalErrors)
	logLastOperation(instanceID, lastBoshTask, operationData, logger)

	return lastOperation, nil
}

func constructLastOperation(ctx context.Context, taskState brokerapi.LastOperationState, lastBoshTask boshdirector.BoshTask, operationData OperationData, exposeError bool) brokerapi.LastOperation {
	description := descriptions[taskState][operationData.OperationType]
	if taskState == brokerapi.Failed {
		if operationData.OperationType == OperationTypeUpgrade {
			description = fmt.Sprintf(description+": %d", lastBoshTask.ID) // Allows instanceiterator to log BOSH task ID when an upgrade fails
		} else {
			description = fmt.Sprintf(description+": %s", NewGenericError(ctx, nil).ErrorForCFUser())
		}

		if exposeError {
			description = fmt.Sprintf("%s, error-message: %s", description, lastBoshTask.Result)
		}

	}
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
