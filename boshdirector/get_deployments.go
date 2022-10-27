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

func (c *Client) GetDeployments(logger *log.Logger) ([]Deployment, error) {
	logger.Println("getting deployments from bosh")
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build director")
	}
	rawDeployments, err := d.Deployments()
	if err != nil {
		return nil, errors.Wrap(err, "Cannot get the list of deployments")
	}
	deployments := make([]Deployment, len(rawDeployments))
	for i, d := range rawDeployments {
		deployments[i] = Deployment{Name: d.Name()}
	}
	return deployments, nil
}
