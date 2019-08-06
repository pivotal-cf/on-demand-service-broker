// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package instanceiterator_test

import (
	"io"
	"log"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/instanceiterator"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/service"
)

var _ = Describe("Logging Listener", func() {
	var (
		logPrefix   = "some-process"
		processType = "some-process"
	)

	It("Logs a refresh service instance info error", func() {
		Expect(logResultsFrom(processType, func(listener instanceiterator.Listener) { listener.FailedToRefreshInstanceInfo("GUID") })).
			To(Say(`\[GUID\] Failed to get refreshed list of instances. Continuing with previously fetched info.`))
	})

	It("Shows starting message", func() {
		Expect(logResultsFromAsString(processType, func(listener instanceiterator.Listener) { listener.Starting(2) })).
			To(ContainSubstring("[%s] STARTING OPERATION with 2 concurrent workers", logPrefix))
	})

	It("Shows starting canaries message", func() {
		Expect(logResultsFromAsString(processType, func(listener instanceiterator.Listener) {
			listener.CanariesStarting(2, config.CanarySelectionParams{})
		})).
			To(ContainSubstring("[%s] STARTING CANARIES: 2 canaries", logPrefix))
	})

	It("Shows starting canaries message with filter params", func() {
		filter := map[string]string{"org": "my-org", "space": "my-space"}
		Expect(logResultsFromAsString(processType, func(listener instanceiterator.Listener) { listener.CanariesStarting(2, filter) })).
			To(ContainSubstring("[%s] STARTING CANARIES: 2 canaries with selection criteria: ", logPrefix))
		Expect(logResultsFrom(processType, func(listener instanceiterator.Listener) { listener.CanariesStarting(2, filter) })).
			To(Say("org: my-org"))
		Expect(logResultsFrom(processType, func(listener instanceiterator.Listener) { listener.CanariesStarting(2, filter) })).
			To(Say("space: my-space"))
	})

	It("Shows canaries finished message", func() {
		Expect(logResultsFromAsString(processType, func(listener instanceiterator.Listener) { listener.CanariesFinished() })).
			To(ContainSubstring("[%s] FINISHED CANARIES", logPrefix))
	})

	It("Shows attempt x of y", func() {
		Expect(logResultsFrom(processType, retryAttempt(2, 5))).
			To(Say("Attempt 2/5"))
	})

	It("Shows processing all instances during first attempt", func() {
		Expect(logResultsFromAsString(processType, retryAttempt(1, 5))).
			To(ContainSubstring("[%s] Processing all instances", logPrefix))
	})

	It("Shows processing all remaining instances during later attempts", func() {
		Expect(logResultsFromAsString(processType, retryAttempt(3, 5))).
			To(ContainSubstring("[%s] Processing all remaining instances", logPrefix))
	})

	It("Shows processing all canaries during first attempt", func() {
		Expect(logResultsFromAsString(processType, retryCanariesAttempt(1, 5, 3))).
			To(ContainSubstring("[%s] Processing all canaries", logPrefix))
	})

	It("Shows processing all remaining canaries during later attempts", func() {
		Expect(logResultsFromAsString(processType, retryCanariesAttempt(3, 5, 2))).
			To(ContainSubstring("[%s] Processing 2 remaining canaries", logPrefix))
	})

	It("Shows which instances to process", func() {
		Expect(logResultsFromAsString(processType, func(listener instanceiterator.Listener) {
			listener.InstancesToProcess([]service.Instance{{GUID: "one"}, {GUID: "two"}})
		})).To(SatisfyAll(
			ContainSubstring("[%s] Service Instances: one two", logPrefix),
			ContainSubstring("[%s] Total Service Instances found: 2", logPrefix),
		))
	})

	It("Shows which instance has started processing", func() {
		buffer := logResultsFromAsString(processType, func(listener instanceiterator.Listener) {
			listener.InstanceOperationStarting("service-instance", 2, 5, false)
		})

		Expect(buffer).To(ContainSubstring("[%s] [service-instance] Starting to process service instance 2 of 5", logPrefix))
	})

	It("Suppress instance number if it's a canary", func() {
		buffer := logResultsFromAsString(processType, func(listener instanceiterator.Listener) {
			listener.InstanceOperationStarting("service-instance", 2, 5, true)
		})

		Expect(buffer).To(ContainSubstring("[%s] [service-instance] Starting to process service instance\n", logPrefix))
	})

	Describe("InstanceOperationStartResult()", func() {
		var (
			result       instanceiterator.OperationState
			loggedString string
		)

		JustBeforeEach(func() {
			loggedString = logResultsFromAsString(processType, func(listener instanceiterator.Listener) {
				listener.InstanceOperationStartResult("service-instance", result)
			})
		})

		Context("when accepted", func() {
			BeforeEach(func() {
				result = instanceiterator.OperationAccepted
			})

			It("Shows accepted operation", func() {
				Expect(loggedString).To(ContainSubstring("[%s] [service-instance] Result: operation accepted", logPrefix))
			})
		})

		Context("when not found", func() {
			BeforeEach(func() {
				result = instanceiterator.InstanceNotFound
			})

			It("shows already deleted from platform", func() {
				Expect(loggedString).To(ContainSubstring("[%s] [service-instance] Result: already deleted from platform", logPrefix))
			})
		})

		Context("when orphaned", func() {
			BeforeEach(func() {
				result = instanceiterator.OrphanDeployment
			})

			It("shows already deleted from platform", func() {
				Expect(loggedString).To(ContainSubstring("[%s] [service-instance] Result: orphan service instance detected - no corresponding bosh deployment", logPrefix))
			})
		})

		Context("when conflict", func() {
			BeforeEach(func() {
				result = instanceiterator.OperationInProgress
			})

			It("shows already deleted from platform", func() {
				Expect(loggedString).To(ContainSubstring("[%s] [service-instance] Result: operation in progress", logPrefix))
			})
		})

		Context("when skipped", func() {
			BeforeEach(func() {
				result = instanceiterator.OperationSkipped
			})

			It("shows already skipped from platform", func() {
				Expect(loggedString).To(ContainSubstring("[%s] [service-instance] Result: instance already up to date - operation skipped", logPrefix))
			})
		})

		Context("when error", func() {
			BeforeEach(func() {
				result = instanceiterator.OperationState(-1)
			})

			It("shows already deleted from platform", func() {
				Expect(loggedString).To(ContainSubstring("[%s] [service-instance] Result: unexpected result", logPrefix))
			})
		})
	})

	It("Shows which instance is still in progress", func() {
		Expect(logResultsFromAsString(processType, func(listener instanceiterator.Listener) { listener.WaitingFor("one", 999) })).
			To(ContainSubstring("[%s] [one] Waiting for operation to complete: bosh task id 999", logPrefix))
	})

	It("Shows which instance has been processed", func() {
		Expect(logResultsFromAsString(processType, func(listener instanceiterator.Listener) { listener.InstanceOperationFinished("one", "success") })).
			To(ContainSubstring("[%s] [one] Result: Service Instance operation success", logPrefix))
	})

	It("Shows a summary of the progress so far", func() {
		result := logResultsFromAsString(processType, func(listener instanceiterator.Listener) {
			listener.Progress(time.Duration(10)*time.Second, 234, 345, 152, 456, 567)
		})

		Expect(result).To(SatisfyAll(
			ContainSubstring("[%s] Progress summary:", logPrefix),
			ContainSubstring("Sleep interval until next attempt: 10s"),
			ContainSubstring("Sleep interval until next attempt: 10s"),
			ContainSubstring("Number of successful operations so far: 345"),
			ContainSubstring("Number of skipped operations so far: 152"),
			ContainSubstring("Number of service instance orphans detected so far: 234"),
			ContainSubstring("Number of deleted instances before operation could happen: 567"),
			ContainSubstring("Number of operations in progress (to retry) so far: 456"),
		))
	})

	It("Shows a final summary where we completed successfully", func() {
		result := logResultsFromAsString(processType, func(listener instanceiterator.Listener) {
			listener.Finished(23, 34, 15, 45, nil, nil)
		})

		Expect(result).To(SatisfyAll(
			ContainSubstring("[%s] FINISHED PROCESSING Status: SUCCESS; Summary", logPrefix),
			ContainSubstring("Number of successful operations: 34"),
			ContainSubstring("Number of skipped operations: 15"),
			ContainSubstring("Number of service instance orphans detected: 23"),
			ContainSubstring("Number of deleted instances before operation could happen: 45"),
			ContainSubstring("Number of busy instances which could not be processed: 0"),
			ContainSubstring("Number of service instances that failed to process: 0"),
			Not(ContainSubstring("[]")),
		))
	})

	It("Shows a final summary where instances could not start", func() {
		result := logResultsFromAsString(processType, func(listener instanceiterator.Listener) {
			busyList := make([]string, 56)
			listener.Finished(23, 34, 0, 45, busyList, nil)
		})

		Expect(result).To(SatisfyAll(
			ContainSubstring("[%s] FINISHED PROCESSING Status: FAILED; Summary", logPrefix),
			ContainSubstring("Number of successful operations: 34"),
			ContainSubstring("Number of service instance orphans detected: 23"),
			ContainSubstring("Number of deleted instances before operation could happen: 45"),
			ContainSubstring("Number of busy instances which could not be processed: 56"),
			ContainSubstring("Number of service instances that failed to process: 0"),
			Not(ContainSubstring("[]")),
		))
	})

	It("Shows a final summary where a single service instance failed to process", func() {
		result := logResultsFromAsString(processType, func(listener instanceiterator.Listener) {
			listener.Finished(23, 34, 0, 45, []string{"foo"}, []string{"2f9752c3-887b-4ccb-8693-7c15811ffbdd"})
		})

		Expect(result).To(SatisfyAll(
			ContainSubstring("[%s] FINISHED PROCESSING Status: FAILED; Summary", logPrefix),
			ContainSubstring("Number of successful operations: 34"),
			ContainSubstring("Number of service instance orphans detected: 23"),
			ContainSubstring("Number of deleted instances before operation could happen: 45"),
			ContainSubstring("Number of busy instances which could not be processed: 1 [foo]"),
			ContainSubstring("Number of service instances that failed to process: 1 [2f9752c3-887b-4ccb-8693-7c15811ffbdd]"),
		))
	})

	It("Shows a final summary where multiple services instances failed the operation", func() {
		result := logResultsFromAsString(processType, func(listener instanceiterator.Listener) {
			listener.Finished(23, 34, 12, 45, make([]string, 56), []string{"2f9752c3-887b-4ccb-8693-7c15811ffbdd", "7a2c7adb-1d47-4355-af39-41c5a2892b92"})
		})

		Expect(result).To(SatisfyAll(
			ContainSubstring("[%s] FINISHED PROCESSING Status: FAILED; Summary", logPrefix),
			ContainSubstring("Number of successful operations: 34"),
			ContainSubstring("Number of skipped operations: 12"),
			ContainSubstring("Number of service instance orphans detected: 23"),
			ContainSubstring("Number of deleted instances before operation could happen: 45"),
			ContainSubstring("Number of busy instances which could not be processed: 56"),
			ContainSubstring("Number of service instances that failed to process: 2 [2f9752c3-887b-4ccb-8693-7c15811ffbdd, 7a2c7adb-1d47-4355-af39-41c5a2892b92]"),
		))
	})
})

func logResultsFrom(processType string, action func(listener instanceiterator.Listener)) *Buffer {
	logBuffer := NewBuffer()
	loggerFactory := loggerfactory.New(io.MultiWriter(GinkgoWriter, logBuffer), "logging-listener-tests", log.LstdFlags)
	listener := instanceiterator.NewLoggingListener(loggerFactory.New(), processType)

	action(listener)

	return logBuffer
}

func logResultsFromAsString(processType string, action func(listener instanceiterator.Listener)) string {
	logBuffer := NewBuffer()
	loggerFactory := loggerfactory.New(io.MultiWriter(GinkgoWriter, logBuffer), "logging-listener-tests", log.LstdFlags)
	listener := instanceiterator.NewLoggingListener(loggerFactory.New(), processType)

	action(listener)

	return string(logBuffer.Contents())
}

func retryAttempt(num, limit int) func(listener instanceiterator.Listener) {
	return func(listener instanceiterator.Listener) {
		listener.RetryAttempt(num, limit)
	}
}

func retryCanariesAttempt(num, limit, n int) func(listener instanceiterator.Listener) {
	return func(listener instanceiterator.Listener) {
		listener.RetryCanariesAttempt(num, limit, n)
	}
}
