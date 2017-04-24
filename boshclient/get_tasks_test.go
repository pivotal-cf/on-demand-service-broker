// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshclient_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshclient"
	"github.com/pivotal-cf/on-demand-service-broker/mockbosh"
)

var _ = Describe("getting tasks", func() {
	Describe("GetTasks", func() {
		var (
			deploymentName   = "an-amazing-deployment"
			actualTasks      boshclient.BoshTasks
			actualTasksError error

			expectedTasks = boshclient.BoshTasks{
				{State: boshclient.BoshTaskProcessing, Description: "snapshot deployment", Result: "result-1"},
				{State: boshclient.BoshTaskDone, Description: "snapshot deployment", Result: "result-2"},
			}
		)

		JustBeforeEach(func() {
			actualTasks, actualTasksError = c.GetTasks(deploymentName, logger)
		})

		Context("when bosh fetches the task successfully", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.Tasks(deploymentName).RespondsWithJson(expectedTasks),
				)
			})

			It("returns the task state", func() {
				Expect(actualTasks).To(Equal(expectedTasks))
			})

			It("does not error", func() {
				Expect(actualTasksError).NotTo(HaveOccurred())
			})
		})

		Context("when bosh returns a client error (HTTP 404)", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.Tasks(deploymentName).NotFound(),
				)
			})

			It("wraps the error", func() {
				Expect(actualTasksError).To(MatchError(ContainSubstring("expected status 200, was 404")))
			})
		})

		Context("when bosh fails to fetch the task", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.Tasks(deploymentName).Fails("because reasons"),
				)
			})

			It("wraps the error", func() {
				Expect(actualTasksError).To(MatchError(ContainSubstring("expected status 200, was 500")))
			})
		})
	})

	Describe("GetTasksByContextID", func() {
		const (
			contextID      = "some-id"
			deploymentName = "some-deployment"
		)
		var (
			actualTasks boshclient.BoshTasks
			actualError error
		)
		Context("when there are no tasks with the context id", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName, contextID).RespondsWithNoTasks(),
				)
			})

			JustBeforeEach(func() {
				actualTasks, actualError = c.GetNormalisedTasksByContext(deploymentName, contextID, logger)
			})

			It("returns no tasks", func() {
				Expect(actualTasks).To(HaveLen(0))
			})

			It("returns no error", func() {
				Expect(actualError).To(Not(HaveOccurred()))
			})
		})

		Context("when there is one task with the context id", func() {
			expectedTask := boshclient.BoshTask{State: boshclient.BoshTaskProcessing, Description: "snapshot deployment", Result: "result-1", ContextID: contextID}

			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName, contextID).RespondsWithATask(expectedTask),
				)
			})

			JustBeforeEach(func() {
				actualTasks, actualError = c.GetNormalisedTasksByContext(deploymentName, contextID, logger)
			})

			It("returns one task", func() {
				Expect(actualTasks).To(HaveLen(1))
			})

			It("returns correct task", func() {
				Expect(actualTasks[0]).To(Equal(expectedTask))
			})

			It("returns no error", func() {
				Expect(actualError).To(Not(HaveOccurred()))
			})
		})

		Context("when there are many tasks with the context id", func() {
			expectedTasks := boshclient.BoshTasks{
				{State: boshclient.BoshTaskProcessing, Description: "snapshot deployment", Result: "result-1", ContextID: contextID},
				{State: boshclient.BoshTaskDone, Description: "something finished", Result: "result-1", ContextID: contextID},
				{State: boshclient.BoshTaskProcessing, Description: "snapshot deployment", Result: "result-1", ContextID: contextID},
			}

			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName, contextID).RespondsWithJson(expectedTasks),
					mockbosh.TaskOutput(0).RespondsWith(""),
				)
			})

			JustBeforeEach(func() {
				actualTasks, actualError = c.GetNormalisedTasksByContext(deploymentName, contextID, logger)
			})

			It("returns three tasks", func() {
				Expect(actualTasks).To(HaveLen(3))
			})

			It("returns correct tasks", func() {
				Expect(actualTasks).To(Equal(expectedTasks))
			})

			It("returns no error", func() {
				Expect(actualError).To(Not(HaveOccurred()))
			})
		})

		Context("when an errand task has finished with a non-zero exit code",
			func() {
				expectedTasks := boshclient.BoshTasks{
					{
						ID:          42,
						State:       boshclient.BoshTaskError,
						Description: "errand completed",
						Result:      "result-1",
						ContextID:   contextID,
					},
				}

				BeforeEach(func() {
					director.VerifyAndMock(
						mockbosh.TasksByContext(deploymentName, contextID).RespondsWithATask(
							boshclient.BoshTask{
								ID:          42,
								State:       boshclient.BoshTaskDone,
								Description: "errand completed",
								Result:      "result-1",
								ContextID:   contextID,
							},
						),
						mockbosh.TaskOutput(42).RespondsWithJson(
							boshclient.BoshTaskOutput{ExitCode: 1},
						),
					)
				})

				JustBeforeEach(func() {
					actualTasks, actualError = c.GetNormalisedTasksByContext(deploymentName, contextID, logger)
				})

				It("does not error", func() {
					Expect(actualError).NotTo(HaveOccurred())
				})

				It("returns the correct task", func() {
					Expect(actualTasks).To(Equal(expectedTasks))
				})
			},
		)

		Context("when an errand task result can not be retrived", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName, contextID).RespondsWithATask(
						boshclient.BoshTask{
							ID:          42,
							State:       boshclient.BoshTaskDone,
							Description: "errand completed",
							Result:      "result-1",
							ContextID:   contextID,
						},
					),
					mockbosh.TaskOutput(42).Fails("you are fake news"),
				)
			})

			JustBeforeEach(func() {
				_, actualError = c.GetNormalisedTasksByContext(deploymentName, contextID, logger)
			})

			It("errors", func() {
				Expect(actualError).To(HaveOccurred())
			})
		})
	})
})
