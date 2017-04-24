// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshclient_test

import (
	"fmt"

	. "github.com/pivotal-cf/on-demand-service-broker/boshclient"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Task", func() {
	Describe("ToLog", func() {
		It("Returns a JSON serialised string representing the task", func() {
			task := BoshTask{
				ID:          1,
				State:       BoshTaskProcessing,
				Description: "snapshot deployment",
				Result:      "result-1",
			}

			Expect(task.ToLog()).To(Equal(fmt.Sprintf(
				`{"ID":1,"State":"%s","Description":"snapshot deployment","Result":"result-1"}`,
				BoshTaskProcessing,
			)))
		})
	})

	DescribeTable("StateType",
		func(state string, expected TaskStateType) {
			t := BoshTask{State: state}
			Expect(t.StateType()).To(Equal(expected))
		},
		Entry("when done", BoshTaskDone, TaskDone),
		Entry("when processing", BoshTaskProcessing, TaskIncomplete),
		Entry("when queued", BoshTaskQueued, TaskIncomplete),
		Entry("when cancelled", BoshTaskCancelled, TaskFailed),
		Entry("when cancelling", BoshTaskCancelling, TaskIncomplete),
		Entry("when error", BoshTaskError, TaskFailed),
		Entry("when timeout", BoshTaskTimeout, TaskFailed),
		Entry("when something unknown", "nonsense", TaskUnknown),
	)
})
