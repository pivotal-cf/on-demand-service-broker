// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/pivotal-cf/brokerapi/v11/domain"
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate -o fakes/fake_http_client.go . HTTPClient
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type BrokerServices struct {
	client            HTTPClient
	authHeaderBuilder authorizationheader.AuthHeaderBuilder
	converter         ResponseConverter
	baseURL           string
	logger            *log.Logger
}

var (
	InstanceNotFoundError = errors.New("Service instance not found")
)

func NewBrokerServices(client HTTPClient, authHeaderBuilder authorizationheader.AuthHeaderBuilder, baseURL string, logger *log.Logger) *BrokerServices {
	return &BrokerServices{
		client:            client,
		authHeaderBuilder: authHeaderBuilder,
		converter:         ResponseConverter{},
		baseURL:           baseURL,
		logger:            logger,
	}
}

func (b *BrokerServices) ProcessInstance(instance service.Instance, operationType string) (BOSHOperation, error) {
	body := strings.NewReader(fmt.Sprintf(`{"plan_id": "%s", "context":{"space_guid":"%s"}}`, instance.PlanUniqueID, instance.SpaceGUID))
	response, err := b.doRequest(
		http.MethodPatch,
		fmt.Sprintf("/mgmt/service_instances/%s?operation_type=%s", instance.GUID, operationType),
		body)
	if err != nil {
		return BOSHOperation{}, err
	}
	return b.converter.ExtractOperationFrom(response)
}

func (b *BrokerServices) LastOperation(instanceGUID string, operationData broker.OperationData) (domain.LastOperation, error) {
	asJSON, err := json.Marshal(operationData)
	if err != nil {
		return domain.LastOperation{}, err
	}

	query := map[string]string{"operation": string(asJSON)}
	path := fmt.Sprintf("/v2/service_instances/%s/last_operation", instanceGUID)
	pathWithQuery := appendQuery(path, query)

	response, err := b.doRequest(http.MethodGet, pathWithQuery, nil)
	if err != nil {
		return domain.LastOperation{}, err
	}

	return b.converter.LastOperationFrom(response)
}

func (b *BrokerServices) Instances(filter map[string]string) ([]service.Instance, error) {
	pathWithQuery := createRequestPath("/mgmt/service_instances", filter)

	response, err := b.doRequest(http.MethodGet, pathWithQuery, nil)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get service instances with status code: %d", response.StatusCode)
	}

	instances, err := decodeServiceInstanceResponse(response)
	if err != nil {
		return instances, fmt.Errorf("failed to decode service instance response body with error: %s", err)
	}
	return instances, nil
}

func (b *BrokerServices) LatestInstanceInfo(instance service.Instance) (service.Instance, error) {
	instances, err := b.Instances(nil)
	if err != nil {
		return service.Instance{}, err
	}
	for _, inst := range instances {
		if inst.GUID == instance.GUID {
			return inst, nil
		}
	}
	return service.Instance{}, InstanceNotFoundError
}

func (b *BrokerServices) OrphanDeployments() ([]mgmtapi.Deployment, error) {
	response, err := b.doRequest(http.MethodGet, "/mgmt/orphan_deployments", nil)
	if err != nil {
		return nil, err
	}

	return b.converter.OrphanDeploymentsFrom(response)
}

func (b *BrokerServices) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	request, err := http.NewRequest(method, b.buildURL(path), body)
	if err != nil {
		return nil, err
	}
	request.Header.Add("X-Broker-Api-Version", "2.13")

	err = b.authHeaderBuilder.AddAuthHeader(request, b.logger)
	if err != nil {
		return nil, err
	}

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

func decodeServiceInstanceResponse(response *http.Response) ([]service.Instance, error) {
	var instances []service.Instance
	decoder := json.NewDecoder(response.Body)
	err := decoder.Decode(&instances)
	return instances, err
}

func createRequestPath(path string, filter map[string]string) string {
	values := map[string]string{}
	for k, v := range filter {
		values[k] = v
	}
	pathWithQuery := appendQuery(path, values)
	return pathWithQuery
}

func appendQuery(u string, query map[string]string) string {
	values := url.Values{}
	for param, value := range query {
		values.Set(param, value)
	}
	return u + "?" + values.Encode()
}
