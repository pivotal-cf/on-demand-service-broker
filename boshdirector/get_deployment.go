// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"log"

	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/pkg/errors"
)

func (c *Client) GetDeployment(name string, logger *log.Logger) ([]byte, bool, error) {
	logger.Printf("getting manifest from bosh for deployment %s", name)
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return nil, false, errors.Wrap(err, "Failed to build director")
	}

	deployments, err := d.ListDeployments()
	if err != nil {
		return nil, false, errors.Wrap(err, "Cannot get the list of deployments")
	}

	for _, deployment := range deployments {
		if deployment.Name == name {
			dep, err := d.FindDeployment(name)
			if err != nil {
				return nil, false, errors.Wrapf(err, "Cannot create deployment object for deployment %q", name)
			}

			rawManifest, err := dep.Manifest()
			if err != nil {
				return nil, false, errors.Wrapf(err, "Cannot obtain manifest for deployment %q", name)
			}

			return []byte(rawManifest), true, nil
		}
	}
	return nil, false, nil
}
