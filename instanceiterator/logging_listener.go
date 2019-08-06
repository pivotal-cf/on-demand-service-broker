// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package instanceiterator

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

type LoggingListener struct {
	logger *log.Logger
	prefix string
}

func NewLoggingListener(logger *log.Logger, processType string) Listener {
	return LoggingListener{
		logger: logger,
		prefix: processType,
	}
}

func (ll LoggingListener) Starting(maxInFlight int) {
	ll.printf("STARTING OPERATION with %d concurrent workers\n", maxInFlight)
}

func (ll LoggingListener) RetryAttempt(num, limit int) {
	if num > 1 {
		ll.printf("Processing all remaining instances. Attempt %d/%d\n", num, limit)
	} else {
		ll.printf("Processing all instances. Attempt %d/%d\n", num, limit)
	}
}

func (ll LoggingListener) RetryCanariesAttempt(attempt, limit, remainingCanaries int) {
	if attempt > 1 {
		ll.printf("Processing %d remaining canaries. Attempt %d/%d\n", remainingCanaries, attempt, limit)
	} else {
		ll.printf("Processing all canaries. Attempt %d/%d\n", attempt, limit)
	}
}

func (ll LoggingListener) InstancesToProcess(instances []service.Instance) {
	msg := "Service Instances:"
	for _, instance := range instances {
		msg = fmt.Sprintf("%s %s", msg, instance.GUID)
	}
	ll.println(msg)
	ll.printf("Total Service Instances found: %d\n", len(instances))
}

func (ll LoggingListener) InstanceOperationStarting(instance string, index, totalInstances int, isCanary bool) {
	var instanceCount string
	if !isCanary {
		instanceCount = fmt.Sprintf(" %d of %d", index, totalInstances)
	}
	ll.printf("[%s] Starting to process service instance%s", instance, instanceCount)
}

func (ll LoggingListener) InstanceOperationStartResult(instance string, status OperationState) {
	var message string

	switch status {
	case OperationAccepted:
		message = "operation accepted"
	case InstanceNotFound:
		message = "already deleted from platform"
	case OrphanDeployment:
		message = "orphan service instance detected - no corresponding bosh deployment"
	case OperationInProgress:
		message = "operation in progress"
	case OperationSkipped:
		message = "instance already up to date - operation skipped"
	default:
		message = "unexpected result"
	}

	ll.printf("[%s] Result: %s", instance, message)
}

func (ll LoggingListener) InstanceOperationFinished(instance string, result string) {
	ll.printf("[%s] Result: Service Instance operation %s\n", instance, result)
}

func (ll LoggingListener) WaitingFor(instance string, boshTaskId int) {
	ll.printf("[%s] Waiting for operation to complete: bosh task id %d", instance, boshTaskId)
}

func (ll LoggingListener) Progress(pollingInterval time.Duration, orphanCount, processedCount, skippedCount, toRetryCount, deletedCount int) {
	ll.printf("Progress summary: "+
		"Sleep interval until next attempt: %s; "+
		"Number of successful operations so far: %d; "+
		"Number of skipped operations so far: %d; "+
		"Number of service instance orphans detected so far: %d; "+
		"Number of deleted instances before operation could happen: %d; "+
		"Number of operations in progress (to retry) so far: %d",
		pollingInterval,
		processedCount,
		skippedCount,
		orphanCount,
		deletedCount,
		toRetryCount,
	)
}

func (ll LoggingListener) Finished(orphanCount, finishedCount, skippedCount, deletedCount int, busyInstances, failedInstances []string) {
	var failedList string
	var busyList string
	if len(failedInstances) > 0 {
		failedList = fmt.Sprintf(" [%s]", strings.Join(failedInstances, ", "))
	}
	if len(busyInstances) > 0 {
		busyList = fmt.Sprintf(" [%s]", strings.Join(busyInstances, ", "))
	}

	status := "SUCCESS"
	if len(failedInstances) > 0 || len(busyInstances) > 0 {
		status = "FAILED"
	}

	ll.printf("FINISHED PROCESSING Status: %s; Summary: "+
		"Number of successful operations: %d; "+
		"Number of skipped operations: %d; "+
		"Number of service instance orphans detected: %d; "+
		"Number of deleted instances before operation could happen: %d; "+
		"Number of busy instances which could not be processed: %d%s; "+
		"Number of service instances that failed to process: %d%s",
		status,
		finishedCount,
		skippedCount,
		orphanCount,
		deletedCount,
		len(busyInstances),
		busyList,
		len(failedInstances),
		failedList,
	)
}

func (ll LoggingListener) CanariesStarting(canaries int, filter config.CanarySelectionParams) {
	msg := fmt.Sprintf("STARTING CANARIES: %d canaries", canaries)
	if len(filter) > 0 {
		msg = fmt.Sprintf("%s with selection criteria: %s", msg, filter)
	}
	ll.println(msg)
}

func (ll LoggingListener) CanariesFinished() {
	ll.printf("FINISHED CANARIES")
}

func (ll LoggingListener) FailedToRefreshInstanceInfo(instance string) {
	ll.logger.Printf("[%s] Failed to get refreshed list of instances. Continuing with previously fetched info.\n", instance)
}

func (ll LoggingListener) printf(args ...interface{}) {
	mask := fmt.Sprintf("[%s] %s", ll.prefix, args[0])
	ll.logger.Printf(mask, args[1:]...)
}

func (ll LoggingListener) println(msg string) {
	ll.printf(msg + "\n")
}
