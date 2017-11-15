// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package cf

import (
	"fmt"
	"log"

	s "github.com/pivotal-cf/on-demand-service-broker/service"
)

type Client struct {
	httpJsonClient
	url string
}

func New(
	url string,
	authHeaderBuilder AuthHeaderBuilder,
	trustedCertPEM []byte,
	disableTLSCertVerification bool,
) (Client, error) {
	httpClient, err := newWrappedHttpClient(authHeaderBuilder, trustedCertPEM, disableTLSCertVerification)
	if err != nil {
		return Client{}, err
	}
	return Client{httpJsonClient: httpClient, url: url}, nil
}

func (c Client) CountInstancesOfServiceOffering(serviceID string, logger *log.Logger) (map[ServicePlan]int, error) {
	plans, err := c.getPlansForServiceID(serviceID, logger)
	if err != nil {
		return map[ServicePlan]int{}, err
	}

	output := map[ServicePlan]int{}
	for _, plan := range plans {
		count, err := c.countServiceInstancesOfServicePlan(plan.ServicePlanEntity.ServiceInstancesUrl, logger)
		if err != nil {
			return nil, err
		}
		output[plan] = count
	}

	return output, nil
}

func (c Client) GetInstanceState(serviceInstanceGUID string, logger *log.Logger) (InstanceState, error) {
	instance, err := c.getServiceInstance(serviceInstanceGUID, logger)
	if err != nil {
		return InstanceState{}, err
	}

	plan, err := c.getServicePlan(instance.Entity.ServicePlanURL, logger)
	if err != nil {
		return InstanceState{}, err
	}

	return InstanceState{
		PlanID:              plan.ServicePlanEntity.UniqueID,
		OperationInProgress: instance.Entity.LastOperation.State == OperationStateInProgress,
	}, nil
}

func (c Client) GetInstance(serviceInstanceGUID string, logger *log.Logger) (Instance, error) {
	instance, err := c.getServiceInstance(serviceInstanceGUID, logger)
	return Instance{
		LastOperation: LastOperation{
			State: instance.Entity.LastOperation.State,
			Type:  instance.Entity.LastOperation.Type,
		},
	}, err
}

func (c Client) CountInstancesOfPlan(serviceID, servicePlanID string, logger *log.Logger) (int, error) {
	plans, err := c.getPlansForServiceID(serviceID, logger)
	if err != nil {
		return 0, err
	}

	for _, plan := range plans {
		if plan.ServicePlanEntity.UniqueID == servicePlanID {
			count, err := c.countServiceInstancesOfServicePlan(plan.ServicePlanEntity.ServiceInstancesUrl, logger)
			if err != nil {
				return 0, err
			}
			return count, nil
		}
	}

	return 0, fmt.Errorf("service plan %s not found for service %s", servicePlanID, serviceID)
}

func (c Client) GetInstancesOfServiceOffering(serviceOfferingID string, logger *log.Logger) ([]s.Instance, error) {
	plans, err := c.getPlansForServiceID(serviceOfferingID, logger)
	if err != nil {
		return nil, err
	}

	var instances []s.Instance
	for _, plan := range plans {
		path := fmt.Sprintf(
			"/v2/service_plans/%s/service_instances?results-per-page=%d",
			plan.Metadata.GUID,
			defaultPerPage,
		)

		for path != "" {
			var serviceInstancesResp serviceInstancesResponse

			instancesURL := fmt.Sprintf("%s%s", c.url, path)

			err := c.get(instancesURL, &serviceInstancesResp, logger)
			if err != nil {
				return nil, err
			}
			for _, instance := range serviceInstancesResp.ServiceInstances {
				instances = append(
					instances,
					s.Instance{
						GUID:         instance.Metadata.GUID,
						PlanUniqueID: plan.ServicePlanEntity.UniqueID,
					},
				)
			}
			path = serviceInstancesResp.NextPath
		}
	}
	return instances, nil
}

func (c Client) GetBindingsForInstance(instanceGUID string, logger *log.Logger) ([]Binding, error) {
	path := fmt.Sprintf(
		"/v2/service_instances/%s/service_bindings?results-per-page=%d",
		instanceGUID,
		defaultPerPage,
	)

	var bindings []Binding
	for path != "" {
		var bindingResponse bindingsResponse
		bindingsURL := fmt.Sprintf("%s%s", c.url, path)

		err := c.get(bindingsURL, &bindingResponse, logger)
		if err != nil {
			return nil, err
		}

		for _, bindingResource := range bindingResponse.Resources {
			bindings = append(bindings, Binding{
				GUID:    bindingResource.Metadata.GUID,
				AppGUID: bindingResource.Entity.AppGUID,
			})
		}

		path = bindingResponse.NextPath
	}

	return bindings, nil
}

func (c Client) DeleteBinding(binding Binding, logger *log.Logger) error {
	url := fmt.Sprintf(
		"%s/v2/apps/%s/service_bindings/%s",
		c.url,
		binding.AppGUID,
		binding.GUID,
	)

	return c.delete(url, logger)
}

func (c Client) GetServiceKeysForInstance(instanceGUID string, logger *log.Logger) ([]ServiceKey, error) {
	path := fmt.Sprintf(
		"/v2/service_instances/%s/service_keys?results-per-page=%d",
		instanceGUID,
		defaultPerPage,
	)

	var serviceKeys []ServiceKey
	for path != "" {
		var serviceKeyResponse serviceKeysResponse
		serviceKeysURL := fmt.Sprintf("%s%s", c.url, path)

		err := c.get(serviceKeysURL, &serviceKeyResponse, logger)
		if err != nil {
			return nil, err
		}

		for _, serviceKeyResource := range serviceKeyResponse.Resources {
			serviceKeys = append(serviceKeys, ServiceKey{
				GUID: serviceKeyResource.Metadata.GUID,
			})
		}

		path = serviceKeyResponse.NextPath
	}

	return serviceKeys, nil
}

