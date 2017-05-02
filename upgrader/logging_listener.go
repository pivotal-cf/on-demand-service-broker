// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrader

import (
	"fmt"
	"log"
	"time"

	"github.com/pivotal-cf/on-demand-service-broker/upgrader/broker_response"
)

type LoggingListener struct {
	logger *log.Logger
}

func NewLoggingListener(logger *log.Logger) Listener {
	return LoggingListener{logger: logger}
}

func (ll LoggingListener) Starting() {
	ll.logger.Println("STARTING UPGRADES")
}

func (ll LoggingListener) InstancesToUpgrade(instances []string) {
	msg := "Service Instances:"
	for _, instance := range instances {
		msg = fmt.Sprintf("%s %s", msg, instance)
	}
	ll.logger.Println(msg)
	ll.logger.Printf("Total Service Instances found in Cloud Foundry: %d\n", len(instances))
}

func (ll LoggingListener) InstanceUpgradeStarting(instance string, index, totalInstances int) {
	ll.logger.Printf("Service instance: %s, upgrade attempt starting (%d of %d)", instance, index+1, totalInstances)
}

func (ll LoggingListener) InstanceUpgradeStartResult(resultType broker_response.UpgradeOperationType) {
	var message string

	switch resultType {
	case broker_response.ResultAccepted:
		message = "accepted upgrade"
	case broker_response.ResultNotFound:
		message = "already deleted in CF"
	case broker_response.ResultOrphan:
		message = "orphan CF service instance detected - no corresponding bosh deployment"
	case broker_response.ResultOperationInProgress:
		message = "operation in progress"
	default:
		message = "unexpected result"
	}

	ll.logger.Printf("Result: %s", message)
}

func (ll LoggingListener) InstanceUpgraded(instance string, result string) {
	ll.logger.Printf("Result: Service Instance %s upgrade %s\n", instance, result)
}

func (ll LoggingListener) WaitingFor(instance string, boshTaskId int) {
	ll.logger.Printf("Waiting for upgrade to complete for %s: bosh task id %d", instance, boshTaskId)
}

func (ll LoggingListener) Progress(pollingInterval time.Duration, orphanCount, upgradedCount, toRetryCount, deletedCount int) {
	ll.logger.Printf("Upgrade progress summary: "+
		"Sleep interval until next attempt: %s; "+
		"Number of successful upgrades so far: %d; "+
		"Number of CF service instance orphans detected so far: %d; "+
		"Number of deleted instances before upgrade could occur: %d; "+
		"Number of operations in progress (to retry) so far: %d",
		pollingInterval,
		upgradedCount,
		orphanCount,
		deletedCount,
		toRetryCount,
	)
}

func (ll LoggingListener) Finished(orphanCount, upgradedCount, deletedCount int) {
	ll.logger.Printf("FINISHED UPGRADES Summary: "+
		"Number of successful upgrades: %d; "+
		"Number of CF service instance orphans detected: %d; "+
		"Number of deleted instances before upgrade could occur: %d",
		upgradedCount,
		orphanCount,
		deletedCount,
	)
}
