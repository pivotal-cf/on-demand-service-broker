// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

var _ = Describe("deleting bosh deployments", func() {
	const deploymentName = "some-deployment"

	It("returns an error when the authorization header cannot be generated", func() {
		authHeaderBuilder.AddAuthHeaderReturns(errors.New("some-error"))
		_, deleteErr := c.DeleteDeployment(deploymentName, "", logger)
		Expect(deleteErr).To(MatchError(ContainSubstring("some-error")))
	})

	It("returns the bosh task ID when bosh accepts the delete request", func() {
		fakeHTTPClient.DoReturns(responseWithRedirectToTaskID(90), nil)
		taskID, deleteErr := c.DeleteDeployment(deploymentName, "", logger)
		Expect(deleteErr).NotTo(HaveOccurred())
		Expect(taskID).To(Equal(90))

		By("calling the appropriate endpoint")
		Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
			receivedHttpRequest{
				Path:   "/deployments/some-deployment",
				Method: "DELETE",
			}, 0))
		Expect(authHeaderBuilder.AddAuthHeaderCallCount()).To(BeNumerically(">", 0))
	})

	It("returns an error when bosh cannot find the deployment", func() {
		fakeHTTPClient.DoReturns(responseWithEmptyBodyAndStatus(http.StatusNotFound), nil)
		_, deleteErr := c.DeleteDeployment(deploymentName, "", logger)
		Expect(deleteErr).To(BeAssignableToTypeOf(boshdirector.DeploymentNotFoundError{}))
	})

	It("returns an error when bosh cannot delete the deployment", func() {
		fakeHTTPClient.DoReturns(responseWithEmptyBodyAndStatus(http.StatusInternalServerError), nil)
		_, deleteErr := c.DeleteDeployment(deploymentName, "", logger)
		Expect(deleteErr).To(MatchError(ContainSubstring("expected status 302, was 500")))
	})

	It("includes the bosh context id header in delete request when bosh context ID is provided", func() {
		fakeHTTPClient.DoReturns(responseWithRedirectToTaskID(90), nil)
		taskID, deleteErr := c.DeleteDeployment(deploymentName, "some-context-id", logger)
		Expect(deleteErr).NotTo(HaveOccurred())
		Expect(taskID).To(Equal(90))

		Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
			receivedHttpRequest{
				Path:   "/deployments/some-deployment",
				Method: "DELETE",
				Header: http.Header{
					"X-Bosh-Context-Id": []string{"some-context-id"},
				},
			}, 0))
	})
})
