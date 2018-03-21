// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package noopservicescontroller

import (
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

type Client struct {
}

func (Client) GetAPIVersion(logger *log.Logger) (string, error) {
	return broker.MinimumCFVersion, nil
}

func (Client) CountInstancesOfPlan(serviceOfferingID, planID string, logger *log.Logger) (int, error) {
	return 1, nil
}

func (Client) CountInstancesOfServiceOffering(serviceOfferingID string, logger *log.Logger) (instanceCountByPlanID map[cf.ServicePlan]int, err error) {
	return make(map[cf.ServicePlan]int), nil
}

func (Client) GetInstanceState(serviceInstanceGUID string, logger *log.Logger) (cf.InstanceState, error) {
	return cf.InstanceState{}, nil
}

func (Client) GetInstancesOfServiceOffering(serviceOfferingID string, logger *log.Logger) ([]service.Instance, error) {
	return []service.Instance{}, nil
}

func (Client) GetInstancesOfServiceOfferingByOrgSpace(serviceOfferingID, org, space string, logger *log.Logger) ([]service.Instance, error) {
	return []service.Instance{}, nil
}

func New() Client {
	return Client{}
}
