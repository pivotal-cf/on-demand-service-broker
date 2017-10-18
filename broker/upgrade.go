// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"context"
	"fmt"
	"log"

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/task"
)

func (b *Broker) Upgrade(ctx context.Context, instanceID string, logger *log.Logger) (OperationData, error) {
	b.deploymentLock.Lock()
	defer b.deploymentLock.Unlock()

	instance, err := b.cfClient.GetInstanceState(instanceID, logger)
	if err != nil {
		return OperationData{}, err
	}

	if instance.OperationInProgress {
		return OperationData{}, NewOperationInProgressError(fmt.Errorf("cloud controller: operation in progress for instance %s", instanceID))
	}

	logger.Printf("upgrading instance %s", instanceID)

	plan, found := b.serviceOffering.FindPlanByID(instance.PlanID)
	if !found {
		logger.Printf("error: finding plan ID %s", instance.PlanID)
		return OperationData{}, fmt.Errorf("plan %s not found", instance.PlanID)
	}

	var boshContextID string
	var operationPostDeployErrand string
	if plan.LifecycleErrands != nil {
		boshContextID = uuid.New()
		operationPostDeployErrand = plan.PostDeployErrand()
	}

	taskID, _, err := b.deployer.Upgrade(
		deploymentName(instanceID),
		instance.PlanID,
		&instance.PlanID,
		boshContextID,
		logger,
	)

	if err != nil {
		logger.Printf("error upgrading instance %s: %s", instanceID, err)

		switch err := err.(type) {
		case DisplayableError:
			return OperationData{}, err.ErrorForCFUser()
		case serviceadapter.UnknownFailureError:
			return OperationData{}, adapterToAPIError(ctx, err)
		case task.TaskInProgressError:
			return OperationData{}, NewOperationInProgressError(err)
		default:
			return OperationData{}, err
		}
	}

	return OperationData{
		BoshContextID: boshContextID,
		BoshTaskID:    taskID,
		OperationType: OperationTypeUpgrade,
		PostDeployErrand: PostDeployErrand{
			Name: operationPostDeployErrand,
		},
	}, nil
}
