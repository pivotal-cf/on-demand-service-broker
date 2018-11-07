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
	"errors"
	"fmt"

	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

type OperationTriggerer struct {
	brokerServices BrokerServices
	instanceLister InstanceLister
	logger         Listener
	operationType  string
}

func NewTriggerer(brokerServices BrokerServices, instanceLister InstanceLister, listener Listener, processType string) (*OperationTriggerer, error) {
	var operationType string
	if processType == "upgrade-all" {
		operationType = "upgrade"
	} else {
		return &OperationTriggerer{}, errors.New(fmt.Sprintf("Invalid process type: %s", processType))
	}

	return &OperationTriggerer{
		brokerServices: brokerServices,
		instanceLister: instanceLister,
		logger:         listener,
		operationType:  operationType,
	}, nil
}

func (t *OperationTriggerer) TriggerOperation(instance service.Instance) (services.BOSHOperation, error) {
	latestInstance, err := t.instanceLister.LatestInstanceInfo(instance)
	if err != nil {
		if err == service.InstanceNotFound {
			return services.BOSHOperation{Type: services.InstanceNotFound}, nil
		}
		latestInstance = instance
		t.logger.FailedToRefreshInstanceInfo(instance.GUID)
	}

	operation, err := t.brokerServices.ProcessInstance(latestInstance)
	if err != nil {
		return services.BOSHOperation{}, fmt.Errorf("Operation type: %s failed for service instance %s: %s", t.operationType, instance.GUID, err)
	}

	return operation, nil
}
