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

var _ = Describe("getting deployment", func() {
	var (
		deploymentName   string = "some-deployment"
		manifest         []byte
		deploymentFound  bool
		rawManifest      = []byte("a-raw-manifest")
		manifestFetchErr error
	)

	Context("when the bosh director can be reached", func() {
		It("returns the manifest when the deployment exists", func() {
			fakeHTTPClient.DoReturns(responseWithRawManifest(rawManifest), nil)
			manifest, deploymentFound, manifestFetchErr = c.GetDeployment(deploymentName, logger)
			Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
				receivedHttpRequest{
					Path:   "/deployments/some-deployment",
					Method: "GET",
				}, 1))

			By("calling the authorization header builder")
			Expect(authHeaderBuilder.AddAuthHeaderCallCount()).To(BeNumerically(">", 0))

			By("returning the manifest")
			Expect(deploymentFound).To(BeTrue())
			Expect(manifest).To(Equal(rawManifest))
			Expect(manifestFetchErr).NotTo(HaveOccurred())
		})

		It("returns a nil manifest and deployment not found when the deployment does not exist", func() {
			fakeHTTPClient.DoReturns(responseWithEmptyBodyAndStatus(http.StatusNotFound), nil)
			manifest, deploymentFound, manifestFetchErr = c.GetDeployment(deploymentName, logger)
			Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
				receivedHttpRequest{
					Path:   "/deployments/some-deployment",
					Method: "GET",
				}, 1))

			By("returning deployment not found")
			Expect(deploymentFound).To(BeFalse())

			By("not returning the manifest")
			Expect(manifest).To(BeNil())

			By("returning no error")
			Expect(manifestFetchErr).NotTo(HaveOccurred())
		})

		It("returns an error when the authorization header cannot be generated", func() {
			authHeaderBuilder.AddAuthHeaderReturns(errors.New("some-error"))
			_, deploymentFound, manifestFetchErr = c.GetDeployment(deploymentName, logger)
			Expect(manifestFetchErr).To(MatchError(ContainSubstring("some-error")))
		})
	})

	Context("when the BOSH director cannot be reached", func() {
		It("returns a bosh request error", func() {
			fakeHTTPClient.DoReturns(nil, errors.New("Unexpected error"))
			_, deploymentFound, manifestFetchErr = c.GetDeployment(deploymentName, logger)

			Expect(manifestFetchErr).To(MatchError(ContainSubstring("error reaching bosh director")))
			Expect(manifestFetchErr).To(BeAssignableToTypeOf(boshdirector.RequestError{}))
			Expect(deploymentFound).To(BeFalse())
		})
	})
})
