// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/pkg/errors"
	"log"
)

func (c *Client) GetTasksInProgress(deploymentName string, logger *log.Logger) (BoshTasks, error) {
	logger.Printf("getting current tasks for deployment %s from bosh\n", deploymentName)
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return BoshTasks{}, errors.Wrap(err, "Failed to build director")
	}

	currentTasks, err := d.CurrentTasks(
		director.TasksFilter{
			All: false, Deployment: deploymentName,
		})
	if err != nil {
		return BoshTasks{}, errors.Wrapf(err, "Could not fetch current tasks for deployment %s", deploymentName)
	}

	var boshTasks BoshTasks
	for _, task := range currentTasks {
		boshTasks = append(boshTasks, BoshTask{
			ID:          task.ID(),
			State:       task.State(),
			Description: task.Description(),
			Result:      task.Result(),
			ContextID:   task.ContextID(),
		})
	}
	return boshTasks, nil
}

func (c *Client) GetNormalisedTasksByContext(deploymentName, contextID string, logger *log.Logger) (BoshTasks, error) {
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build director")
	}
	tasks, err := d.FindTasksByContextId(contextID)
	if err != nil {
		return BoshTasks{}, errors.Wrapf(err, "Could not fetch tasks for deployment %s with context id %s", deploymentName, contextID)
	}

	var boshTasks BoshTasks
	for _, task := range tasks {
		if task.DeploymentName() == deploymentName {
			taskState, err := c.fetchTaskState(task, logger)
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
func (c *Client) fetchTaskState(task director.Task, logger *log.Logger) (string, error) {
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
