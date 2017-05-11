// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/mockbosh"
)

var _ = Describe("deployments", func() {
	Context("gets deployments", func() {
		var (
			actualDeployments      []boshdirector.BoshDeployment
			actualDeploymentsError error
		)

		JustBeforeEach(func() {
			actualDeployments, actualDeploymentsError = c.GetDeployments(logger)
		})

		Context("when bosh fetches the deployments successfully", func() {
			var expectedDeployments []boshdirector.BoshDeployment

			BeforeEach(func() {
				expectedDeployments = []boshdirector.BoshDeployment{
					{Name: "service-instance_one"},
					{Name: "service-instance_two"},
					{Name: "service-instance_three"},
				}
				director.VerifyAndMock(
					mockbosh.Deployments().RespondsOKWithJSON(expectedDeployments),
				)
			})

			It("returns the deployments", func() {
				Expect(actualDeployments).To(Equal(expectedDeployments))
			})

			It("does not error", func() {
				Expect(actualDeploymentsError).NotTo(HaveOccurred())
			})
		})

		Context("when bosh returns a client error (HTTP 404)", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.Deployments().RespondsNotFoundWith(""),
				)
			})

			It("wraps the error", func() {
				Expect(actualDeploymentsError).To(MatchError(ContainSubstring("expected status 200, was 404")))
			})
		})

		Context("when bosh fails to fetch the task", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.Deployments().RespondsInternalServerErrorWith("because reasons"),
				)
			})

			It("wraps the error", func() {
				Expect(actualDeploymentsError).To(MatchError(ContainSubstring("expected status 200, was 500")))
			})
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
			var deployments []boshdirector.BoshDeployment
			Expect(json.Unmarshal(data, &deployments)).To(Succeed())

			Expect(deployments).To(Equal([]boshdirector.BoshDeployment{
				{Name: "service-instance_one"},
				{Name: "service-instance_two"},
			}))
		})
	})
})
