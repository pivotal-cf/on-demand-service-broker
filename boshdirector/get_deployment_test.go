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
	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
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
		JustBeforeEach(func() {
			manifest, deploymentFound, manifestFetchErr = c.GetDeployment(deploymentName, logger)
		})

		Context("when the deployment exists", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName).RespondsWithRawManifest(rawManifest),
				)
			})

			It("calls the authorization header builder", func() {
				Expect(authHeaderBuilder.BuildCallCount()).To(BeNumerically(">", 0))
			})

			It("returns deployment found", func() {
				Expect(deploymentFound).To(BeTrue())
			})

			It("returns the manifest", func() {
				Expect(manifest).To(Equal(rawManifest))
			})

			It("returns no error", func() {
				Expect(manifestFetchErr).NotTo(HaveOccurred())
			})
		})

		Context("when the deployment does not exist", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.GetDeployment(deploymentName).RespondsNotFoundWith(""),
				)
			})

			It("returns deployment not found", func() {
				Expect(deploymentFound).To(BeFalse())
			})

			It("does not returns the manifest", func() {
				Expect(manifest).To(BeNil())
			})

			It("returns no error", func() {
				Expect(manifestFetchErr).NotTo(HaveOccurred())
			})
		})

		Context("when the Authorization header cannot be generated", func() {
			BeforeEach(func() {
				authHeaderBuilder.BuildReturns("", errors.New("some-error"))
			})

			It("returns an error", func() {
				Expect(manifestFetchErr).To(MatchError(ContainSubstring("some-error")))
			})
		})
	})

	Context("when the BOSH director cannot be reached", func() {
		It("returns a bosh request error", func() {
			c, err := boshdirector.New("http://localhost", authorizationheader.NewBasicAuthHeaderBuilder("", ""), false, nil)
			Expect(err).NotTo(HaveOccurred())

			_, deploymentFound, manifestFetchErr = c.GetDeployment(deploymentName, logger)

			Expect(manifestFetchErr).To(MatchError(ContainSubstring("error reaching bosh director")))
			Expect(manifestFetchErr).To(BeAssignableToTypeOf(boshdirector.RequestError{}))
			Expect(deploymentFound).To(BeFalse())
		})
	})
})
