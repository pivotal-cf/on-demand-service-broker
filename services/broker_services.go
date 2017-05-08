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
	"net/url"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
)

type HTTPClient interface {
	Do(request *http.Request) (*http.Response, error)
}

type BrokerServices struct {
	username  string
	password  string
	url       string
	client    HTTPClient
	converter ResponseConverter
}

func NewBrokerServices(username, password, url string, client HTTPClient) BrokerServices {
	return BrokerServices{
		username:  username,
		password:  password,
		url:       url,
		client:    client,
		converter: ResponseConverter{},
	}
}

func (b BrokerServices) Instances() ([]string, error) {
	response, err := b.responseTo("GET", "/mgmt/service_instances")
	if err != nil {
		return nil, err
	}
	return b.converter.ListInstancesFrom(response)
}

func (b BrokerServices) UpgradeInstance(instanceGUID string) (UpgradeOperation, error) {
	response, err := b.responseTo("PATCH", fmt.Sprintf("/mgmt/service_instances/%s", instanceGUID))
	if err != nil {
		return UpgradeOperation{}, err
	}
	return b.converter.UpgradeOperationFrom(response)
}

func (b BrokerServices) LastOperation(instanceGUID string, operationData broker.OperationData) (brokerapi.LastOperation, error) {
	asJSON, err := json.Marshal(operationData)
	if err != nil {
		return brokerapi.LastOperation{}, err
	}

	operationQueryStr := url.QueryEscape(string(asJSON))
	response, err := b.responseTo("GET", fmt.Sprintf("/v2/service_instances/%s/last_operation?operation=%s", instanceGUID, operationQueryStr))
	if err != nil {
		return brokerapi.LastOperation{}, err
	}
	return b.converter.LastOperationFrom(response)
}

func (b BrokerServices) responseTo(verb, path string) (*http.Response, error) {
	request, err := http.NewRequest(verb, fmt.Sprintf("%s%s", b.url, path), nil)
	if err != nil {
		return nil, err
	}

	request.SetBasicAuth(b.username, b.password)

	return b.client.Do(request)
}
