// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"errors"
	"fmt"

	"github.com/cloudfoundry/bosh-cli/v7/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
)

var _ = Describe("getting tasks", func() {
	var (
		deploymentName           = "an-amazing-deployment"
		processingTask, doneTask *fakes.FakeTask
		expectedTasks            boshdirector.BoshTasks
	)

	BeforeEach(func() {
		processingTask = new(fakes.FakeTask)
		processingTask.IDReturns(1)
		processingTask.StateReturns("processing")
		processingTask.DescriptionReturns("snapshot deployment")
		processingTask.ResultReturns("result-1")
		processingTask.ContextIDReturns("")
		processingTask.DeploymentNameReturns(deploymentName)

		doneTask = new(fakes.FakeTask)
		doneTask.IDReturns(2)
		doneTask.StateReturns("done")
		doneTask.DescriptionReturns("snapshot deployment")
		doneTask.ResultReturns("result-2")
		doneTask.ContextIDReturns("some-context")
		doneTask.DeploymentNameReturns(deploymentName)

		fakeDirector.RecentTasksReturns([]director.Task{processingTask, doneTask}, nil)

		fakeDirector.FindTaskReturns(processingTask, nil)
		processingTask.ResultOutputStub = func(r director.TaskReporter) error {
			r.TaskOutputChunk(processingTask.ID(), []byte(`{"foo":0,"other":"foo","err":"bar"}`))
			return nil
		}

		expectedTasks = boshdirector.BoshTasks{
			{ID: 1, State: boshdirector.TaskProcessing, Description: "snapshot deployment", Result: "result-1", ContextID: ""},
			{ID: 2, State: boshdirector.TaskDone, Description: "snapshot deployment", Result: "result-2", ContextID: "some-context"},
		}
	})

	Describe("GetTasksInProgress", func() {
		BeforeEach(func() {
			fakeDirector.CurrentTasksReturns([]director.Task{processingTask}, nil)
			expectedTasks = boshdirector.BoshTasks{
				{ID: 1, State: boshdirector.TaskProcessing, Description: "snapshot deployment", Result: "result-1", ContextID: ""},
			}
		})

		It("returns the tasks", func() {
			actualTasks, err := c.GetTasksInProgress(deploymentName, logger)
			Expect(err).NotTo(HaveOccurred())

			By("fetching all tasks")
			taskFilter := fakeDirector.CurrentTasksArgsForCall(0)
			Expect(taskFilter).To(Equal(director.TasksFilter{All: false, Deployment: deploymentName}))

			By("returning the expected tasks structure")
			Expect(actualTasks).To(Equal(expectedTasks))
		})

		It("wraps the error when fetching current tasks fails", func() {
			fakeDirector.CurrentTasksReturns([]director.Task{}, errors.New("boom"))

			_, err := c.GetTasksInProgress(deploymentName, logger)
			Expect(err).To(MatchError(fmt.Sprintf("Could not fetch current tasks for deployment %s: boom", deploymentName)))
		})
	})

	Describe("GetTasksByContextID", func() {
		var (
			multipleTaskContextID = "multiple-context-id"
			singleTaskContextID   = "some-context-id"
			errandTaskContextID   = "errand-task-context-id"

			errandTask *fakes.FakeTask
		)

		BeforeEach(func() {
			errandTask = new(fakes.FakeTask)
			errandTask.IDReturns(42)
			errandTask.StateReturns("done")
			errandTask.DescriptionReturns("errand completed")
			errandTask.ResultReturns("result-1")
			errandTask.ContextIDReturns("some-context")
			errandTask.DeploymentNameReturns(deploymentName)

			fakeDirector.FindTasksByContextIdStub = func(contextID string) ([]director.Task, error) {
				processingTask.ContextIDReturns(contextID)
				doneTask.ContextIDReturns(contextID)
				errandTask.ContextIDReturns(contextID)

				switch contextID {
				case multipleTaskContextID:
					return []director.Task{processingTask, doneTask}, nil
				case singleTaskContextID:
					return []director.Task{processingTask}, nil
				case errandTaskContextID:
					return []director.Task{errandTask}, nil
				default:
					return []director.Task{}, nil
				}
			}
		})

		It("returns no tasks when there are no tasks with the context id", func() {
			actualTasks, err := c.GetNormalisedTasksByContext(deploymentName, "some-id", logger)
			Expect(actualTasks).To(HaveLen(0))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns one task when there is one task with the context id", func() {
			processingTask.ContextIDReturns(singleTaskContextID)
			expectedTask := boshdirector.BoshTask{ID: 1, State: boshdirector.TaskProcessing, Description: "snapshot deployment", Result: "result-1", ContextID: singleTaskContextID}

			actualTasks, err := c.GetNormalisedTasksByContext(deploymentName, singleTaskContextID, logger)
			Expect(actualTasks).To(HaveLen(1))
			Expect(actualTasks[0]).To(Equal(expectedTask))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the tasks only for the deployment", func() {
			otherDeploymentTask := new(fakes.FakeTask)
			otherDeploymentTask.IDReturns(3)
			otherDeploymentTask.StateReturns("done")
			otherDeploymentTask.DescriptionReturns("snapshot deployment")
			otherDeploymentTask.ResultReturns("result-2")
			otherDeploymentTask.ContextIDReturns("some-context")
			otherDeploymentTask.DeploymentNameReturns("other-deployment")
			processingTask.ContextIDReturns("some-context")

			fakeDirector.FindTasksByContextIdReturns([]director.Task{processingTask, doneTask, otherDeploymentTask}, nil)

			actualTasks, err := c.GetNormalisedTasksByContext(deploymentName, "some-context", logger)

			Expect(err).NotTo(HaveOccurred())
			Expect(actualTasks).To(Equal(expectedTasksWithID("some-context")))
		})

		It("returns the correct tasks when there are many tasks with the context id", func() {
			actualTasks, err := c.GetNormalisedTasksByContext(deploymentName, multipleTaskContextID, logger)
			Expect(actualTasks).To(HaveLen(2))
			Expect(actualTasks).To(Equal(expectedTasksWithID(multipleTaskContextID)))
			Expect(err).To(Not(HaveOccurred()))
		})

		It("wraps the error when it fails to fetch the tasks", func() {
			fakeDirector.FindTasksByContextIdReturns(nil, errors.New("some error"))

			_, err := c.GetNormalisedTasksByContext(deploymentName, "context-id", logger)
			Expect(err).To(MatchError(fmt.Sprintf("Could not fetch tasks for deployment %s with context id context-id: some error", deploymentName)))
		})

		It("returns the correct state when an errand task has finished with a non-zero exit code", func() {
			fakeDirector.FindTaskReturns(errandTask, nil)
			errandTask.ResultOutputStub = func(r director.TaskReporter) error {
				r.TaskOutputChunk(errandTask.ID(), []byte(`{"exit_code":1,"stdout":"foo","stderr":"bar"}`))
				r.TaskFinished(errandTask.ID(), "status")
				return nil
			}

			expectedTask := boshdirector.BoshTask{
				ID:          42,
				State:       boshdirector.TaskError,
				Description: "errand completed",
				Result:      "result-1",
				ContextID:   errandTaskContextID,
			}

			actualTasks, err := c.GetNormalisedTasksByContext(deploymentName, errandTaskContextID, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualTasks).To(Equal(boshdirector.BoshTasks{expectedTask}))
		})

		It("returns an error when the task output can not be retrived", func() {
			fakeDirector.FindTaskReturns(errandTask, nil)
			errandTask.ResultOutputReturns(errors.New("some problem"))

			_, err := c.GetNormalisedTasksByContext(deploymentName, errandTaskContextID, logger)
			Expect(err).To(MatchError(ContainSubstring("Could not retrieve task output")))
		})
	})
})

func expectedTasksWithID(contextID string) boshdirector.BoshTasks {
	return boshdirector.BoshTasks{
		{ID: 1, State: boshdirector.TaskProcessing, Description: "snapshot deployment", Result: "result-1", ContextID: contextID},
		{ID: 2, State: boshdirector.TaskDone, Description: "snapshot deployment", Result: "result-2", ContextID: contextID},
	}
}
