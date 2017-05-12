// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
)

var _ = Describe("get task", func() {
	var (
		taskID = 2112
	)

	Context("getting task state", func() {
		var (
			taskState  boshdirector.BoshTask
			getTaskErr error
		)

		JustBeforeEach(func() {
			taskState, getTaskErr = c.GetTask(taskID, logger)
		})

		Context("when bosh fetches the task successfully", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.Task(taskID).RespondsOKWithJSON(boshdirector.BoshTask{State: "a-state", Description: "a-description"}),
				)
			})

			It("returns the task state", func() {
				Expect(taskState).To(Equal(boshdirector.BoshTask{State: "a-state", Description: "a-description"}))
			})

			It("does not error", func() {
				Expect(getTaskErr).NotTo(HaveOccurred())
			})
		})

		Context("when bosh fails to fetch the task", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.Task(taskID).RespondsInternalServerErrorWith("because reasons"),
				)
			})

			It("wraps the error", func() {
				Expect(getTaskErr).To(MatchError(ContainSubstring("expected status 200, was 500")))
			})
		})
	})

	Context("getting task output", func() {
		var (
			actualTaskOutput    []boshdirector.BoshTaskOutput
			actualTaskOutputErr error
			expectedTaskOutputs []boshdirector.BoshTaskOutput
		)

		JustBeforeEach(func() {
			actualTaskOutput, actualTaskOutputErr = c.GetTaskOutput(taskID, logger)
		})

		Context("when the client fetches task output of a valid errand task", func() {
			BeforeEach(func() {
				expectedTaskOutputs = []boshdirector.BoshTaskOutput{
					{ExitCode: 1, StdOut: "a-description", StdErr: "err"},
					{ExitCode: 2, StdOut: "a-description", StdErr: "err"},
					{ExitCode: 3, StdOut: "a-description", StdErr: "err"},
					{ExitCode: 5, StdOut: "a-description", StdErr: "err"},
				}
				director.VerifyAndMock(
					mockbosh.TaskOutput(taskID).RespondsWithTaskOutput(expectedTaskOutputs),
				)
			})

			It("returns the correct output of the task", func() {
				Expect(actualTaskOutputErr).NotTo(HaveOccurred())
				Expect(actualTaskOutput).To(ConsistOf(expectedTaskOutputs))
			})
		})

		Context("when there is no output", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.TaskOutput(taskID).RespondsWithTaskOutput([]boshdirector.BoshTaskOutput{}),
				)
			})

			It("returns the correct output of the task", func() {
				Expect(actualTaskOutputErr).NotTo(HaveOccurred())
				Expect(actualTaskOutput).To(BeEmpty())
			})
		})

		Context("when bosh fails to fetch the output", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.TaskOutput(taskID).RespondsInternalServerErrorWith("because reasons"),
				)
			})

			It("returns an error", func() {
				Expect(actualTaskOutputErr).To(MatchError(ContainSubstring("expected status 200, was 500.")))
			})
		})
	})
})
