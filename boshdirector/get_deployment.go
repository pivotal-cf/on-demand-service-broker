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

func (c *Client) GetDeployment(name string, logger *log.Logger) ([]byte, bool, error) {
	logger.Printf("getting manifest from bosh for deployment %s", name)
	respJSON := make(map[string]string)

	err := c.getDataCheckingForErrors(
		fmt.Sprintf("%s/deployments/%s", c.url, name),
		http.StatusOK,
		&respJSON,
		logger,
	)

	if err != nil {
		if deploymentDoesNotExistYet(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return []byte(respJSON["manifest"]), true, nil
}

func deploymentDoesNotExistYet(err error) bool {
	e, ok := err.(unexpectedStatusError)
	return ok && e.actualStatus == http.StatusNotFound
}
