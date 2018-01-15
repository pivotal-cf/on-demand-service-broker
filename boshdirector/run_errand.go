// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"log"

	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/pkg/errors"
)

type Instance struct {
	Group string `json:"group"`
	ID    string `json:"id,omitempty"`
}

func (c *Client) RunErrand(deploymentName, errandName string, errandInstances []string, contextID string, logger *log.Logger) (int, error) {
	logger.Printf("running errand %s on colocated instances %v from deployment %s\n", errandName, errandInstances, deploymentName)

	deployment, err := c.director.FindDeployment(deploymentName)
	if err != nil {
		return -1, errors.Wrapf(err, `Could not find deployment "%s"`, deploymentName)
	}

	var instances []director.InstanceGroupOrInstanceSlug
	for _, errandInstance := range errandInstances {
		instanceGroupOrSlug, err := director.NewInstanceGroupOrInstanceSlugFromString(errandInstance)
		if err != nil {
			return -1, errors.Wrapf(err, "Invalid instance name %s for errand %s", errandName, errandInstance)
		}
		instances = append(instances, instanceGroupOrSlug)
	}

	_, err = deployment.RunErrand(errandName, false, false, instances)
	if err != nil {
		return -1, errors.Wrapf(err, "Could not run errand %s", errandName)
	}

	tasks, err := c.director.RecentTasks(1, director.TasksFilter{Deployment: deploymentName})
	if err != nil {
		return -1, errors.Wrap(err, "Could not fetch task")
	}

	if len(tasks) == 0 {
		return 0, nil
	}
	return tasks[0].ID(), nil
}
