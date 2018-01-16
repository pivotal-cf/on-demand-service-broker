// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cloudfoundry/bosh-cli/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
)

var _ = Describe("Bosh Tasks", func() {
	var (
		taskID   = 2112
		fakeTask *fakes.FakeTask
	)

	BeforeEach(func() {
		fakeTask = new(fakes.FakeTask)
		fakeTask.IDReturns(taskID)
		fakeTask.StateReturns("state")
		fakeTask.DescriptionReturns("description")
		fakeTask.ResultReturns("result")
		fakeTask.ContextIDReturns("contextID")

		fakeDirector.FindTaskStub = func(id int) (director.Task, error) {
			if id == taskID {
				return fakeTask, nil
			}
			return nil, fmt.Errorf("errored")
		}
	})

	Describe("GetTask", func() {
		It("gets the task state", func() {
			taskState, err := c.GetTask(taskID, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(taskState).To(Equal(boshdirector.BoshTask{
				ID:          taskID,
				State:       "state",
				Description: "description",
				Result:      "result",
				ContextID:   "contextID",
			}))
		})

		It("returns an error if the getting the task fails", func() {
			_, err := c.GetTask(-1, logger)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("Cannot find task with ID: -1")))
		})
	})

	Describe("GetTaskOutput", func() {
		BeforeEach(func() {
			expectedTaskOutput := boshdirector.BoshTaskOutput{
				ExitCode: 42,
				StdOut:   "a-description",
				StdErr:   "err",
			}
			body := bytes.NewBuffer([]byte{})
			encoder := json.NewEncoder(body)
			err := encoder.Encode(expectedTaskOutput)
			Expect(err).NotTo(HaveOccurred())

			fakeTask.ResultOutputStub = func(reporter director.TaskReporter) error {
				reporter.TaskOutputChunk(192, body.Bytes())
				return nil
			}
		})

		It("gets the task result", func() {
			taskOutput, err := c.GetTaskOutput(taskID, logger)
			Expect(err).NotTo(HaveOccurred())

			By("calling ResultOutput")
			Expect(fakeTask.ResultOutputCallCount()).To(Equal(1))
			Expect(fakeTask.ResultOutputArgsForCall(0)).To(BeAssignableToTypeOf(&boshdirector.BoshTaskOutputReporter{}))

			By("returning the result output")
			Expect(taskOutput).To(Equal(boshdirector.BoshTaskOutput{}))
		})

		It("returns empty when the task doesn't have output", func() {
			fakeTask.ResultOutputReturns(nil)

			taskOutput, err := c.GetTaskOutput(taskID, logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(taskOutput).To(Equal(boshdirector.BoshTaskOutput{}))
		})

		It("errors when it fails to fetch the task", func() {
			fakeDirector.FindTaskReturns(nil, errors.New("boom"))
			taskOutput, err := c.GetTaskOutput(taskID, logger)

			Expect(taskOutput).To(Equal(boshdirector.BoshTaskOutput{}))
			Expect(err).To(MatchError(fmt.Sprintf("Could not fetch task with id %d: boom", taskID)))
		})

		It("errors when it fails to fetch the output", func() {
			fakeTask.ResultOutputReturns(errors.New("boom"))
			taskOutput, err := c.GetTaskOutput(taskID, logger)

			Expect(taskOutput).To(Equal(boshdirector.BoshTaskOutput{}))
			Expect(err).To(MatchError("Could not fetch task output: boom"))
		})
	})
})
