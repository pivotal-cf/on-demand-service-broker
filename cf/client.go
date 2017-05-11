// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package cf

import (
	"fmt"
	"log"
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

func (c Client) CountInstancesOfServiceOffering(serviceID string, logger *log.Logger) (map[string]int, error) {
	plans, err := c.getPlansForServiceID(serviceID, logger)
	if err != nil {
		return map[string]int{}, err
	}

	output := map[string]int{}
	for _, plan := range plans {
		count, err := c.countServiceInstancesOfServicePlan(plan.ServicePlanEntity.ServiceInstancesUrl, logger)
		if err != nil {
			return nil, err
		}
		output[plan.ServicePlanEntity.UniqueID] = count
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

func (c Client) GetInstancesOfServiceOffering(serviceOfferingID string, logger *log.Logger) ([]string, error) {
	plans, err := c.getPlansForServiceID(serviceOfferingID, logger)
	if err != nil {
		return nil, err
	}

	var instances []string
	for _, plan := range plans {
		path := fmt.Sprintf(
			"/v2/service_plans/%s/service_instances?results-per-page=%d",
			plan.Metadata.GUID,
			defaultPerPage,
		)

		for path != "" {
			var serviceInstancesResp serviceInstancesResponse

			instancesURL := fmt.Sprintf("%s%s", c.url, path)

			err := c.Get(instancesURL, &serviceInstancesResp, logger)
			if err != nil {
				return nil, err
			}
			for _, instance := range serviceInstancesResp.ServiceInstances {
				instances = append(instances, instance.Metadata.GUID)
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

		err := c.Get(bindingsURL, &bindingResponse, logger)
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

	return c.Delete(url, logger)
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

		err := c.Get(serviceKeysURL, &serviceKeyResponse, logger)
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

	return c.Delete(url, logger)
}

func (c Client) DeleteServiceInstance(instanceGUID string, logger *log.Logger) error {
	url := fmt.Sprintf(
		"%s/v2/service_instances/%s?accepts_incomplete=true",
		c.url,
		instanceGUID,
	)

	return c.Delete(url, logger)
}

func (c Client) GetAPIVersion(logger *log.Logger) (string, error) {
	var infoResponse infoResponse
	err := c.Get(fmt.Sprintf("%s/v2/info", c.url), &infoResponse, logger)
	if err != nil {
		return "", err
	}
	return infoResponse.APIVersion, nil
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
	return resp, c.Get(fmt.Sprintf("%s%s", c.url, path), &resp, logger)
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
	err := c.Get(fmt.Sprintf("%s%s", c.url, path), &instance, logger)
	return instance, err
}

func (c Client) getServicePlan(servicePlanPath string, logger *log.Logger) (ServicePlan, error) {
	var plan ServicePlan
	err := c.Get(fmt.Sprintf("%s%s", c.url, servicePlanPath), &plan, logger)
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
	return servicePlanResponse, c.Get(fmt.Sprintf("%s%s", c.url, path), &servicePlanResponse, logger)
}

func (c Client) countServiceInstancesOfServicePlan(path string, logger *log.Logger) (int, error) {
	resp := serviceInstancesResponse{}
	err := c.Get(fmt.Sprintf("%s%s?results-per-page=%d", c.url, path, defaultPerPage), &resp, logger)
	if err != nil {
		return 0, err
	}
	return resp.TotalResults, nil
}
