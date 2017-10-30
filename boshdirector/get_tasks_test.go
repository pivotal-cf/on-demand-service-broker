// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"net/url"

	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

var _ = Describe("getting tasks", func() {
	var (
		deploymentName = "an-amazing-deployment"
	)

	Describe("GetTasks", func() {
		It("returns the task state when bosh fetches the task successfully", func() {
			expectedTasks := boshdirector.BoshTasks{
				{State: boshdirector.TaskProcessing, Description: "snapshot deployment", Result: "result-1"},
				{State: boshdirector.TaskDone, Description: "snapshot deployment", Result: "result-2"},
			}
			fakeHTTPClient.DoReturns(responseOKWithJSON(expectedTasks), nil)
			actualTasks, err := c.GetTasks(deploymentName, logger)
			Expect(actualTasks).To(Equal(expectedTasks))
			Expect(err).NotTo(HaveOccurred())

			By("calling the right endpoint")
			Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
				receivedHttpRequest{
					Path:   "/tasks",
					Method: "GET",
					Query:  url.Values{"deployment": []string{deploymentName}},
				}, 1))

		})

		It("wraps the error when bosh returns a client error (HTTP 404)", func() {
			fakeHTTPClient.DoReturns(responseWithEmptyBodyAndStatus(http.StatusNotFound), nil)
			_, err := c.GetTasks(deploymentName, logger)
			Expect(err).To(MatchError(ContainSubstring("expected status 200, was 404")))

		})

		It("wraps the error when bosh fails to fetch the task", func() {
			fakeHTTPClient.DoReturns(responseWithEmptyBodyAndStatus(http.StatusInternalServerError), nil)
			_, err := c.GetTasks(deploymentName, logger)
			Expect(err).To(MatchError(ContainSubstring("expected status 200, was 500")))
		})
	})

	Describe("GetTasksByContextID", func() {
		var (
			contextID = "some-context-id"
		)

		It("returns no tasks when there are no tasks with the context id", func() {
			fakeHTTPClient.DoReturns(responseOKWithJSON([]boshdirector.BoshTask{}), nil)
			actualTasks, err := c.GetNormalisedTasksByContext(deploymentName, contextID, logger)
			Expect(actualTasks).To(HaveLen(0))
			Expect(err).NotTo(HaveOccurred())

			By("calling the right endpoint")
			Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
				receivedHttpRequest{
					Path:   "/tasks",
					Method: "GET",
					Query:  url.Values{"deployment": []string{deploymentName}, "context_id": []string{contextID}},
				}, 1))
		})

		It("returns one task when there is one task with the context id", func() {
			expectedTask := boshdirector.BoshTask{State: boshdirector.TaskProcessing, Description: "snapshot deployment", Result: "result-1", ContextID: contextID}

			fakeHTTPClient.DoReturns(responseOKWithJSON([]boshdirector.BoshTask{expectedTask}), nil)
			actualTasks, err := c.GetNormalisedTasksByContext(deploymentName, contextID, logger)
			Expect(actualTasks).To(HaveLen(1))
			Expect(actualTasks[0]).To(Equal(expectedTask))
			Expect(err).NotTo(HaveOccurred())

			By("calling the right endpoint")
			Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
				receivedHttpRequest{
					Path:   "/tasks",
					Method: "GET",
					Query:  url.Values{"deployment": []string{deploymentName}, "context_id": []string{contextID}},
				}, 1))

		})

		It("returns the correct tasks when there are many tasks with the context id", func() {
			expectedTasks := boshdirector.BoshTasks{
				{State: boshdirector.TaskProcessing, Description: "snapshot deployment", Result: "result-1", ContextID: contextID},
				{State: boshdirector.TaskDone, Description: "something finished", Result: "result-1", ContextID: contextID},
				{State: boshdirector.TaskProcessing, Description: "snapshot deployment", Result: "result-1", ContextID: contextID},
			}

			fakeHTTPClient.DoReturnsOnCall(1, responseOKWithJSON(expectedTasks), nil)
			fakeHTTPClient.DoReturnsOnCall(2, responseWithEmptyBodyAndStatus(http.StatusOK), nil)

			actualTasks, err := c.GetNormalisedTasksByContext(deploymentName, contextID, logger)
			Expect(actualTasks).To(HaveLen(3))
			Expect(actualTasks).To(Equal(expectedTasks))
			Expect(err).To(Not(HaveOccurred()))

			By("calling the right endpoint")
			Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
				receivedHttpRequest{
					Path:   "/tasks",
					Method: "GET",
					Query:  url.Values{"deployment": []string{deploymentName}, "context_id": []string{contextID}},
				}, 1))

			Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
				receivedHttpRequest{
					Path:   "/tasks/0/output",
					Method: "GET",
					Query:  url.Values{"type": []string{"result"}},
				}, 2))

		})

		It("returns the correct task when an errand task has finished with a non-zero exit code", func() {
			expectedTask := boshdirector.BoshTask{
				ID:          42,
				State:       boshdirector.TaskDone,
				Description: "errand completed",
				Result:      "result-1",
				ContextID:   contextID,
			}

			fakeHTTPClient.DoReturnsOnCall(1, responseOKWithJSON([]boshdirector.BoshTask{expectedTask}), nil)
			fakeHTTPClient.DoReturnsOnCall(2, responseOKWithJSON(map[string]int{"ExitCode": 1}), nil)

			actualTasks, err := c.GetNormalisedTasksByContext(deploymentName, contextID, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualTasks).To(Equal(boshdirector.BoshTasks{expectedTask}))

			By("calling the right endpoints")
			Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
				receivedHttpRequest{
					Path:   "/tasks",
					Method: "GET",
					Query:  url.Values{"deployment": []string{deploymentName}, "context_id": []string{contextID}},
				}, 1))
			Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
				receivedHttpRequest{
					Path:   "/tasks/42/output",
					Method: "GET",
					Query:  url.Values{"type": []string{"result"}},
				}, 2))
		})

		It("returns an error when an errand task result can not be retrived", func() {
			expectedTask := boshdirector.BoshTask{
				ID:          42,
				State:       boshdirector.TaskDone,
				Description: "errand completed",
				Result:      "result-1",
				ContextID:   contextID,
			}

			fakeHTTPClient.DoReturnsOnCall(1, responseOKWithJSON([]boshdirector.BoshTask{expectedTask}), nil)
			fakeHTTPClient.DoReturnsOnCall(2, responseWithEmptyBodyAndStatus(http.StatusInternalServerError), nil)

			_, err := c.GetNormalisedTasksByContext(deploymentName, contextID, logger)
			Expect(err).To(HaveOccurred())

			By("calling the right endpoints")
			Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
				receivedHttpRequest{
					Path:   "/tasks",
					Method: "GET",
					Query:  url.Values{"deployment": []string{deploymentName}, "context_id": []string{contextID}},
				}, 1))
			Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
				receivedHttpRequest{
					Path:   "/tasks/42/output",
					Method: "GET",
					Query:  url.Values{"type": []string{"result"}},
				}, 2))

		})
	})
})
