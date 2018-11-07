// Copyright (C) 2018-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"fmt"
	"log"

	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/pkg/errors"
)

func (c *Client) UpdateConfig(configType, configName string, configContent []byte, logger *log.Logger) error {
	logger.Printf("updating %s config %s\n", configType, configName)
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return errors.Wrap(err, "Failed to build director")
	}
	if _, err := d.UpdateConfig(configType, configName, configContent); err != nil {
		return errors.Wrap(err, fmt.Sprintf(`BOSH error updating "%s" config "%s"`, configType, configName))
	}

	return nil
}
