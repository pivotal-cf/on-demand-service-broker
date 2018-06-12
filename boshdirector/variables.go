// Copyright (C) 2018-Present Pivotal Software, Inc. All rights reserved.
//
// This program and the accompanying materials are made available under the
// terms of the under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
//
// You may obtain a copy of the License at http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package boshdirector

import (
	"fmt"
	"log"

	"github.com/cloudfoundry/bosh-cli/director"
)

func (c *Client) Variables(deploymentName string, logger *log.Logger) ([]Variable, error) {
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return nil, fmt.Errorf("failed to build director: %s", err)
	}
	deployment, err := d.FindDeployment(deploymentName)
	if err != nil {
		return nil, fmt.Errorf("can't find deployment with name '%s': %s", deploymentName, err)
	}
	boshVars, err := deployment.Variables()
	if err != nil {
		return nil, fmt.Errorf("can't retrieve variables for deployment '%s': %s", deploymentName, err)
	}
	var variables []Variable
	for _, variable := range boshVars {
		variables = append(variables, Variable{Path: variable.Name, ID: variable.ID})
	}
	return variables, nil
}
