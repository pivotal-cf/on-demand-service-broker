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
		fakeDeployment *fakes.FakeBOSHDeployment
		fakeTask       *fakes.FakeTask
		manifest       []byte
	)
	BeforeEach(func() {
		manifest = getManifest()
		fakeDeployment = new(fakes.FakeBOSHDeployment)
		fakeTask = new(fakes.FakeTask)
		fakeTask.IDReturns(90)
		fakeDirector.WithContextReturns(fakeDirector)
		fakeDirector.FindDeploymentReturns(fakeDeployment, nil)
		fakeDirector.RecentTasksStub = func(limit int, filter director.TasksFilter) ([]director.Task, error) {
			if filter.Deployment == "bill" {
				return []director.Task{fakeTask}, nil
			}
			return []director.Task{}, nil
		}
	})

	It("succeeds", func() {
		taskID, err := c.Deploy(manifest, "some-context-id", logger)

		By("querying the latest task for the deployment")
		limit, tasksFilter := fakeDirector.RecentTasksArgsForCall(0)
		Expect(limit).To(Equal(1))
		Expect(tasksFilter.Deployment).To(Equal("bill"))

		By("returning the correct task id")
		Expect(err).NotTo(HaveOccurred())
		Expect(taskID).To(Equal(fakeTask.ID()))
	})

	It("succeeds when no task is found", func() {
		fakeDirector.RecentTasksReturns([]director.Task{}, nil)
		_, deleteErr := c.Deploy(manifest, "delete-some-deployment", logger)

		Expect(deleteErr).NotTo(HaveOccurred())
	})

	It("returns an error if cannot update the deployment", func() {
		fakeDeployment.UpdateReturns(errors.New("oops"))
		_, err := c.Deploy(manifest, "some-context-id", logger)

		Expect(err).To(MatchError(ContainSubstring("Could not update deployment bill")))
	})

	It("returns an error if cannot fetch the deployment name from the manifest", func() {
		fakeDeployment.UpdateReturns(errors.New("oops"))
		_, err := c.Deploy(namelessManifest(), "some-context-id", logger)

		Expect(err).To(MatchError(ContainSubstring("Error fetching deployment name")))
	})

	It("returns an error if the manifest is invalid", func() {
		fakeDeployment.UpdateReturns(errors.New("oops"))
		_, err := c.Deploy([]byte("not-yaml"), "some-context-id", logger)

		Expect(err).To(MatchError(ContainSubstring("Error fetching deployment name")))
	})

	It("returns an error if cannot get the deployment", func() {
		fakeDirector.FindDeploymentReturns(nil, errors.New("oops"))
		_, err := c.Deploy(manifest, "some-context-id", logger)

		Expect(err).To(MatchError(ContainSubstring("BOSH CLI error")))
	})

	It("returns an error if cannot get the recent tasks", func() {
		fakeDirector.RecentTasksReturns(nil, errors.New("boom"))
		_, err := c.Deploy(manifest, "some-context-id", logger)

		Expect(err).To(MatchError(ContainSubstring(`Could not find tasks for deployment "bill"`)))
	})
})
