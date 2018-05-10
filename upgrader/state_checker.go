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

package upgrader

import (
	"fmt"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
)

type LastOperationChecker struct {
	brokerServices BrokerServices
}

func NewStateChecker(brokerServices BrokerServices) *LastOperationChecker {
	return &LastOperationChecker{
		brokerServices: brokerServices,
	}
}

func (l *LastOperationChecker) Check(guid string, operationData broker.OperationData) (services.UpgradeOperation, error) {
	lastOperation, err := l.brokerServices.LastOperation(guid, operationData)
	if err != nil {
		return services.UpgradeOperation{}, fmt.Errorf("error getting last operation: %s", err)
	}

	upgradeOperation := services.UpgradeOperation{Data: operationData, Description: lastOperation.Description}

	switch lastOperation.State {
	case brokerapi.Failed:
		upgradeOperation.Type = services.UpgradeFailed
	case brokerapi.Succeeded:
		upgradeOperation.Type = services.UpgradeSucceeded
	case brokerapi.InProgress:
		upgradeOperation.Type = services.UpgradeAccepted
	default:
		return services.UpgradeOperation{}, fmt.Errorf("uknown state from last operation: %s", lastOperation.State)
	}

	return upgradeOperation, nil
}
