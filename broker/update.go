// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/adapterclient"
	"github.com/pivotal-cf/on-demand-service-broker/boshclient"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
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
		return errs(NewDisplayableError(fmt.Errorf("plan %s not found", details.PlanID), fmt.Errorf("finding plan ID %s", details.PlanID)))
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
	var operationDataPlanID string
	if plan.PostDeployErrand() != "" {
		boshContextID = uuid.New()
		operationDataPlanID = details.PlanID
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
	case boshclient.RequestError:
		return errs(NewBoshRequestError("update", fmt.Errorf("error deploying instance: %s", err)))
	case TaskError:
		return errs(b.asDisplayableError(err))
	case DisplayableError:
		return errs(err)
	case adapterclient.UnknownFailureError:
		return brokerapi.UpdateServiceSpec{IsAsync: true}, adapterToAPIError(ctx, err)
	case error:
		return errs(
			NewGenericError(ctx, fmt.Errorf("error deploying instance: %s", err)),
		)
	}

	ctx = brokercontext.WithBoshTaskID(ctx, boshTaskID)

	operationData, err := json.Marshal(OperationData{
		BoshTaskID:    boshTaskID,
		OperationType: OperationTypeUpdate,
		BoshContextID: boshContextID,
		PlanID:        operationDataPlanID,
	})
	if err != nil {
		return errs(NewGenericError(ctx, err))
	}

	return brokerapi.UpdateServiceSpec{IsAsync: true, OperationData: string(operationData)}, nil
}

func (b *Broker) asDisplayableError(err TaskError) DisplayableError {
	if b.featureFlags.CFUserTriggeredUpgrades() {
		return NewPendingChangesError(err)
	}
	if err.taskErrorType == ApplyChangesWithPendingChanges {
		return NewApplyChangesDisabledError(err)
	}
	return NewApplyChangesNotPermittedError(err)
}
