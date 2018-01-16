// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"fmt"
	"log"

	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/pkg/errors"
)

func (c *Client) DeleteDeployment(name, contextID string, logger *log.Logger) (int, error) {
	logger.Printf("deleting deployment %s\n", name)
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return 0, errors.Wrap(err, "Failed to build director")
	}
	deployment, err := d.FindDeployment(name)
	if err != nil {
		return 0, errors.Wrap(err, fmt.Sprintf(`BOSH error when deleting deployment "%s"`, name))
	}
	err = deployment.Delete(false)
	if err != nil {
		return 0, errors.Wrap(err, fmt.Sprintf("Could not delete deployment %s", name))
	}
	tasks, err := d.RecentTasks(1, director.TasksFilter{Deployment: name})
	if err != nil {
		return 0, errors.Wrap(err, fmt.Sprintf(`Could not find tasks for deployment "%s"`, name))
	}

	if len(tasks) == 0 {
		return 0, nil
	}
	return tasks[0].ID(), nil
}
