// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrader_test

import (
	"io"
	"log"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader"
)

const (
	ten_seconds = time.Duration(10) * time.Second
)

var _ = Describe("Logging Listener", func() {
	It("Shows starting message", func() {
		Expect(logResultsFrom(func(listener upgrader.Listener) { listener.Starting(2) })).
			To(Say("STARTING UPGRADES with 2 concurrent workers"))
	})

	It("Shows starting canaries message", func() {
		Expect(logResultsFrom(func(listener upgrader.Listener) { listener.CanariesStarting(2) })).
			To(Say("STARTING CANARY UPGRADES: 2 canaries"))
	})

	It("Shows canaries finished message", func() {
		Expect(logResultsFrom(func(listener upgrader.Listener) { listener.CanariesFinished() })).
			To(Say("FINISHED CANARY UPGRADES"))
	})

	It("Shows attempt x of y", func() {
		Expect(logResultsFrom(retryAttempt(2, 5))).
			To(Say("Attempt 2/5"))
	})

	It("Shows upgrading all instances during first attempt", func() {
		Expect(logResultsFrom(retryAttempt(1, 5))).
			To(Say("Upgrading all instances"))
	})

	It("Shows upgrading all remaining instances during later attempts", func() {
		Expect(logResultsFrom(retryAttempt(3, 5))).
			To(Say("Upgrading all remaining instances"))
	})

	It("Shows upgrading all canaries during first attempt", func() {
		Expect(logResultsFrom(retryCanariesAttempt(1, 5, 3))).
			To(Say("Upgrading all canaries"))
	})

	It("Shows upgrading all remaining canaries during later attempts", func() {
		Expect(logResultsFrom(retryCanariesAttempt(3, 5, 2))).
			To(Say("Upgrading 2 remaining canaries"))
	})

	It("Shows which instances to upgrade", func() {
		Expect(logResultsFrom(func(listener upgrader.Listener) {
			listener.InstancesToUpgrade([]service.Instance{{GUID: "one"}, {GUID: "two"}})
		})).
			To(Say("Service Instances: one two"))
	})

	It("Shows which instance has started upgrading", func() {
		buffer := logResultsFrom(func(listener upgrader.Listener) {
			listener.InstanceUpgradeStarting("service-instance", 2, 5)
		})

		Expect(buffer).To(Say(`\[service-instance\] Starting to upgrade service instance 2 of 5`))
	})

	Describe("instance upgrade start result", func() {
		var (
			result services.UpgradeOperationType
			buffer *Buffer
		)

		JustBeforeEach(func() {
			buffer = logResultsFrom(func(listener upgrader.Listener) {
				listener.InstanceUpgradeStartResult("service-instance", result)
			})
		})

		Context("when accepted", func() {
			BeforeEach(func() {
				result = services.UpgradeAccepted
			})

			It("Shows accepted upgrade", func() {
				Expect(buffer).To(Say(`\[service-instance\] Result: accepted upgrade`))
			})
		})

		Context("when not found", func() {
			BeforeEach(func() {
				result = services.InstanceNotFound
			})

			It("shows already deleted in CF", func() {
				Expect(buffer).To(Say(`\[service-instance\] Result: already deleted in CF`))
			})
		})

		Context("when orphaned", func() {
			BeforeEach(func() {
				result = services.OrphanDeployment
			})

			It("shows already deleted in CF", func() {
				Expect(buffer).To(Say(`\[service-instance\] Result: orphan CF service instance detected - no corresponding bosh deployment`))
			})
		})

		Context("when conflict", func() {
			BeforeEach(func() {
				result = services.OperationInProgress
			})

			It("shows already deleted in CF", func() {
				Expect(buffer).To(Say(`\[service-instance\] Result: operation in progress`))
			})
		})

		Context("when error", func() {
			BeforeEach(func() {
				result = services.UpgradeOperationType(-1)
			})

			It("shows already deleted in CF", func() {
				Expect(buffer).To(Say(`\[service-instance\] Result: unexpected result`))
			})
		})
	})

	It("Shows which instance is still in progress", func() {
		Expect(logResultsFrom(func(listener upgrader.Listener) { listener.WaitingFor("one", 999) })).
			To(Say(`\[one\] Waiting for upgrade to complete: bosh task id 999`))
	})

	It("Shows which instance has been upgraded", func() {
		Expect(logResultsFrom(func(listener upgrader.Listener) { listener.InstanceUpgraded("one", "success") })).
			To(Say(`\[one\] Result: Service Instance upgrade success`))
	})

	It("Shows a summary of the progress so far", func() {
		buffer := logResultsFrom(func(listener upgrader.Listener) {
			listener.Progress(ten_seconds, 234, 345, 456, 567)
		})

		Expect(buffer).To(Say("Sleep interval until next attempt: 10s"))
		Expect(buffer).To(Say("Number of successful upgrades so far: 345"))
		Expect(buffer).To(Say("Number of CF service instance orphans detected so far: 234"))
		Expect(buffer).To(Say("Number of deleted instances before upgrade could occur: 567"))
		Expect(buffer).To(Say("Number of operations in progress \\(to retry\\) so far: 456"))
	})

	It("Shows a final summary", func() {
		buffer := logResultsFrom(func(listener upgrader.Listener) {
			listener.Finished(23, 34, 45, 56)
		})

		Expect(buffer).To(Say("FINISHED UPGRADES"))
		Expect(buffer).To(Say("Number of successful upgrades: 34"))
		Expect(buffer).To(Say("Number of CF service instance orphans detected: 23"))
		Expect(buffer).To(Say("Number of deleted instances before upgrade could occur: 45"))
		Expect(buffer).To(Say("Number of busy instances which could not be upgraded: 56"))
	})
})

func logResultsFrom(action func(listener upgrader.Listener)) *Buffer {
	logBuffer := NewBuffer()
	loggerFactory := loggerfactory.New(io.MultiWriter(GinkgoWriter, logBuffer), "logging-listener-tests", log.LstdFlags)
	listener := upgrader.NewLoggingListener(loggerFactory.New())

	action(listener)

	return logBuffer
}

func retryAttempt(num, limit int) func(listener upgrader.Listener) {
	return func(listener upgrader.Listener) {
		listener.RetryAttempt(num, limit)
	}
}

func retryCanariesAttempt(num, limit, n int) func(listener upgrader.Listener) {
	return func(listener upgrader.Listener) {
		listener.RetryCanariesAttempt(num, limit, n)
	}
}
