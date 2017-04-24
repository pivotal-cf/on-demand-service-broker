// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshclient_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/mockbosh"
)

var _ = Describe("running errands", func() {
	const (
		deploymentName = "deploymentName"
		errandName     = "errandName"
		contextID      = "some-context-id"
	)

	Context("successfully", func() {
		It("invokes BOSH to queue up an errand", func() {
			taskID := 5
			director.VerifyAndMock(
				mockbosh.Errand(deploymentName, errandName).WithContextID(contextID).RedirectsToTask(taskID),
			)

			actualTaskID, actualErr := c.RunErrand(deploymentName, errandName, contextID, logger)
			Expect(actualTaskID).To(Equal(taskID))
			Expect(actualErr).NotTo(HaveOccurred())
		})
	})

	Context("has an error", func() {
		It("invokes BOSH to queue up an errand", func() {
			director.VerifyAndMock(
				mockbosh.Errand(deploymentName, errandName).WithAnyContextID().Fails("because reasons"),
			)

			_, actualErr := c.RunErrand(deploymentName, errandName, contextID, logger)
			Expect(actualErr).To(HaveOccurred())
		})
	})
})
