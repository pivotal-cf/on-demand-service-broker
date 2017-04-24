// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshclient_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshclient"
	"github.com/pivotal-cf/on-demand-service-broker/mockbosh"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
)

var _ = Describe("vms", func() {
	Describe("getting VM info", func() {
		var (
			name = "some-deployment"

			vms    bosh.BoshVMs
			vmsErr error

			taskIDToReturn = 42
		)

		JustBeforeEach(func() {
			vms, vmsErr = c.VMs(name, logger)
		})

		Context("when bosh starts VMs task successfully", func() {
			Context("when task state can be fetched from bosh successfully", func() {
				Context("when task output has one instance group", func() {
					BeforeEach(func() {
						director.VerifyAndMock(
							mockbosh.VMsForDeployment(name).RedirectsToTask(taskIDToReturn),
							mockbosh.Task(taskIDToReturn).RespondsWithTaskContainingState(boshclient.BoshTaskDone),
							mockbosh.TaskOutput(taskIDToReturn).RespondsWithVMsOutput([]boshclient.BoshVMsOutput{
								{IPs: []string{"ip1", "ip2"}, InstanceGroup: "an-instance-group"},
							}),
						)
					})

					It("returns BOSH VMs", func() {
						Expect(vms).To(HaveLen(1))
						Expect(vms["an-instance-group"]).To(ConsistOf("ip1", "ip2"))
					})

					It("calls the authorization header builder", func() {
						Expect(authHeaderBuilder.BuildCallCount()).To(BeNumerically(">", 0))
					})

					It("returns no error", func() {
						Expect(vmsErr).NotTo(HaveOccurred())
					})
				})

				Context("the output has multiple lines of the same instance group", func() {
					BeforeEach(func() {
						director.VerifyAndMock(
							mockbosh.VMsForDeployment(name).RedirectsToTask(taskIDToReturn),
							mockbosh.Task(taskIDToReturn).RespondsWithTaskContainingState(boshclient.BoshTaskDone),
							mockbosh.TaskOutput(taskIDToReturn).RespondsWithVMsOutput([]boshclient.BoshVMsOutput{
								{IPs: []string{"ip1"}, InstanceGroup: "kafka-broker"},
								{IPs: []string{"ip2"}, InstanceGroup: "kafka-broker"},
								{IPs: []string{"ip3"}, InstanceGroup: "zookeeper"},
							}),
						)
					})

					It("groups ips by instance group name", func() {
						Expect(vms).To(HaveLen(2))
						Expect(vms).To(HaveKeyWithValue("kafka-broker", []string{"ip1", "ip2"}))
						Expect(vms).To(HaveKeyWithValue("zookeeper", []string{"ip3"}))
					})
				})

				Context("when the task is finished, but has failed", func() {
					BeforeEach(func() {
						director.VerifyAndMock(
							mockbosh.VMsForDeployment(name).RedirectsToTask(taskIDToReturn),
							mockbosh.Task(taskIDToReturn).RespondsWithTaskContainingState(boshclient.BoshTaskError),
						)
					})

					It("returns an error", func() {
						Expect(vmsErr).To(MatchError(ContainSubstring(fmt.Sprintf("task %d failed", taskIDToReturn))))
					})
				})

				Context("when fetching task output from bosh fails", func() {
					BeforeEach(func() {
						director.VerifyAndMock(
							mockbosh.VMsForDeployment(name).Fails("because reasons"),
						)
					})

					It("returns an error", func() {
						Expect(vmsErr).To(MatchError(ContainSubstring("expected status 302, was 500")))
					})
				})

				Context("when the deployment is not found", func() {
					BeforeEach(func() {
						director.VerifyAndMock(
							mockbosh.VMsForDeployment(name).NotFound(),
						)
					})

					It("returns an error", func() {
						Expect(vmsErr).To(BeAssignableToTypeOf(boshclient.DeploymentNotFoundError{}))
					})
				})
			})

			Context("when the authorization header cannot be generated", func() {
				BeforeEach(func() {
					authHeaderBuilder.BuildReturns("", errors.New("some-error"))
				})

				It("returns an error", func() {
					Expect(vmsErr).To(MatchError(ContainSubstring("some-error")))
				})
			})

			Context("when fetching task state from bosh fails", func() {
				BeforeEach(func() {
					director.VerifyAndMock(
						mockbosh.VMsForDeployment(name).RedirectsToTask(taskIDToReturn),
						mockbosh.Task(taskIDToReturn).Fails("because reasons"),
					)
				})

				It("returns an error", func() {
					Expect(vmsErr).To(MatchError(ContainSubstring("expected status 200, was 500")))
				})
			})
		})

		Context("when bosh fails to start VMs task", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.VMsForDeployment(name).Fails("because reasons"),
				)
			})

			It("returns an error", func() {
				Expect(vmsErr).To(MatchError(ContainSubstring("expected status 302, was 500")))
			})
		})
	})

	Describe("getting output from a BOSH VMs task", func() {
		var (
			taskID            = 2
			expectedVMsOutput = []boshclient.BoshVMsOutput{
				{IPs: []string{"a-nice-ip"}, InstanceGroup: "a-nice-instance-group"},
			}

			boshOutput []boshclient.BoshVMsOutput
			outputErr  error
		)

		JustBeforeEach(func() {
			boshOutput, outputErr = c.VMsOutput(taskID, logger)
		})

		Context("when bosh succeeds", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.TaskOutput(taskID).RespondsWithVMsOutput(expectedVMsOutput),
				)
			})

			It("calls the authorization header builder", func() {
				Expect(authHeaderBuilder.BuildCallCount()).To(BeNumerically(">", 0))
			})

			It("does not error", func() {
				Expect(outputErr).NotTo(HaveOccurred())
			})

			It("returns the IPs in the output", func() {
				Expect(boshOutput).To(ConsistOf(expectedVMsOutput[0]))
			})
		})

		Context("when bosh fails", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.TaskOutput(taskID).Fails("because reasons"),
				)
			})

			It("returns an error", func() {
				Expect(outputErr).To(MatchError(ContainSubstring("expected status 200, was 500.")))
			})
		})

		Context("when the Authorization header cannot be generated", func() {
			BeforeEach(func() {
				authHeaderBuilder.BuildReturns("", errors.New("some-error"))
			})

			It("returns an error", func() {
				Expect(outputErr).To(MatchError(ContainSubstring("some-error")))
			})
		})
	})
})
