// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

var _ = Describe("Task", func() {
	Describe("ToLog", func() {
		It("Returns a JSON serialised string representing the task", func() {
			task := BoshTask{
				ID:          1,
				State:       TaskProcessing,
				Description: "snapshot deployment",
				Result:      "result-1",
			}

			Expect(task.ToLog()).To(Equal(fmt.Sprintf(
				`{"ID":1,"State":"%s","Description":"snapshot deployment","Result":"result-1"}`,
				TaskProcessing,
			)))
		})
	})

	DescribeTable("StateType",
		func(state string, expected TaskStateType) {
			t := BoshTask{State: state}
			Expect(t.StateType()).To(Equal(expected))
		},
		Entry("when done", TaskDone, TaskComplete),
		Entry("when processing", TaskProcessing, TaskIncomplete),
		Entry("when queued", TaskQueued, TaskIncomplete),
		Entry("when cancelled", TaskCancelled, TaskFailed),
		Entry("when cancelling", TaskCancelling, TaskIncomplete),
		Entry("when error", TaskError, TaskFailed),
		Entry("when timeout", TaskTimeout, TaskFailed),
		Entry("when something unknown", "nonsense", TaskUnknown),
	)
})
