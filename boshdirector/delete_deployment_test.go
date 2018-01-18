// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
)

var _ = Describe("deleting bosh deployments", func() {
	const deploymentName = "some-deployment"
	var (
		fakeDeployment *fakes.FakeBOSHDeployment
		taskReporter   *boshdirector.AsyncTaskReporter
		taskId         = 90
	)

	BeforeEach(func() {
		fakeDeployment = new(fakes.FakeBOSHDeployment)
		fakeDirector.FindDeploymentReturns(fakeDeployment, nil)
		taskReporter = boshdirector.NewAsyncTaskReporter()
		fakeDeployment.DeleteStub = func(force bool) error {
			taskReporter.TaskStarted(taskId)
			return nil
		}
	})

	It("returns the bosh task ID when bosh accepts the delete request", func() {
		taskID, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", logger, taskReporter)

		Expect(deleteErr).NotTo(HaveOccurred())
		Expect(taskID).To(Equal(taskId))
	})

	It("returns an error when cannot find the deployment", func() {
		fakeDirector.FindDeploymentReturns(new(fakes.FakeBOSHDeployment), errors.New("oops"))
		_, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", logger, taskReporter)

		Expect(deleteErr).To(MatchError(ContainSubstring(`BOSH error when deleting deployment "some-deployment"`)))
	})

	It("returns an error when cannot delete the deployment", func() {
		fakeDeployment.DeleteReturns(errors.New("oops"))
		_, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", logger, taskReporter)

		Expect(deleteErr).To(MatchError("Could not delete deployment some-deployment: oops"))
	})
})
