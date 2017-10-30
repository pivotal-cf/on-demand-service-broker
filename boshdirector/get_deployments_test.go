// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"encoding/json"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

var _ = Describe("deployments", func() {
	Context("gets deployments", func() {
		var (
			actualDeployments      []boshdirector.Deployment
			actualDeploymentsError error
		)

		It("returns the deployments when bosh fetches the deployments successfully", func() {
			expectedDeployments := []boshdirector.Deployment{
				{Name: "service-instance_one"},
				{Name: "service-instance_two"},
				{Name: "service-instance_three"},
			}
			fakeHTTPClient.DoReturns(responseOKWithJSON(expectedDeployments), nil)
			actualDeployments, actualDeploymentsError = c.GetDeployments(logger)
			Expect(actualDeployments).To(Equal(expectedDeployments))
			Expect(actualDeploymentsError).NotTo(HaveOccurred())

			By("calling the appropriate endpoints")
			Expect(fakeHTTPClient).To(HaveReceivedHttpRequestAtIndex(
				receivedHttpRequest{
					Path:   "/deployments",
					Method: "GET",
				}, 1))
		})

		It("wraps the error when bosh returns a client error (HTTP 404)", func() {
			fakeHTTPClient.DoReturns(responseWithEmptyBodyAndStatus(http.StatusNotFound), nil)
			_, actualDeploymentsError = c.GetDeployments(logger)

			Expect(actualDeploymentsError).To(MatchError(ContainSubstring("expected status 200, was 404")))
		})

		It("wraps the error when when bosh fails to fetch the task", func() {
			fakeHTTPClient.DoReturns(responseWithEmptyBodyAndStatus(http.StatusInternalServerError), nil)
			_, actualDeploymentsError = c.GetDeployments(logger)

			Expect(actualDeploymentsError).To(MatchError(ContainSubstring("expected status 200, was 500")))
		})
	})

	Context("deserialization", func() {
		It("unmarshals deployments response", func() {
			data := []byte(`[
			  {
			    "name": "service-instance_one",
			    "cloud_config": "latest",
			    "releases": [
			      {
			        "name": "one",
			        "version": "42"
			      }
			    ],
			    "stemcells": [
			      {
			        "name": "bosh-warden-boshlite-ubuntu-trusty-go_agent",
			        "version": "3312.7"
			      }
			    ]
			  },
			  {
			    "name": "service-instance_two",
			    "cloud_config": "latest",
			    "releases": [
			      {
			        "name": "two",
			        "version": "101"
			      }
			    ],
			    "stemcells": [
			      {
			        "name": "bosh-warden-boshlite-ubuntu-trusty-go_agent",
			        "version": "3312.7"
			      }
			    ]
			  }
			]`)
			var deployments []boshdirector.Deployment
			Expect(json.Unmarshal(data, &deployments)).To(Succeed())

			Expect(deployments).To(Equal([]boshdirector.Deployment{
				{Name: "service-instance_one"},
				{Name: "service-instance_two"},
			}))
		})
	})
})
