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

	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

type UpgradeTriggerer struct {
	brokerServices BrokerServices
}

func NewUpgradeTriggerer(brokerServices BrokerServices) *UpgradeTriggerer {
	return &UpgradeTriggerer{
		brokerServices: brokerServices,
	}
}

func (t *UpgradeTriggerer) TriggerOperation(instance service.Instance) (services.BOSHOperation, error) {
	operation, err := t.brokerServices.ProcessInstance(instance, "upgrade")
	if err != nil {
		return services.BOSHOperation{}, fmt.Errorf("Operation type: upgrade failed for service instance %s: %s", instance.GUID, err)
	}
	return operation, nil
}

type RecreateTriggerer struct {
	brokerServices BrokerServices
}

func NewRecreateTriggerer(brokerServices BrokerServices) *RecreateTriggerer {
	return &RecreateTriggerer{
		brokerServices: brokerServices,
	}
}

func (t *RecreateTriggerer) TriggerOperation(instance service.Instance) (services.BOSHOperation, error) {
	operation, err := t.brokerServices.ProcessInstance(instance, "recreate")
	if err != nil {
		return services.BOSHOperation{}, fmt.Errorf("Operation type: recreate failed for service instance %s: %s", instance.GUID, err)
	}
	return operation, nil
}
