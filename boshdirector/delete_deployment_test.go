// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"errors"
	"fmt"

	"github.com/cloudfoundry/bosh-cli/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
)

var _ = Describe("deleting bosh deployments", func() {
	const deploymentName = "some-deployment"
	var (
		fakeDeployment *fakes.FakeBOSHDeployment
		fakeTask       *fakes.FakeTask
	)
	BeforeEach(func() {
		fakeDeployment = new(fakes.FakeBOSHDeployment)
		fakeTask = new(fakes.FakeTask)
		fakeTask.IDReturns(90)
		fakeDirector.FindDeploymentReturns(fakeDeployment, nil)
		fakeDirector.RecentTasksStub = func(limit int, filter director.TasksFilter) ([]director.Task, error) {
			if filter.Deployment == deploymentName {
				return []director.Task{fakeTask}, nil
			}
			return []director.Task{}, nil
		}
	})

	It("returns the bosh task ID when bosh accepts the delete request", func() {
		taskID, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", logger)

		By("querying the latest task for the deployment")
		limit, tasksFilter := fakeDirector.RecentTasksArgsForCall(0)
		Expect(limit).To(Equal(1))
		Expect(tasksFilter.Deployment).To(Equal(deploymentName))

		Expect(deleteErr).NotTo(HaveOccurred())
		Expect(taskID).To(Equal(90))
	})

	It("succeeds when no task is found", func() {
		fakeDirector.RecentTasksReturns([]director.Task{}, nil)
		_, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", logger)

		Expect(deleteErr).NotTo(HaveOccurred())
	})

	It("returns an error when cannot find the deployment", func() {
		fakeDirector.FindDeploymentReturns(new(fakes.FakeBOSHDeployment), errors.New("oops"))
		_, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", logger)

		Expect(deleteErr).To(MatchError(ContainSubstring(`BOSH error when deleting deployment "some-deployment"`)))
	})

	It("returns an error when cannot delete the deployment", func() {
		fakeDeployment.DeleteReturns(errors.New("oops"))
		_, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", logger)

		Expect(deleteErr).To(MatchError("Could not delete deployment some-deployment: oops"))
	})

	It("returns an error when cannot find the task ID", func() {
		fakeDirector.RecentTasksReturns(nil, errors.New("oops tasks"))
		_, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", logger)

		Expect(deleteErr).To(MatchError(ContainSubstring(fmt.Sprintf(`Could not find tasks for deployment "%s"`, deploymentName))))
	})
})
