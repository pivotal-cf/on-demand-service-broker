// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"errors"
	"fmt"

	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
)

type NetworkActivity struct {
	Response        *http.Response
	Error           error
	ExpectedRequest receivedHttpRequest
}

func appendRequestsToClient(fakeHTTPClient *fakes.FakeNetworkDoer, offset int, networkActivities ...NetworkActivity) []types.GomegaMatcher {
	var expectedHttpRequests []types.GomegaMatcher

	for i, activity := range networkActivities {
		fakeHTTPClient.DoReturnsOnCall(i+offset, activity.Response, activity.Error)
		expectedHttpRequests = append(expectedHttpRequests, HaveReceivedHttpRequestAtIndex(activity.ExpectedRequest, i+offset))
	}

	return expectedHttpRequests
}

var _ = Describe("vms", func() {
	Describe("getting VM info", func() {

		const (
			taskIDToReturn = 42
			deploymentName = "some-deployment"
		)

		Context("when bosh starts VMs task successfully", func() {
			Context("when task state can be fetched from bosh successfully", func() {
				It("returns the BOSH VMs when task output has one instance group", func() {
					expectedTaskOutputs := []boshdirector.BoshVMsOutput{
						{IPs: []string{"ip1", "ip2"}, InstanceGroup: "an-instance-group"},
					}

					const expectedPreviousRequests = 1

					activities := []NetworkActivity{
						{
							Response: responseWithRedirectToTaskID(taskIDToReturn),
							ExpectedRequest: receivedHttpRequest{
								Path:   fmt.Sprintf("/deployments/%s/vms", deploymentName),
								Method: "GET",
							},
						},
						{
							Response: responseOKWithJSON(boshdirector.BoshTask{
								ID:    taskIDToReturn,
								State: boshdirector.TaskDone,
							}),
							ExpectedRequest: receivedHttpRequest{
								Path:   fmt.Sprintf("/tasks/%d", taskIDToReturn),
								Method: "GET",
							},
						},
						{
							Response: responseOKWithTaskOutput(expectedTaskOutputs),
							ExpectedRequest: receivedHttpRequest{
								Path:   fmt.Sprintf("/tasks/%d/output", taskIDToReturn),
								Method: "GET",
							},
						},
					}
					expectedHttpRequests := appendRequestsToClient(fakeHTTPClient, expectedPreviousRequests, activities...)

					vms, vmsErr := c.VMs(deploymentName, logger)

					Expect(vms).To(HaveLen(1))
					Expect(vmsErr).NotTo(HaveOccurred())
					Expect(vms["an-instance-group"]).To(ConsistOf("ip1", "ip2"))

					By("calling the right endpoints")
					Expect(fakeHTTPClient).To(SatisfyAll(expectedHttpRequests...))
					Expect(authHeaderBuilder.AddAuthHeaderCallCount()).To(BeNumerically(">", 0))
				})

				It("groups ips by instance group deploymentName when the output has multiple lines of the same instance group", func() {
					fakeHTTPClient.DoReturnsOnCall(1, responseWithRedirectToTaskID(taskIDToReturn), nil)
					fakeHTTPClient.DoReturnsOnCall(2, responseOKWithJSON(boshdirector.BoshTask{
						ID:    taskIDToReturn,
						State: boshdirector.TaskDone,
					}), nil)
					fakeHTTPClient.DoReturnsOnCall(3, responseOKWithTaskOutput([]boshdirector.BoshVMsOutput{
						{IPs: []string{"ip1"}, InstanceGroup: "kafka-broker"},
						{IPs: []string{"ip2"}, InstanceGroup: "kafka-broker"},
						{IPs: []string{"ip3"}, InstanceGroup: "zookeeper"},
					}), nil)

					vms, err := c.VMs(deploymentName, logger)

					Expect(err).NotTo(HaveOccurred())
					Expect(vms).To(HaveLen(2))
					Expect(vms).To(HaveKeyWithValue("kafka-broker", []string{"ip1", "ip2"}))
					Expect(vms).To(HaveKeyWithValue("zookeeper", []string{"ip3"}))
				})

				It("returns an error when the task is finished, but has failed", func() {
					fakeHTTPClient.DoReturnsOnCall(1, responseWithRedirectToTaskID(taskIDToReturn), nil)
					fakeHTTPClient.DoReturnsOnCall(2, responseOKWithJSON(boshdirector.BoshTask{
						ID:    taskIDToReturn,
						State: boshdirector.TaskError,
					}), nil)

					_, err := c.VMs(deploymentName, logger)

					Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("task %d failed", taskIDToReturn))))
				})

				It("returns an error when fetching task output from bosh fails", func() {
					fakeHTTPClient.DoReturnsOnCall(1, responseWithEmptyBodyAndStatus(http.StatusInternalServerError), nil)

					_, err := c.VMs(deploymentName, logger)

					Expect(err).To(MatchError(ContainSubstring("expected status 302, was 500")))
				})

				It("returns an error when the deployment is not found", func() {
					fakeHTTPClient.DoReturnsOnCall(1, responseWithEmptyBodyAndStatus(http.StatusNotFound), nil)

					_, err := c.VMs(deploymentName, logger)

					Expect(err).To(BeAssignableToTypeOf(boshdirector.DeploymentNotFoundError{}))
				})
			})

			It("returns an error when the authorization header cannot be generated", func() {
				authHeaderBuilder.AddAuthHeaderReturns(errors.New("some-error"))

				_, err := c.VMs(deploymentName, logger)

				Expect(err).To(MatchError(ContainSubstring("some-error")))
			})

			It("returns an error when fetching task state from bosh fails", func() {
				fakeHTTPClient.DoReturnsOnCall(1, responseWithRedirectToTaskID(taskIDToReturn), nil)
				fakeHTTPClient.DoReturnsOnCall(2, responseWithEmptyBodyAndStatus(http.StatusInternalServerError), nil)

				_, err := c.VMs(deploymentName, logger)

				Expect(err).To(MatchError(ContainSubstring("expected status 200, was 500")))
			})
		})

		It("returns an error when bosh fails to start VMs task", func() {
			fakeHTTPClient.DoReturnsOnCall(1, responseWithEmptyBodyAndStatus(http.StatusInternalServerError), nil)

			_, err := c.VMs(deploymentName, logger)

			Expect(err).To(MatchError(ContainSubstring("expected status 302, was 500")))
		})
	})

	Describe("getting output from a BOSH VMs task", func() {
		const (
			taskID = 2
		)
		var (
			expectedVMsOutput = []boshdirector.BoshVMsOutput{
				{IPs: []string{"a-nice-ip"}, InstanceGroup: "a-nice-instance-group"},
			}
		)

		It("returns the IPs in the output when bosh succeeds", func() {
			fakeHTTPClient.DoReturnsOnCall(1, responseOKWithTaskOutput(expectedVMsOutput), nil)

			boshOutput, err := c.VMsOutput(taskID, logger)

			Expect(err).NotTo(HaveOccurred())
			Expect(boshOutput).To(ConsistOf(expectedVMsOutput[0]))

			By("calling the authorization header builder")
			Expect(authHeaderBuilder.AddAuthHeaderCallCount()).To(BeNumerically(">", 0))

		})

		It("returns an error when bosh fails", func() {
			fakeHTTPClient.DoReturnsOnCall(1, responseWithEmptyBodyAndStatus(http.StatusInternalServerError), nil)

			_, err := c.VMsOutput(taskID, logger)

			Expect(err).To(MatchError(ContainSubstring("expected status 200, was 500.")))
		})

		It("returns an error when the Authorization header cannot be generated", func() {
			authHeaderBuilder.AddAuthHeaderReturns(errors.New("some-error"))

			_, err := c.VMsOutput(taskID, logger)

			Expect(err).To(MatchError(ContainSubstring("some-error")))
		})
	})
})
