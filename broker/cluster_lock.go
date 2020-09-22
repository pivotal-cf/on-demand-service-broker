// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"fmt"
)

// acquireClusterLock acquires the lock for the instance specified by instanceID
func (b *Broker) acquireClusterLock(instanceID string) error {
	b.deploymentLock.Lock()
	defer b.deploymentLock.Unlock()

	if _, ok := b.clusterRegister[instanceID]; ok {
		return fmt.Errorf("od-broker is processing a request for the instance: %s, please try again later", instanceID)
	}

	b.clusterRegister[instanceID] = struct{}{}
	return nil
}

// releaseClusterLock releases the lock for the instance specified by instanceID
func (b *Broker) releaseClusterLock(instanceID string) {
	b.deploymentLock.Lock()
	defer b.deploymentLock.Unlock()

	delete(b.clusterRegister, instanceID)
}
