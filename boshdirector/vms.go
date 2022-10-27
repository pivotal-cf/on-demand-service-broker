// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"log"

	"github.com/cloudfoundry/bosh-cli/v7/director"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pkg/errors"
)

func (c *Client) VMs(deploymentName string, logger *log.Logger) (bosh.BoshVMs, error) {
	logger.Printf("retrieving VMs for deployment %s from bosh\n", deploymentName)
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return bosh.BoshVMs{}, errors.Wrap(err, "Failed to build director")
	}

	deployment, err := d.FindDeployment(deploymentName)
	if err != nil {
		return nil, errors.Wrapf(err, `Could not find deployment "%s"`, deploymentName)
	}

	vmsInfo, err := deployment.VMInfos()
	if err != nil {
		return nil, errors.Wrapf(err, `Could not fetch VMs info for deployment "%s"`, deploymentName)
	}
	boshVms := bosh.BoshVMs{}
	for _, vmInfo := range vmsInfo {
		boshVms[vmInfo.JobName] = append(boshVms[vmInfo.JobName], vmInfo.IPs...)
	}
	return boshVms, nil
}
