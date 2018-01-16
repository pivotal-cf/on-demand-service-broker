// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"log"
	"math"

	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/pkg/errors"
)

func (c *Client) GetTasks(deploymentName string, logger *log.Logger) (BoshTasks, error) {
	logger.Printf("getting tasks for deployment %s from bosh\n", deploymentName)
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return BoshTasks{}, errors.Wrap(err, "Failed to build director")
	}

	tasks, err := d.RecentTasks(math.MaxInt32, director.TasksFilter{
		All: false, Deployment: deploymentName,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "Could not fetch recent tasks for deployment %s", deploymentName)
	}
	return c.toBoshTasks(tasks, deploymentName, logger)

}

func (c *Client) GetNormalisedTasksByContext(deploymentName, contextID string, logger *log.Logger) (BoshTasks, error) {
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build director")
	}
	tasks, err := d.FindTasksByContextId(contextID)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not fetch tasks for deployment %s with context id %s", deploymentName, contextID)
	}
	return c.toBoshTasks(tasks, deploymentName, logger)
}

func (c *Client) toBoshTasks(tasks []director.Task, deploymentName string, logger *log.Logger) (BoshTasks, error) {
	var boshTasks BoshTasks
	for _, task := range tasks {
		if task.DeploymentName() == deploymentName {
			taskState, err := c.taskState(task, logger)
			if err != nil {
				return nil, errors.Wrap(err, "Could not retrieve task output")
			}
			boshTasks = append(boshTasks, BoshTask{
				ID:          task.ID(),
				State:       taskState,
				Description: task.Description(),
				Result:      task.Result(),
				ContextID:   task.ContextID(),
			})
		}
	}
	return boshTasks, nil
}

// bosh status for failed errands is 'done', not 'error'
// https://github.com/cloudfoundry/bosh/issues/1592
func (c *Client) taskState(task director.Task, logger *log.Logger) (string, error) {
	if task.State() == TaskDone {
		taskOutput, err := c.GetTaskOutput(task.ID(), logger)
		if err != nil {
			return "", err
		}
		if taskOutput.ExitCode != 0 {
			return TaskError, nil
		}
	}
	return task.State(), nil
}
