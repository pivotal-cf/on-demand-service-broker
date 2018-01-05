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

	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

type LoggingListener struct {
	logger *log.Logger
}

func NewLoggingListener(logger *log.Logger) Listener {
	return LoggingListener{logger: logger}
}

func (ll LoggingListener) Starting(maxInFlight int) {
	ll.logger.Printf("STARTING UPGRADES with %d concurrent workers\n", maxInFlight)
}

func (ll LoggingListener) RetryAttempt(num, limit int) {
	var msg string
	if num == 1 {
		msg = "Upgrading all instances"
	} else {
		msg = "Upgrading all remaining instances"
	}
	ll.logger.Printf("%s. Attempt %d/%d\n", msg, num, limit)
}

func (ll LoggingListener) InstancesToUpgrade(instances []service.Instance) {
	msg := "Service Instances:"
	for _, instance := range instances {
		msg = fmt.Sprintf("%s %s", msg, instance.GUID)
	}
	ll.logger.Println(msg)
	ll.logger.Printf("Total Service Instances found in Cloud Foundry: %d\n", len(instances))
}

func (ll LoggingListener) InstanceUpgradeStarting(instance string, index int32, totalInstances int) {
	ll.logger.Printf("[%s] Starting to upgrade service instance %d of %d", instance, index+1, totalInstances)
}

func (ll LoggingListener) InstanceUpgradeStartResult(instance string, resultType services.UpgradeOperationType) {
	var message string

	switch resultType {
	case services.UpgradeAccepted:
		message = "accepted upgrade"
	case services.InstanceNotFound:
		message = "already deleted in CF"
	case services.OrphanDeployment:
		message = "orphan CF service instance detected - no corresponding bosh deployment"
	case services.OperationInProgress:
		message = "operation in progress"
	default:
		message = "unexpected result"
	}

	ll.logger.Printf("[%s] Result: %s", instance, message)
}

func (ll LoggingListener) InstanceUpgraded(instance string, result string) {
	ll.logger.Printf("[%s] Result: Service Instance upgrade %s\n", instance, result)
}

func (ll LoggingListener) WaitingFor(instance string, boshTaskId int) {
	ll.logger.Printf("[%s] Waiting for upgrade to complete: bosh task id %d", instance, boshTaskId)
}

func (ll LoggingListener) Progress(pollingInterval time.Duration, orphanCount, upgradedCount int32, toRetryCount int, deletedCount int32) {
	ll.logger.Printf("Upgrade progress summary:\n"+
		"\tSleep interval until next attempt: %s\n"+
		"\tNumber of successful upgrades so far: %d\n"+
		"\tNumber of CF service instance orphans detected so far: %d\n"+
		"\tNumber of deleted instances before upgrade could occur: %d \n"+
		"\tNumber of operations in progress (to retry) so far: %d \n",
		pollingInterval,
		upgradedCount,
		orphanCount,
		deletedCount,
		toRetryCount,
	)
}

func (ll LoggingListener) Finished(orphanCount, upgradedCount, deletedCount int32, couldNotStartCount int) {
	ll.logger.Printf("FINISHED UPGRADES Summary:\n"+
		"\tNumber of successful upgrades: %d\n"+
		"\tNumber of CF service instance orphans detected: %d\n"+
		"\tNumber of deleted instances before upgrade could occur: %d\n"+
		"\tNumber of busy instances which could not be upgraded: %d\n",
		upgradedCount,
		orphanCount,
		deletedCount,
		couldNotStartCount,
	)
}
