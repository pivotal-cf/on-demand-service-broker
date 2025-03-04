// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"code.cloudfoundry.org/brokerapi/v13/domain"
	"code.cloudfoundry.org/brokerapi/v13/domain/apiresponses"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

func (b *Broker) getDeploymentInfo(instanceID string, ctx context.Context, action string, logger *log.Logger) ([]byte, bosh.BoshVMs, BrokerError) {
	manifest, found, err := b.boshClient.GetDeployment(deploymentName(instanceID), logger)
	if err != nil {
		return nil, nil, NewGenericError(ctx, fmt.Errorf("gathering deployment list %s", err))
	}
	if !found {
		return nil, nil, NewDisplayableError(apiresponses.ErrInstanceDoesNotExist, fmt.Errorf("error %sing: instance %s, not found", action, instanceID))
	}

	vms, err := b.boshClient.VMs(deploymentName(instanceID), logger)
	if err != nil {
		return nil, nil, NewGenericError(ctx, fmt.Errorf("gathering %sing info %s", action, err))
	}

	return manifest, vms, nil
}

func convertDetailsToMap(details domain.DetailsWithRawParameters) (map[string]interface{}, error) {
	arbitraryParams := map[string]interface{}{}

	if len(details.GetRawParameters()) > 0 {
		if err := json.Unmarshal(details.GetRawParameters(), &arbitraryParams); err != nil {
			return nil, apiresponses.ErrRawParamsInvalid
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
