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

func (c *Client) GetTask(taskID int, logger *log.Logger) (BoshTask, error) {
	logger.Printf("getting task %d from bosh\n", taskID)
	var getTaskResponse BoshTask

	if err := c.getDataFromBoshCheckingForErrors(
		fmt.Sprintf("%s/tasks/%d", c.boshURL, taskID),
		http.StatusOK,
		&getTaskResponse,
		logger,
	); err != nil {
		return BoshTask{}, err
	}

	return getTaskResponse, nil
}

type BoshTaskOutput struct {
	ExitCode int    `json:"exit_code"`
	StdOut   string `json:"stdout"`
	StdErr   string `json:"stderr"`
}

func (c *Client) GetTaskOutput(taskID int, logger *log.Logger) ([]BoshTaskOutput, error) {
	logger.Printf("getting task output for task %d from bosh\n", taskID)
	outputs := []BoshTaskOutput{}
	var output BoshTaskOutput
	outputReadyCallback := func() {
		outputs = append(outputs, output)
		output = BoshTaskOutput{}
		// `output` is reused for JSON decoding, so use a fresh struct;
		// else you will override your previous values with the current one
	}

	err := c.getMultipleDataFromBoshCheckingForErrors(
		fmt.Sprintf("%s/tasks/%d/output?type=result", c.boshURL, taskID),
		http.StatusOK,
		&output,
		outputReadyCallback,
		logger,
	)

	return outputs, err
}
