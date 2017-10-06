// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
)

var _ = Describe("running errands", func() {
	var (
		deploymentName  = "deploymentName"
		errandName      = "errandName"
		contextID       = "some-context-id"
	)

	Context("successfully", func() {
		It("invokes BOSH to queue up an errand", func() {
			taskID := 5
			director.VerifyAndMock(
				mockbosh.Errand(deploymentName, errandName, `{}`).WithContextID(contextID).RedirectsToTask(taskID),
			)

			actualTaskID, actualErr := c.RunErrand(deploymentName, errandName, nil, contextID, logger)
			Expect(actualTaskID).To(Equal(taskID))
			Expect(actualErr).NotTo(HaveOccurred())
		})
	})

	Context("successfully with instances", func() {
		It("invokes BOSH to queue up an errand with instances", func() {
			taskID := 5
			errandInstances := []string{"errand_instance/4529480d-9770-4c32-b9bb-d936c0a908ca"}
			expectedBody := `{
			"instances":[
			  {
			    "group": "errand_instance",
			    "id": "4529480d-9770-4c32-b9bb-d936c0a908ca"
			  }
			]}`

			director.VerifyAndMock(
				mockbosh.Errand(deploymentName, errandName, expectedBody).WithContextID(contextID).RedirectsToTask(taskID),
			)

			actualTaskID, actualErr := c.RunErrand(deploymentName, errandName, errandInstances, contextID, logger)
			Expect(actualTaskID).To(Equal(taskID))
			Expect(actualErr).NotTo(HaveOccurred())
		})

		It("invokes BOSH to queue up an errand with instances", func() {
			taskID := 5
			errandInstances := []string{"errand_instance"}
			expectedBody := `{
			"instances":[
			  {
			    "group": "errand_instance"
			  }
			]}`

			director.VerifyAndMock(
				mockbosh.Errand(deploymentName, errandName, expectedBody).WithContextID(contextID).RedirectsToTask(taskID),
			)

			actualTaskID, actualErr := c.RunErrand(deploymentName, errandName, errandInstances, contextID, logger)
			Expect(actualTaskID).To(Equal(taskID))
			Expect(actualErr).NotTo(HaveOccurred())
		})
	})

	Context("has an error", func() {
		It("returns an error when the errandInstance names are invalid", func() {
			errandInstances := []string{"some/invalid/errand"}

			_, actualErr := c.RunErrand(deploymentName, errandName, errandInstances, contextID, logger)
			Expect(actualErr).To(HaveOccurred())
		})


		It("invokes BOSH to queue up an errand", func() {
			director.VerifyAndMock(
				mockbosh.Errand(deploymentName, errandName, `{}`).WithAnyContextID().RespondsInternalServerErrorWith("because reasons"),
			)

			_, actualErr := c.RunErrand(deploymentName, errandName, nil, contextID, logger)
			Expect(actualErr).To(HaveOccurred())
		})
	})
})
