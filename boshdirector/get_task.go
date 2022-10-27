// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"log"

	"github.com/cloudfoundry/bosh-cli/v7/director"
	"github.com/pkg/errors"
)

func (c *Client) GetTask(taskID int, logger *log.Logger) (BoshTask, error) {
	logger.Printf("getting task %d from bosh\n", taskID)
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return BoshTask{}, errors.Wrap(err, "Failed to build director")
	}
	task, err := d.FindTask(taskID)
	if err != nil {
		return BoshTask{}, errors.Wrapf(err, "Cannot find task with ID: %d", taskID)
	}
	return BoshTask{
		ID:          task.ID(),
		State:       task.State(),
		Description: task.Description(),
		Result:      task.Result(),
		ContextID:   task.ContextID(),
	}, nil
}

type BoshTaskOutput struct {
	ExitCode int    `json:"exit_code"`
	StdOut   string `json:"stdout"`
	StdErr   string `json:"stderr"`
}

func (c *Client) GetTaskOutput(taskID int, logger *log.Logger) (BoshTaskOutput, error) {
	logger.Printf("getting task output for task %d from bosh\n", taskID)
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return BoshTaskOutput{}, errors.Wrap(err, "Failed to build director")
	}
	task, err := d.FindTask(taskID)
	if err != nil {
		return BoshTaskOutput{}, errors.Wrapf(err, "Could not fetch task with id %d", taskID)
	}

	reporter := &BoshTaskOutputReporter{Logger: logger}
	err = task.ResultOutput(reporter)

	return reporter.Output, errors.Wrap(err, "Could not fetch task output")

}
