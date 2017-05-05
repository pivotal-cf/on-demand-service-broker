// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package brokerclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/craigfurman/herottp"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
)

type BrokerServicesHTTPClient struct {
	username  string
	password  string
	url       string
	client    *herottp.Client
	converter ResponseConverter
}

func NewBrokerServicesHTTPClient(username, password, url string, timeout time.Duration) BrokerServicesHTTPClient {
	config := herottp.Config{
		Timeout: timeout,
	}
	return BrokerServicesHTTPClient{
		username:  username,
		password:  password,
		url:       url,
		client:    herottp.New(config),
		converter: ResponseConverter{},
	}
}

func (b BrokerServicesHTTPClient) Instances() ([]string, error) {
	response, err := b.responseTo("GET", "/mgmt/service_instances")
	if err != nil {
		return nil, err
	}
	return b.converter.ListInstancesFrom(response)
}

func (b BrokerServicesHTTPClient) UpgradeInstance(instanceGUID string) (UpgradeOperation, error) {
	response, err := b.responseTo("PATCH", fmt.Sprintf("/mgmt/service_instances/%s", instanceGUID))
	if err != nil {
		return UpgradeOperation{}, err
	}
	return b.converter.UpgradeOperationFrom(response)
}

func (b BrokerServicesHTTPClient) LastOperation(instanceGUID string, operationData broker.OperationData) (brokerapi.LastOperation, error) {
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

func (b BrokerServicesHTTPClient) responseTo(verb, path string) (*http.Response, error) {
	request, err := http.NewRequest(verb, fmt.Sprintf("%s%s", b.url, path), nil)
	if err != nil {
		return nil, err
	}

	request.SetBasicAuth(b.username, b.password)

	return b.client.Do(request)
}
