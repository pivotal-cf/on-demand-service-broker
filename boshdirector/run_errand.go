// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type Instance struct {
	Group string `json:"group"`
	ID    string `json:"id,omitempty"`
}

func (c *Client) RunErrand(deploymentName, errandName string, errandInstances []string, contextID string, logger *log.Logger) (int, error) {
	logger.Printf("running errand %s on colocated instances %v from deployment %s\n", errandName, errandInstances, deploymentName)

	var errandBody struct {
		Instances []Instance `json:"instances,omitempty"`
	}

	for _, errandInstance := range errandInstances {
		collection := strings.Split(errandInstance, "/")
		switch len(collection) {
		case 1:
			group := collection[0]
			errandBody.Instances = append(errandBody.Instances, Instance{Group: group})
		case 2:
			group := collection[0]
			id := collection[1]
			errandBody.Instances = append(errandBody.Instances, Instance{Group: group, ID: id})
		default:
			return 0, fmt.Errorf("invalid errand instances names passed in: %v", errandInstances)
		}
	}

	body, err := json.Marshal(errandBody)
	if err != nil {
		return 0, err
	}

	return c.postAndGetTaskIDCheckingForErrors(
		fmt.Sprintf("%s/deployments/%s/errands/%s/runs", c.url, deploymentName, errandName),
		http.StatusFound,
		body,
		"application/json",
		contextID,
		logger,
	)
}
