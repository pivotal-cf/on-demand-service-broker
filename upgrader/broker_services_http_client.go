// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrader

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/craigfurman/herottp"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader/broker_response"
	"time"
)

type BrokerServicesHTTPClient struct {
	username string
	password string
	url      string
	client   *herottp.Client
}

func NewBrokerServicesHTTPClient(username, password, url string, timeout time.Duration) BrokerServicesHTTPClient {
	config := herottp.Config{
		Timeout: timeout,
	}
	return BrokerServicesHTTPClient{
		username: username,
		password: password,
		url:      url,
		client:   herottp.New(config),
	}
}

func (u BrokerServicesHTTPClient) Instances() ([]string, error) {
	response, err := u.responseTo("GET", "/mgmt/service_instances")
	if err != nil {
		return nil, err
	}
	return broker_response.ListInstancesFrom(response)
}

func (u BrokerServicesHTTPClient) UpgradeInstance(instanceGUID string) (broker_response.UpgradeOperation, error) {
	response, err := u.responseTo("PATCH", fmt.Sprintf("/mgmt/service_instances/%s", instanceGUID))
	if err != nil {
		return broker_response.UpgradeOperation{}, err
	}
	return broker_response.UpgradeOperationFrom(response)
}

func (u BrokerServicesHTTPClient) LastOperation(instanceGUID string, operationData broker.OperationData) (brokerapi.LastOperation, error) {
	asJSON, err := json.Marshal(operationData)
	if err != nil {
		return brokerapi.LastOperation{}, err
	}

	operationQueryStr := url.QueryEscape(string(asJSON))
	response, err := u.responseTo("GET", fmt.Sprintf("/v2/service_instances/%s/last_operation?operation=%s", instanceGUID, operationQueryStr))
	if err != nil {
		return brokerapi.LastOperation{}, err
	}
	return broker_response.LastOperationFrom(response)
}

func (u BrokerServicesHTTPClient) responseTo(verb, path string) (*http.Response, error) {
	request, err := http.NewRequest(verb, fmt.Sprintf("%s%s", u.url, path), nil)
	if err != nil {
		return nil, err
	}

	request.SetBasicAuth(u.username, u.password)

	return u.client.Do(request)
}
