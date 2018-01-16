// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"errors"

	"github.com/cloudfoundry/bosh-cli/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
)

func getManifest() []byte {
	return []byte(`
---
name: bill
`)
}

func namelessManifest() []byte {
	return []byte(`
---
`)
}

var _ = Describe("deploying a manifest", func() {
	//const deploymentName = "some-deployment"
	var (
		fakeDeployment    *fakes.FakeBOSHDeployment
		manifest          []byte
		asyncTaskReporter *boshdirector.AsyncTaskReporter
		taskId            = 90
	)
	BeforeEach(func() {
		manifest = getManifest()
		fakeDeployment = new(fakes.FakeBOSHDeployment)
		fakeDirector.WithContextReturns(fakeDirector)
		fakeDirector.FindDeploymentReturns(fakeDeployment, nil)
		asyncTaskReporter = boshdirector.NewAsyncTaskReporter()
		fakeDeployment.UpdateStub = func(manifest []byte, opts director.UpdateOpts) error {
			asyncTaskReporter.TaskStarted(taskId)
			return nil
		}
	})

	It("succeeds", func() {
		taskID, err := c.Deploy(manifest, "some-context-id", logger, asyncTaskReporter)

		By("returning the correct task id")
		Expect(err).NotTo(HaveOccurred())
		Expect(taskID).To(Equal(taskId))
	})

	It("returns an error if cannot update the deployment", func() {
		fakeDeployment.UpdateReturns(errors.New("oops"))
		_, err := c.Deploy(manifest, "some-context-id", logger, asyncTaskReporter)

		Expect(err).To(MatchError(ContainSubstring("Could not update deployment bill")))
	})

	It("returns an error if cannot fetch the deployment name from the manifest", func() {
		_, err := c.Deploy(namelessManifest(), "some-context-id", logger, asyncTaskReporter)

		Expect(err).To(MatchError(ContainSubstring("Error fetching deployment name")))
	})

	It("returns an error if the manifest is invalid", func() {
		_, err := c.Deploy([]byte("not-yaml"), "some-context-id", logger, asyncTaskReporter)

		Expect(err).To(MatchError(ContainSubstring("Error fetching deployment name")))
	})

	It("returns an error if cannot get the deployment", func() {
		fakeDirector.FindDeploymentReturns(nil, errors.New("oops"))
		_, err := c.Deploy(manifest, "some-context-id", logger, asyncTaskReporter)

		Expect(err).To(MatchError(ContainSubstring("BOSH CLI error")))
	})
})
