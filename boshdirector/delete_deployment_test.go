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
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
)

var _ = Describe("deleting bosh deployments", func() {
	const deploymentName = "some-deployment"

	var (
		taskID    int
		deleteErr error

		contextID string
	)

	BeforeEach(func() {
		contextID = ""
	})

	JustBeforeEach(func() {
		taskID, deleteErr = c.DeleteDeployment(deploymentName, contextID, logger)
	})

	Context("when the authorization header cannot be generated", func() {
		BeforeEach(func() {
			authHeaderBuilder.AddAuthHeaderReturns(errors.New("some-error"))
		})

		It("returns an error", func() {
			Expect(deleteErr).To(MatchError(ContainSubstring("some-error")))
		})
	})

	Context("when bosh accepts the delete request", func() {
		BeforeEach(func() {
			director.VerifyAndMock(
				mockbosh.DeleteDeployment(deploymentName).WithoutContextID().RedirectsToTask(90),
			)
		})

		It("calls the authorization header builder", func() {
			Expect(authHeaderBuilder.AddAuthHeaderCallCount()).To(BeNumerically(">", 0))
		})

		It("returns the bosh task ID", func() {
			Expect(taskID).To(Equal(90))
		})
	})

	Context("when bosh cannot find the deployment", func() {
		BeforeEach(func() {
			director.VerifyAndMock(
				mockbosh.DeleteDeployment(deploymentName).WithoutContextID().RespondsNotFoundWith(""),
			)
		})

		It("returns an error", func() {
			Expect(deleteErr).To(BeAssignableToTypeOf(boshdirector.DeploymentNotFoundError{}))
		})
	})

	Context("when bosh cannot delete the deployment", func() {
		BeforeEach(func() {
			director.VerifyAndMock(
				mockbosh.DeleteDeployment(deploymentName).WithoutContextID().RespondsInternalServerErrorWith("because reasons"),
			)
		})

		It("returns an error", func() {
			Expect(deleteErr).To(MatchError(ContainSubstring("expected status 302, was 500")))
		})
	})

	Context("when bosh context ID is provided", func() {
		BeforeEach(func() {
			contextID = "some-context-id"
			director.VerifyAndMock(
				mockbosh.DeleteDeployment(deploymentName).WithContextID(contextID).RedirectsToTask(90),
			)
		})

		It("includes the bosh context id header in delete request", func() {
			director.VerifyMocks()
		})
	})
})
