package cf

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"net/http"
)

func (c Client) GetServiceInstance(serviceInstanceGUID string, logger *log.Logger) (ServiceInstanceResource, error) {
	serviceInstance, err := c.getServiceInstance(serviceInstanceGUID, logger)
	if err != nil {
		return ServiceInstanceResource{}, errors.Wrap(err, fmt.Sprintf("failed to get service instance %q", serviceInstanceGUID))
	}

	return serviceInstance, err
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
