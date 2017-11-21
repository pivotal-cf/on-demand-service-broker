// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

//go:generate counterfeiter -o fakes/fake_http_client.go . HTTPClient
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type BrokerServices struct {
	client            HTTPClient
	authHeaderBuilder authorizationheader.AuthHeaderBuilder
	converter         ResponseConverter
	baseURL           string
}

func NewBrokerServices(client HTTPClient, authHeaderBuilder authorizationheader.AuthHeaderBuilder, baseURL string) *BrokerServices {
	return &BrokerServices{
		client:            client,
		authHeaderBuilder: authHeaderBuilder,
		converter:         ResponseConverter{},
		baseURL:           baseURL,
	}
}

func (b *BrokerServices) UpgradeInstance(instance service.Instance) (UpgradeOperation, error) {
	body := strings.NewReader(fmt.Sprintf(`{"plan_id": "%s"}`, instance.PlanUniqueID))
	//TODO missing error test case
	request, _ := http.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("%s/mgmt/service_instances/%s", b.baseURL, instance.GUID),
		body)

	//TODO get logger from main
	logger := new(log.Logger)

	// missing error test case
	_ = b.authHeaderBuilder.AddAuthHeader(request, logger)
	response, err := b.client.Do(request)
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
	path := fmt.Sprintf("/v2/service_instances/%s/last_operation", instanceGUID)

	// TODO missing error test case
	url := b.buildURL(path)
	urlWithQuery := appendQuery(url, query)

	//TODO missing error test case
	request, _ := http.NewRequest(
		http.MethodGet,
		urlWithQuery,
		nil)

	//TODO get logger from main
	logger := new(log.Logger)

	// missing error test case
	b.authHeaderBuilder.AddAuthHeader(request, logger)
	response, err := b.client.Do(request)
	if err != nil {
		return brokerapi.LastOperation{}, err
	}
	return b.converter.LastOperationFrom(response)
}

func (b *BrokerServices) OrphanDeployments() ([]mgmtapi.Deployment, error) {
	response, err := b.doRequest(http.MethodGet, "/mgmt/orphan_deployments", nil)
	if err != nil {
		return nil, err
	}

	return b.converter.OrphanDeploymentsFrom(response)
}

func (b *BrokerServices) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	request, err := http.NewRequest(
		method,
		b.buildURL(path),
		nil)

	if err != nil {
		return nil, err
	}

	logger := new(log.Logger)
	b.authHeaderBuilder.AddAuthHeader(request, logger)
	return b.client.Do(request)
}

func (b *BrokerServices) buildURL(path string) string {
	baseURL := b.baseURL
	if strings.HasSuffix(b.baseURL, "/") {
		baseURL = strings.TrimRight(b.baseURL, "/")
	}

	if !strings.HasPrefix(path, "/") && path != "" {
		path = "/" + path
	}

	return baseURL + path
}

func appendQuery(u string, query map[string]string) string {
	values := url.Values{}
	for param, value := range query {
		values.Set(param, value)
	}
	return u + "?" + values.Encode()
}
