// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"fmt"

	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("running errands", func() {
	var (
		deploymentName = "deploymentName"
		errandName     = "errandName"
		contextID      = "some-context-id"
	)

	It("invokes BOSH to queue up an errand", func() {
		fakeHTTPClient.DoReturns(responseWithRedirectToTaskID(5), nil)

		actualTaskID, actualErr := c.RunErrand(deploymentName, errandName, nil, contextID, logger)
		Expect(actualTaskID).To(Equal(5))
		Expect(actualErr).NotTo(HaveOccurred())

		By("calling the correct endpoint")
		Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
			receivedHttpRequest{
				Path:   fmt.Sprintf("/deployments/%s/errands/%s/runs", deploymentName, errandName),
				Method: "POST",
			}, 1))
	})

	It("invokes BOSH to queue up an errand with instances with group and ID when a specific instance is configured", func() {
		fakeHTTPClient.DoReturns(responseWithRedirectToTaskID(5), nil)

		errandInstances := []string{"errand_instance/4529480d-9770-4c32-b9bb-d936c0a908ca"}
		actualTaskID, actualErr := c.RunErrand(deploymentName, errandName, errandInstances, contextID, logger)
		Expect(actualTaskID).To(Equal(5))
		Expect(actualErr).NotTo(HaveOccurred())

		By("calling the correct endpoint")
		Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
			receivedHttpRequest{
				Path:   fmt.Sprintf("/deployments/%s/errands/%s/runs", deploymentName, errandName),
				Method: "POST",
			}, 1))

	})

	It("invokes BOSH to queue up an errand with instances with group only when an instance group is configured", func() {
		fakeHTTPClient.DoReturns(responseWithRedirectToTaskID(5), nil)

		errandInstances := []string{"errand_instance"}
		actualTaskID, actualErr := c.RunErrand(deploymentName, errandName, errandInstances, contextID, logger)
		Expect(actualTaskID).To(Equal(5))
		Expect(actualErr).NotTo(HaveOccurred())

		By("calling the correct endpoint")
		Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
			receivedHttpRequest{
				Path:   fmt.Sprintf("/deployments/%s/errands/%s/runs", deploymentName, errandName),
				Method: "POST",
			}, 1))
	})

	It("returns an error when the errandInstance names are invalid", func() {
		errandInstances := []string{"some/invalid/errand"}

		_, actualErr := c.RunErrand(deploymentName, errandName, errandInstances, contextID, logger)
		Expect(actualErr).To(HaveOccurred())
	})

	It("returns the error when when bosh fails to queue up an errand", func() {
		fakeHTTPClient.DoReturns(responseWithEmptyBodyAndStatus(http.StatusInternalServerError), nil)
		_, err := c.RunErrand(deploymentName, errandName, nil, contextID, logger)

		Expect(err).To(MatchError(ContainSubstring("expected status 302, was 500")))
	})
})
