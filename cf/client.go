// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package cf

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/pkg/errors"

	s "github.com/pivotal-cf/on-demand-service-broker/service"
)

type CFResponse struct {
	Resources []struct {
		Entity   map[string]interface{} `json:"entity"`
		Metadata map[string]interface{} `json:"metadata"`
	} `json:"resources"`
}

type Client struct {
	httpJsonClient
	url    string
	logger *log.Logger
}

func New(url string, authHeaderBuilder AuthHeaderBuilder, trustedCertPEM []byte, disableTLSCertVerification bool, logger *log.Logger) (Client, error) {
	httpClient, err := newWrappedHttpClient(authHeaderBuilder, trustedCertPEM, disableTLSCertVerification)
	if err != nil {
		return Client{}, err
	}
	return Client{httpJsonClient: httpClient, url: url, logger: logger}, nil
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

func (c Client) GetInstancesOfServiceOfferingByOrgSpace(serviceOfferingID, orgName, spaceName string, logger *log.Logger) ([]s.Instance, error) {
	plans, err := c.getPlansForServiceID(serviceOfferingID, logger)
	if err != nil {
		return nil, err
	}

	orgResponse, err := c.getOrganization(orgName, logger)
	if err != nil {
		return nil, err
	}

	if len(orgResponse.Resources) == 0 {
		return []s.Instance{}, nil
	}

	spaceURL := fmt.Sprintf("%s%s?q=name:%s",
		c.url,
		orgResponse.Resources[0].Entity["spaces_url"],
		spaceName,
	)

	var spaceResponse CFResponse

	if err = c.get(spaceURL, &spaceResponse, logger); err != nil {
		return nil, err
	}
	if len(spaceResponse.Resources) == 0 {
		return []s.Instance{}, nil
	}

	query := fmt.Sprintf("&q=space_guid:%s", spaceResponse.Resources[0].Metadata["guid"])
	return c.getInstances(plans, query, logger)
}

func (c Client) GetInstancesOfServiceOffering(serviceOfferingID string, logger *log.Logger) ([]s.Instance, error) {
	plans, err := c.getPlansForServiceID(serviceOfferingID, logger)
	if err != nil {
		return nil, err
	}

	return c.getInstances(plans, "", logger)
}

func (c Client) getInstances(plans []ServicePlan, query string, logger *log.Logger) ([]s.Instance, error) {
	instances := []s.Instance{}
	for _, plan := range plans {
		path := fmt.Sprintf(
			"/v2/service_plans/%s/service_instances?results-per-page=%d%s",
			plan.Metadata.GUID,
			defaultPerPage,
			query,
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
	)

	brokers, err := c.listServiceBrokers(logger)
	if err != nil {
		return "", err
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

func (c Client) CreateServicePlanVisibility(orgName string, serviceOfferingID string, planName string, logger *log.Logger) error {
	orgResponse, err := c.getOrganization(orgName, logger)
	if err != nil {
		return errors.Wrap(err, "failed to create service plan visibility")
	}
	if len(orgResponse.Resources) == 0 {
		return fmt.Errorf("failed to find org with name %q", orgName)
	}
	orgGUID := orgResponse.Resources[0].Metadata["guid"]

	plans, err := c.getPlansForServiceID(serviceOfferingID, logger)
	if err != nil {
		return errors.Wrap(err, "failed to create service plan visibility")
	}

	planGUID := findPlanGUID(plans, planName)
	if planGUID == "" {
		return fmt.Errorf(`planID %q not found while updating plan access`, planName)
	}

	path := fmt.Sprintf("%s/v2/service_plan_visibilities", c.url)
	body := bytes.NewBuffer([]byte(
		fmt.Sprintf(`{"service_plan_guid":"%s","organization_guid":"%s"}`, planGUID, orgGUID),
	))
	response, err := c.post(path, body, logger)
	if err != nil {
		return errors.Wrap(err, "failed to create service plan visibility")
	}
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code %d when creating service plan visibility", response.StatusCode)
	}

	return nil
}

func (c Client) getServicePlanVisibilities(planGUID string) ([]ServicePlanVisibility, error) {
	path := fmt.Sprintf(
		"/v2/service_plan_visibilities?q=service_plan_guid:%s&results-per-page=%d",
		planGUID,
		defaultPerPage,
	)

	visibilities := []ServicePlanVisibility{}

	for path != "" {
		var visibilityResponse visibilityResponse
		visibilityURL := fmt.Sprintf("%s%s", c.url, path)

		err := c.get(visibilityURL, &visibilityResponse, c.logger)
		if err != nil {
			return nil, err
		}

		visibilities = append(visibilities, visibilityResponse.Resources...)

		path = visibilityResponse.NextPath
	}

	return visibilities, nil
}

func (c Client) deleteServicePlanVisibilities(planGUID string) error {
	vs, err := c.getServicePlanVisibilities(planGUID)
	if err != nil {
		return errors.Wrap(
			err,
			fmt.Sprintf("failed to get plan visibilities for plan %s", planGUID),
		)
	}

	for _, v := range vs {
		err := c.delete(fmt.Sprintf("%s/v2/service_plan_visibilities/%s", c.url, v.Metadata.GUID), c.logger)
		if err != nil {
			return errors.Wrap(
				err,
				fmt.Sprintf("failed to delete plan visibility for plan %s", planGUID),
			)
		}
	}

	return nil
}

func (c Client) EnableServiceAccess(serviceOfferingID, planName string, logger *log.Logger) error {
	return c.manageServiceAccess(serviceOfferingID, planName, true, logger)
}

func (c Client) DisableServiceAccess(serviceOfferingID, planName string, logger *log.Logger) error {
	return c.manageServiceAccess(serviceOfferingID, planName, false, logger)
}

func (c Client) manageServiceAccess(serviceOfferingID string, planName string, isPublic bool, logger *log.Logger) error {
	plans, err := c.getPlansForServiceID(serviceOfferingID, logger)
	if err != nil {
		return err
	}

	planGUID := findPlanGUID(plans, planName)
	if planGUID == "" {
		return fmt.Errorf(`planID %q not found while updating plan access`, planName)
	}

	err = c.setAccessForPlan(planGUID, isPublic, logger)
	if err != nil {
		return err
	}

	if err := c.deleteServicePlanVisibilities(planGUID); err != nil {
		return errors.Wrap(
			err,
			fmt.Sprintf("failed to delete plan visibilities for plan %s", planGUID),
		)
	}
	return nil

}

func (c Client) DisableServiceAccessForAllPlans(serviceOfferingID string, logger *log.Logger) error {
	plans, err := c.getPlansForServiceID(serviceOfferingID, logger)
	if err != nil {
		return err
	}

	for _, p := range plans {
		err = c.setAccessForPlan(p.Metadata.GUID, false, logger)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c Client) DeregisterBroker(brokerGUID string, logger *log.Logger) error {
	return c.delete(fmt.Sprintf("%s/v2/service_brokers/%s", c.url, brokerGUID), logger)
}

func (c Client) CreateServiceBroker(name, username, password, url string) error {
	reqBody := bytes.NewBuffer([]byte(fmt.Sprintf(`{
		"name": "%s",
		"broker_url": "%s",
		"auth_username": "%s",
		"auth_password": "%s"
	}`, name, url, username, password)))

	path := fmt.Sprintf("%s/v2/service_brokers", c.url)

	resp, err := c.post(path, reqBody, c.logger)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusCreated {
		return nil
	}

	body, _ := ioutil.ReadAll(resp.Body)
	return errors.Wrap(
		fmt.Errorf("unexpected response status %d; response body %q", resp.StatusCode, string(body)),
		fmt.Sprintf("failed to create service broker %s", name),
	)
}

func (c Client) UpdateServiceBroker(brokerGUID, name, username, password, url string) error {
	reqBody := fmt.Sprintf(`{
		"name": "%s",
		"broker_url": "%s",
		"auth_username": "%s",
		"auth_password": "%s"
	}`, name, url, username, password)

	path := fmt.Sprintf("%s/v2/service_brokers/%s", c.url, brokerGUID)

	resp, err := c.put(path, reqBody, c.logger)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return errors.Wrap(
			fmt.Errorf("unexpected response status %d; response body %q", resp.StatusCode, string(body)),
			fmt.Sprintf("failed to update service broker %s", name),
		)
	}
	return nil
}

func (c Client) ServiceBrokers() ([]ServiceBroker, error) {
	return c.listServiceBrokers(c.logger)
}

func (c Client) setAccessForPlan(planGUID string, public bool, logger *log.Logger) error {
	body := fmt.Sprintf(`{"public":%v}`, public)
	resp, err := c.put(fmt.Sprintf("%s/v2/service_plans/%s", c.url, planGUID), body, logger)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return errors.Wrap(
			fmt.Errorf("unexpected response status %d; response body %q", resp.StatusCode, string(body)),
			fmt.Sprintf("failed to update service access for plan %s", planGUID),
		)
	}

	return nil
}

func findPlanGUID(plans []ServicePlan, planName string) string {
	planGUID := ""
	for _, p := range plans {
		if p.ServicePlanEntity.Name == planName {
			planGUID = p.Metadata.GUID
		}
	}
	return planGUID
}

func (c Client) getOrganization(orgName string, logger *log.Logger) (CFResponse, error) {
	var orgResponse CFResponse

	orgURL := fmt.Sprintf("%s/v2/organizations?q=name:%s", c.url, orgName)
	if err := c.get(orgURL, &orgResponse, logger); err != nil {
		return CFResponse{}, err
	}

	return orgResponse, nil
}

func (c Client) listServiceBrokers(logger *log.Logger) ([]ServiceBroker, error) {
	var err error
	var brokers []ServiceBroker

	path := "/v2/service_brokers"
	for path != "" {
		var response serviceBrokerResponse
		fullPath := fmt.Sprintf("%s%s", c.url, path)

		err = c.get(fullPath, &response, logger)
		if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve list of brokers")
		}

		for _, r := range response.Resources {
			brokers = append(brokers, ServiceBroker{
				GUID: r.Metadata.GUID,
				Name: r.Entity.Name,
			})
		}

		path = response.NextPath
	}

	return brokers, nil
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
