// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"log"
	"strings"
)

func (b *Broker) OrphanDeployments(logger *log.Logger) ([]string, error) {
	rawInstances, err := b.Instances(logger)
	if err != nil {
		logger.Printf("error listing instances: %s", err)
		return nil, b.processError(err, logger)
	}

	instanceIDs := map[string]bool{}
	for _, instance := range rawInstances {
		instanceIDs[instance.GUID] = true
	}

	deployments, err := b.boshClient.GetDeployments(logger)
	if err != nil {
		logger.Printf("error getting deployments: %s", err)
		return nil, b.processError(err, logger)
	}

	var orphanDeploymentNames []string
	for _, deployment := range deployments {
		if !strings.HasPrefix(deployment.Name, InstancePrefix) {
			continue
		}

		if !instanceIDs[instanceID(deployment.Name)] {
			orphanDeploymentNames = append(orphanDeploymentNames, deployment.Name)
		}
	}

	return orphanDeploymentNames, nil
}