func (c Client) DeleteServiceKey(serviceKey ServiceKey, logger *log.Logger) error {
	url := fmt.Sprintf(
		"%s/v2/service_keys/%s",
		c.url,
		serviceKey.GUID,
	)

	return c.delete(url, logger)
}

func (c Client) DeleteServiceInstance(instanceGUID string, logger *log.Logger) error {
	url := fmt.Sprintf(
		"%s/v2/service_instances/%s?accepts_incomplete=true",
		c.url,
		instanceGUID,
	)

	return c.delete(url, logger)
}

func (c Client) GetAPIVersion(logger *log.Logger) (string, error) {
	var infoResponse infoResponse
	err := c.get(fmt.Sprintf("%s/v2/info", c.url), &infoResponse, logger)
	if err != nil {
		return "", err
	}
	return infoResponse.APIVersion, nil
}

func (c Client) GetServiceOfferingGUID(brokerName string, logger *log.Logger) (string, error) {
	var (
		brokers    []ServiceBroker
		brokerGUID string
		err        error
	)

	path := "/v2/service_brokers"
	for path != "" {
		var response serviceBrokerResponse
		fullPath := fmt.Sprintf("%s%s", c.url, path)

		err = c.get(fullPath, &response, logger)
		if err != nil {
			return "", err
		}

		for _, r := range response.Resources {
			brokers = append(brokers, ServiceBroker{
				GUID: r.Metadata.GUID,
				Name: r.Entity.Name,
			})
		}

		path = response.NextPath
	}

	for _, broker := range brokers {
		if broker.Name == brokerName {
			brokerGUID = broker.GUID
		}
	}

	if brokerGUID == "" {
		return "", fmt.Errorf("Failed to find broker with name: %s", brokerName)
	}

	return brokerGUID, nil
}

func (c Client) DisableServiceAccess(serviceOfferingID string, logger *log.Logger) error {
	plans, err := c.getPlansForServiceID(serviceOfferingID, logger)
	if err != nil {
		return err
	}

	publicFalse := `{"public":false}`
	for _, p := range plans {
		err := c.put(fmt.Sprintf("%s/v2/service_plans/%s", c.url, p.Metadata.GUID), publicFalse, logger)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c Client) DeregisterBroker(brokerGUID string, logger *log.Logger) error {
	return c.delete(fmt.Sprintf("%s/v2/service_brokers/%s", c.url, brokerGUID), logger)
}

func (c Client) getPlansForServiceID(serviceID string, logger *log.Logger) ([]ServicePlan, error) {
	requiredService, err := c.findServiceByUniqueID(serviceID, logger)
	if err != nil {
		return nil, err
	}

	if requiredService == nil {
		return nil, nil
	}

	return c.listAllPlans(requiredService.ServiceEntity.ServicePlansUrl, logger)
}

func (c Client) listServices(path string, logger *log.Logger) (serviceResponse, error) {
	resp := serviceResponse{}
	return resp, c.get(fmt.Sprintf("%s%s", c.url, path), &resp, logger)
}

func (c Client) findServiceByUniqueID(uniqueID string, logger *log.Logger) (*service, error) {
	path := fmt.Sprintf("/v2/services?results-per-page=%d", defaultPerPage)
	for path != "" {
		resp, err := c.listServices(path, logger)
		if err != nil {
			return nil, err
		}
		if service := resp.Services.findByUniqueID(uniqueID); service != nil {
			return service, nil
		}
		path = resp.NextPath
	}

	return nil, nil
}

func (c Client) getServiceInstance(serviceInstanceGUID string, logger *log.Logger) (serviceInstanceResource, error) {
	path := fmt.Sprintf("/v2/service_instances/%s", serviceInstanceGUID)
	var instance serviceInstanceResource
	err := c.get(fmt.Sprintf("%s%s", c.url, path), &instance, logger)
	return instance, err
}

func (c Client) getServicePlan(servicePlanPath string, logger *log.Logger) (ServicePlan, error) {
	var plan ServicePlan
	err := c.get(fmt.Sprintf("%s%s", c.url, servicePlanPath), &plan, logger)
	return plan, err
}

func (c Client) listAllPlans(path string, logger *log.Logger) ([]ServicePlan, error) {
	plans := []ServicePlan{}
	path = fmt.Sprintf("%s?results-per-page=%d", path, defaultPerPage)
	for path != "" {
		servicePlanResponse, err := c.listPlans(path, logger)
		if err != nil {
			return nil, err
		}
		plans = append(plans, servicePlanResponse.ServicePlans...)
		path = servicePlanResponse.NextPath
	}

	return plans, nil
}

func (c Client) listPlans(path string, logger *log.Logger) (ServicePlanResponse, error) {
	servicePlanResponse := ServicePlanResponse{}
	return servicePlanResponse, c.get(fmt.Sprintf("%s%s", c.url, path), &servicePlanResponse, logger)
}

func (c Client) countServiceInstancesOfServicePlan(path string, logger *log.Logger) (int, error) {
	resp := serviceInstancesResponse{}
	err := c.get(fmt.Sprintf("%s%s?results-per-page=%d", c.url, path, defaultPerPage), &resp, logger)
	if err != nil {
		return 0, err
	}
	return resp.TotalResults, nil
}
