// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

func (b *Broker) getDeploymentInfo(instanceID string, logger *log.Logger) (bosh.BoshVMs, []byte, error) {
	vms, err := b.boshClient.VMs(deploymentName(instanceID), logger)
	if err != nil {
		return nil, nil, err
	}
	manifest, found, err := b.boshClient.GetDeployment(deploymentName(instanceID), logger)
	if !found {
		return nil, nil, fmt.Errorf("manifest not found for deployment: %s", instanceID)
	}

	return vms, manifest, err
}

func convertDetailsToMap(details brokerapi.DetailsWithRawParameters) (map[string]interface{}, error) {
	var arbitraryParams map[string]interface{}

	if len(details.GetRawParameters()) > 0 {
		arbitraryParams = map[string]interface{}{}
		if err := json.Unmarshal(details.GetRawParameters(), &arbitraryParams); err != nil {
			return nil, brokerapi.ErrRawParamsInvalid
		}
	}

	requestParams, err := convertToMap(details)
	if err != nil {
		return nil, err
	}

	requestParams["parameters"] = arbitraryParams

	return requestParams, nil
}

func convertToMap(object interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}

	genericMap := map[string]interface{}{}
	err = json.Unmarshal(data, &genericMap)
	if err != nil {
		return nil, err
	}
	return genericMap, nil
}
