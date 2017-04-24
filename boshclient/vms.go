// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshclient

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

func (c *Client) VMs(name string, logger *log.Logger) (bosh.BoshVMs, error) {
	logger.Printf("retrieving VMs for deployment %s from bosh\n", name)
	errs := func(err error) (bosh.BoshVMs, error) {
		return nil, err
	}

	taskID, err := c.getTaskIdFromBoshCheckingForErrors(
		fmt.Sprintf("%s/deployments/%s/vms?format=full", c.boshURL, name),
		http.StatusFound,
		logger,
	)

	if err != nil {
		return errs(err)
	}

	for {
		task, getTaskErr := c.GetTask(taskID, logger)
		if getTaskErr != nil {
			return errs(getTaskErr)
		}

		if task.State == BoshTaskError {
			return nil, fmt.Errorf("task %d failed", taskID)
		}

		if task.State == BoshTaskDone {
			logger.Printf("Task %d finished: %s\n", taskID, task.ToLog())
			break
		}

		time.Sleep(time.Second * c.BoshPollingInterval)
	}

	vmsOutputForEachJob, err := c.VMsOutput(taskID, logger)
	if err != nil {
		return errs(err)
	}

	vms := bosh.BoshVMs{}
	for _, vmsOutput := range vmsOutputForEachJob {
		vms[vmsOutput.InstanceGroup] = append(vms[vmsOutput.InstanceGroup], vmsOutput.IPs...)
	}

	return vms, nil
}

type BoshVMsOutput struct {
	IPs           []string
	InstanceGroup string `json:"job_name"`
}

func (c *Client) VMsOutput(taskID int, logger *log.Logger) ([]BoshVMsOutput, error) {
	outputs := []BoshVMsOutput{}
	var output BoshVMsOutput
	outputReadyCallback := func() {
		outputs = append(outputs, output)
		output = BoshVMsOutput{}
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
