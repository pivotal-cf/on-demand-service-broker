// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"errors"

	"github.com/cloudfoundry/bosh-cli/v7/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
)

var _ = Describe("deleting bosh deployments", func() {
	const deploymentName = "some-deployment"
	var (
		fakeDeployment         *fakes.FakeBOSHDeployment
		fakeDeploymentResponse director.DeploymentResp
		taskReporter           *boshdirector.AsyncTaskReporter
		taskId                 = 90
	)

	BeforeEach(func() {
		fakeDeploymentResponse = director.DeploymentResp{
			Name: deploymentName,
		}

		fakeDeployment = new(fakes.FakeBOSHDeployment)
		fakeDirector.ListDeploymentsReturns([]director.DeploymentResp{fakeDeploymentResponse}, nil)
		fakeDirector.FindDeploymentReturns(fakeDeployment, nil)
		fakeDirector.WithContextReturns(fakeDirector)
		taskReporter = boshdirector.NewAsyncTaskReporter()
		fakeDeployment.DeleteStub = func(force bool) error {
			taskReporter.TaskStarted(taskId)
			return nil
		}
	})

	It("returns the bosh task ID when bosh accepts the delete request", func() {
		taskID, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", false, taskReporter, logger)

		Expect(deleteErr).NotTo(HaveOccurred())
		Expect(taskID).To(Equal(taskId))
	})

	It("returns an error when the FindDeployment errors", func() {
		fakeDirector.FindDeploymentReturns(new(fakes.FakeBOSHDeployment), errors.New("oops"))
		_, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", false, taskReporter, logger)

		Expect(deleteErr).To(MatchError(ContainSubstring(`BOSH error when deleting deployment "some-deployment"`)))
	})

	It("returns an error when the GetDeployment errors", func() {
		fakeDirector.ListDeploymentsReturns(nil, errors.New("oops"))
		_, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", false, taskReporter, logger)

		Expect(deleteErr).To(MatchError(ContainSubstring(`BOSH error when deleting deployment "some-deployment"`)))
	})

	It("reports task started and task finished when the deployment doesn't exist", func() {
		fakeDirector.ListDeploymentsReturns([]director.DeploymentResp{}, nil)
		taskID, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", false, taskReporter, logger)

		Expect(taskID).To(Equal(0))
		Expect(deleteErr).NotTo(HaveOccurred())

		Eventually(taskReporter.Task).Should(Receive())
		Eventually(taskReporter.Finished).Should(Receive())
	})

	It("returns an error when cannot delete the deployment", func() {
		fakeDeployment.DeleteReturns(errors.New("oops"))
		_, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", false, taskReporter, logger)

		Expect(deleteErr).To(MatchError("Could not delete deployment some-deployment: oops"))
	})

	When("force flag is passed to Delete deployment", func() {
		It("passes true when true is passed", func() {
			expectedForceDelete := true
			_, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", expectedForceDelete, taskReporter, logger)

			Expect(deleteErr).NotTo(HaveOccurred())
			actualForceDelete := fakeDeployment.DeleteArgsForCall(0)
			Expect(actualForceDelete).To(Equal(expectedForceDelete))
		})

		It("passes false when false is passed", func() {
			expectedForceDelete := false
			_, deleteErr := c.DeleteDeployment(deploymentName, "delete-some-deployment", expectedForceDelete, taskReporter, logger)

			Expect(deleteErr).NotTo(HaveOccurred())
			actualForceDelete := fakeDeployment.DeleteArgsForCall(0)
			Expect(actualForceDelete).To(Equal(expectedForceDelete))
		})
	})
})
