// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"fmt"
	"log"
	"net/http"
)

func (c *Client) GetTasks(deploymentName string, logger *log.Logger) (BoshTasks, error) {
	logger.Printf("getting tasks for deployment %s from bosh\n", deploymentName)
	return c.getTasks(deploymentName, "", logger)
}

func (c *Client) GetNormalisedTasksByContext(deploymentName, contextID string, logger *log.Logger) (BoshTasks, error) {
	logger.Printf("getting tasks for deployment %s with context %s from bosh\n", deploymentName, contextID)
	tasks, err := c.getTasks(deploymentName, contextID, logger)
	if err != nil {
		return BoshTasks{}, err
	}

	return c.resolveErrandState(tasks, logger)
}

// bosh status for failed errands is 'done', not 'error'
// https://github.com/cloudfoundry/bosh/issues/1592
func (c *Client) resolveErrandState(tasks BoshTasks, logger *log.Logger) (BoshTasks, error) {
	for i, task := range tasks {
		if task.State == BoshTaskDone {
			taskOutputs, err := c.GetTaskOutput(task.ID, logger)
			if err != nil {
				return nil, err
			}
			if len(taskOutputs) > 0 && taskOutputs[0].ExitCode != 0 {
				tasks[i].State = BoshTaskError
			}
		}
	}

	return tasks, nil
}

func (c *Client) getTasks(deploymentName string, contextID string, logger *log.Logger) (BoshTasks, error) {
	url := fmt.Sprintf("%s/tasks?deployment=%s", c.boshURL, deploymentName)

	if contextID != "" {
		url = url + fmt.Sprintf("&context_id=%s", contextID)
	}

	var tasks BoshTasks
	if err := c.getDataFromBoshCheckingForErrors(
		fmt.Sprintf(url),
		http.StatusOK,
		&tasks,
		logger,
	); err != nil {
		return nil, err
	}

	return tasks, nil
}
