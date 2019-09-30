package cf

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/pkg/errors"
)

func (c Client) GetServiceInstance(serviceInstanceGUID string, logger *log.Logger) (ServiceInstanceResource, error) {
	path := fmt.Sprintf("/v2/service_instances/%s", serviceInstanceGUID)
	var instance ServiceInstanceResource
	err := c.get(fmt.Sprintf("%s%s", c.url, path), &instance, logger)
	return instance, err
}

func (c Client) UpgradeServiceInstance(serviceInstanceGUID string, maintenanceInfo MaintenanceInfo, logger *log.Logger) (LastOperation, error) {
	path := fmt.Sprintf(`%s/v2/service_instances/%s?accepts_incomplete=true`, c.url, serviceInstanceGUID)

	requestBody, err := serialiseMaintenanceInfo(maintenanceInfo)
	if err != nil {
		return LastOperation{}, errors.Wrap(err, "failed to serialize request body")
	}

	resp, err := c.put(path, requestBody, logger)
	if err != nil {
		return LastOperation{}, err
	}
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return LastOperation{},
			fmt.Errorf("unexpected response status %d when upgrading service instance %q; response body %q", resp.StatusCode, serviceInstanceGUID, string(body))
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var parsedResponse ServiceInstanceResource
	err = json.Unmarshal(body, &parsedResponse)
	if err != nil {
		return LastOperation{}, errors.Wrap(err, "failed to de-serialise the response body")
	}

	return parsedResponse.Entity.LastOperation, nil
}

func (c Client) DeleteServiceInstance(instanceGUID string, logger *log.Logger) error {
	url := fmt.Sprintf(
		"%s/v2/service_instances/%s?accepts_incomplete=true",
		c.url,
		instanceGUID,
	)

	return c.delete(url, logger)
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

func serialiseMaintenanceInfo(maintenanceInfo MaintenanceInfo) (string, error) {
	var requestBody struct {
		MaintenanceInfo MaintenanceInfo `json:"maintenance_info"`
	}
	requestBody.MaintenanceInfo = maintenanceInfo
	serialisedRequestBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}
	return string(serialisedRequestBody), nil
}
