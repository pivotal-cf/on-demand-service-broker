// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package services

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
)

//go:generate counterfeiter -o fakes/fake_client.go . Client
type Client interface {
	Get(path string, query map[string]string) (*http.Response, error)
	Patch(path string) (*http.Response, error)
}

type BrokerServices struct {
	client    Client
	converter ResponseConverter
}

func NewBrokerServices(client Client) *BrokerServices {
	return &BrokerServices{
		client:    client,
		converter: ResponseConverter{},
	}
}

func (b *BrokerServices) Instances() ([]string, error) {
	response, err := b.client.Get("/mgmt/service_instances", nil)
	if err != nil {
		return nil, err
	}
	return b.converter.ListInstancesFrom(response)
}

func (b *BrokerServices) UpgradeInstance(instanceGUID string) (UpgradeOperation, error) {
	response, err := b.client.Patch(fmt.Sprintf("/mgmt/service_instances/%s", instanceGUID))
	if err != nil {
		return UpgradeOperation{}, err
	}
	return b.converter.UpgradeOperationFrom(response)
}

func (b *BrokerServices) LastOperation(instanceGUID string, operationData broker.OperationData) (brokerapi.LastOperation, error) {
	asJSON, err := json.Marshal(operationData)
	if err != nil {
		return brokerapi.LastOperation{}, err
	}

	query := map[string]string{"operation": string(asJSON)}
	response, err := b.client.Get(fmt.Sprintf("/v2/service_instances/%s/last_operation", instanceGUID), query)
	if err != nil {
		return brokerapi.LastOperation{}, err
	}
	return b.converter.LastOperationFrom(response)
}

func (b *BrokerServices) OrphanDeployments() ([]mgmtapi.Deployment, error) {
	response, err := b.client.Get("/mgmt/orphan_deployments", nil)
	if err != nil {
		return nil, err
	}

	return b.converter.OrphanDeploymentsFrom(response)
}
