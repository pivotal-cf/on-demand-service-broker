// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package instanceiterator

import (
	"fmt"
	"github.com/pivotal-cf/brokerapi/v7/domain"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

type BOSHTriggerer struct {
	operationType  string
	brokerServices BrokerServices
}

func NewBOSHUpgradeTriggerer(brokerServices BrokerServices) *BOSHTriggerer {
	return &BOSHTriggerer{operationType: "upgrade", brokerServices: brokerServices}
}

func NewRecreateTriggerer(brokerServices BrokerServices) *BOSHTriggerer {
	return &BOSHTriggerer{operationType: "recreate", brokerServices: brokerServices}
}

func (t *BOSHTriggerer) TriggerOperation(instance service.Instance) (TriggeredOperation, error) {
	operation, err := t.brokerServices.ProcessInstance(instance, t.operationType)
	if err != nil {
		return TriggeredOperation{},
			fmt.Errorf(
				"operation type: %s failed for service instance %s: %s",
				t.operationType,
				instance.GUID,
				err,
			)
	}
	return t.translateTriggerResponse(operation), nil
}

func (t *BOSHTriggerer) Check(serviceInstanceGUID string, operationData broker.OperationData) (TriggeredOperation, error) {
	lastOperation, err := t.brokerServices.LastOperation(serviceInstanceGUID, operationData)
	if err != nil {
		return TriggeredOperation{}, fmt.Errorf("error getting last operation: %s", err)
	}

	return t.translateCheckResponse(lastOperation, operationData)
}

func (t *BOSHTriggerer) translateCheckResponse(lastOperation domain.LastOperation, operationData broker.OperationData) (TriggeredOperation, error) {
	var operationState OperationState
	switch lastOperation.State {
	case domain.Failed:
		operationState = OperationFailed
	case domain.Succeeded:
		operationState = OperationSucceeded
	case domain.InProgress:
		operationState = OperationAccepted
	default:
		return TriggeredOperation{}, fmt.Errorf("unknown state from last operation: %s", lastOperation.State)
	}

	return TriggeredOperation{
		State:       operationState,
		Data:        operationData,
		Description: lastOperation.Description,
	}, nil
}

func (t *BOSHTriggerer) translateTriggerResponse(boshOperation services.BOSHOperation) TriggeredOperation {
	var operationState OperationState
	switch boshOperation.Type {
	case services.OperationAccepted:
		operationState = OperationAccepted
	case services.OperationSkipped:
		operationState = OperationSkipped
	case services.OperationFailed:
		operationState = OperationFailed
	case services.OperationInProgress:
		operationState = OperationInProgress
	case services.InstanceNotFound:
		operationState = InstanceNotFound
	case services.OperationPending:
		operationState = OperationPending
	case services.OrphanDeployment:
		operationState = OrphanDeployment
	}

	return TriggeredOperation{
		State:       operationState,
		Data:        boshOperation.Data,
		Description: boshOperation.Description,
	}
}
