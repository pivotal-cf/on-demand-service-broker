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
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"gopkg.in/yaml.v2"
)

var _ = Describe("deploying a manifest", func() {
	var (
		deploymentManifest = bosh.BoshManifest{
			Name:           "big deployment",
			Releases:       []bosh.Release{},
			Stemcells:      []bosh.Stemcell{},
			InstanceGroups: []bosh.InstanceGroup{},
		}
		deploymentManifestBytes []byte
	)

	BeforeEach(func() {
		var err error
		deploymentManifestBytes, err = yaml.Marshal(deploymentManifest)
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns a BOSH task ID when there is no bosh context id", func() {
		fakeHTTPClient.DoReturns(responseWithRedirectToTaskID(90), nil)
		taskID, err := c.Deploy(deploymentManifestBytes, "", logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(taskID).To(Equal(90))

		By("calling the appropriate endpoint")
		Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(receivedHttpRequest{Path: "/deployments", Method: "POST"}, 1))

		By("using authentication")
		Expect(authHeaderBuilder.AddAuthHeaderCallCount()).To(BeNumerically(">", 0))
	})

	It("returns a BOSH task ID when there is bosh context id", func() {
		fakeHTTPClient.DoReturns(responseWithRedirectToTaskID(90), nil)
		taskID, err := c.Deploy(deploymentManifestBytes, "some-context-id", logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(taskID).To(Equal(90))

		By("calling the appropriate endpoint")
		Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(receivedHttpRequest{
			Path:   "/deployments",
			Method: "POST",
			Header: http.Header{
				"X-Bosh-Context-Id": []string{"some-context-id"},
			},
		}, 1))

		By("using authentication")
		Expect(authHeaderBuilder.AddAuthHeaderCallCount()).To(BeNumerically(">", 0))
	})

	It("returns an error when the authorization header cannot be generated", func() {
		authHeaderBuilder.AddAuthHeaderReturns(errors.New("some-error"))
		_, err := c.Deploy(deploymentManifestBytes, "", logger)
		Expect(err).To(MatchError(ContainSubstring("some-error")))
	})

	It("returns an error when the BOSH director cannot be reached", func() {
		fakeHTTPClient.DoReturns(nil, errors.New("Unexpected error"))
		_, err := c.Deploy(deploymentManifestBytes, "", logger)

		Expect(err).To(MatchError(ContainSubstring("error reaching bosh director")))
		Expect(err).To(BeAssignableToTypeOf(boshdirector.RequestError{}))
	})

	It("returns an error when the bosh director responds with an error", func() {
		fakeHTTPClient.DoReturns(responseWithEmptyBodyAndStatus(http.StatusInternalServerError), nil)
		_, err := c.Deploy(deploymentManifestBytes, "", logger)
		Expect(err).To(MatchError(ContainSubstring("expected status 302, was 500")))
	})

})
