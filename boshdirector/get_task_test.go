// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

var _ = Describe("get task", func() {
	var (
		taskID = 2112
	)

	Context("getting task state", func() {
		It("returns the task state when bosh fetches the task successfully", func() {
			fakeHTTPClient.DoReturns(responseOKWithJSON(boshdirector.BoshTask{State: "a-state", Description: "a-description"}), nil)
			taskState, err := c.GetTask(taskID, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(taskState).To(Equal(boshdirector.BoshTask{State: "a-state", Description: "a-description"}))

			By("calling the right endpoint")
			Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
				receivedHttpRequest{
					Path:   fmt.Sprintf("/tasks/%d", taskID),
					Method: "GET",
				}, 1))
		})

		It("wraps the error when when bosh fails to fetch the task", func() {
			fakeHTTPClient.DoReturns(responseWithEmptyBodyAndStatus(http.StatusInternalServerError), nil)
			_, err := c.GetTask(taskID, logger)
			Expect(err).To(MatchError(ContainSubstring("expected status 200, was 500")))
		})
	})

	Context("getting task output", func() {
		It("returns the correct output for the task when the client fetches task output of a valid errand task", func() {
			expectedTaskOutputs := []boshdirector.BoshTaskOutput{
				{ExitCode: 1, StdOut: "a-description", StdErr: "err"},
				{ExitCode: 2, StdOut: "a-description", StdErr: "err"},
				{ExitCode: 3, StdOut: "a-description", StdErr: "err"},
				{ExitCode: 5, StdOut: "a-description", StdErr: "err"},
			}

			body := bytes.NewBuffer([]byte{})
			encoder := json.NewEncoder(body)

			for _, line := range expectedTaskOutputs {
				Expect(encoder.Encode(line)).ToNot(HaveOccurred())
			}

			fakeHTTPClient.DoReturns(responseOKWithRawBody(body.Bytes()), nil)
			taskOutput, err := c.GetTaskOutput(taskID, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(taskOutput).To(Equal(expectedTaskOutputs))

			By("calling the right endpoint")
			Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
				receivedHttpRequest{
					Path:   fmt.Sprintf("/tasks/%d/output", taskID),
					Method: "GET",
					Query:  url.Values{"type": []string{"result"}},
				}, 1))
		})

		It("returns the correct output for the task when there is no output", func() {
			fakeHTTPClient.DoReturns(responseOKWithRawBody([]byte{}), nil)
			taskOutput, err := c.GetTaskOutput(taskID, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(taskOutput).To(BeEmpty())
		})

		It("wraps the error when when bosh fails to fetch the output", func() {
			fakeHTTPClient.DoReturns(responseWithEmptyBodyAndStatus(http.StatusInternalServerError), nil)
			_, err := c.GetTaskOutput(taskID, logger)

			Expect(err).To(MatchError(ContainSubstring("expected status 200, was 500")))
		})
	})
})
