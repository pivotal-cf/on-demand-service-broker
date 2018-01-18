// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"errors"
	"time"

	"github.com/cloudfoundry/bosh-cli/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
)

var _ = Describe("running errands", func() {
	var (
		deploymentName = "deploymentName"
		errandName     = "errandName"
		contextID      = "some-context-id"
		fakeDeployment *fakes.FakeBOSHDeployment
		taskReporter   *boshdirector.AsyncTaskReporter
		taskId         = 5
		startTime      time.Time
		endTime        time.Time
	)

	BeforeEach(func() {
		taskReporter = boshdirector.NewAsyncTaskReporter()

		fakeDeployment = new(fakes.FakeBOSHDeployment)
		fakeDeployment.RunErrandStub = func(errandName string, a, b bool, instances []director.InstanceGroupOrInstanceSlug) ([]director.ErrandResult, error) {
			taskReporter.TaskStarted(taskId)
			taskReporter.TaskFinished(taskId, "done")
			return []director.ErrandResult{}, nil
		}

		fakeDirector.FindDeploymentReturns(fakeDeployment, nil)
	})

	It("invokes BOSH to queue up an errand", func() {
		actualTaskID, actualErr := c.RunErrand(deploymentName, errandName, nil, contextID, logger, taskReporter)
		Expect(actualTaskID).To(Equal(taskId))
		Expect(actualErr).NotTo(HaveOccurred())

		Expect(fakeDirector.FindDeploymentArgsForCall(0)).To(Equal(deploymentName))

		name, keepAlive, whenChanged, instances := fakeDeployment.RunErrandArgsForCall(0)
		Expect(name).To(Equal(errandName))
		Expect(keepAlive).To(BeFalse())
		Expect(whenChanged).To(BeFalse())
		Expect(instances).To(BeNil())
	})

	It("blocks until the errand is finished", func() {
		fakeDeployment.RunErrandStub = func(errandName string, a, b bool, instances []director.InstanceGroupOrInstanceSlug) ([]director.ErrandResult, error) {
			taskReporter.TaskStarted(taskId)

			time.Sleep(100 * time.Millisecond)

			taskReporter.TaskFinished(taskId, "done")
			return []director.ErrandResult{}, nil
		}

		startTime = time.Now()
		actualTaskID, actualErr := c.RunErrand(deploymentName, errandName, nil, contextID, logger, taskReporter)
		endTime = time.Now()

		Expect(actualTaskID).To(Equal(taskId))
		Expect(actualErr).NotTo(HaveOccurred())
		Expect(endTime.Sub(startTime)).To(BeNumerically(">", time.Millisecond*100))

	})

	It("invokes BOSH to queue up an errand with instances with group and ID when a specific instance is configured", func() {
		errandInstances := []string{"errand_instance/4529480d-9770-4c32-b9bb-d936c0a908ca"}
		actualTaskID, actualErr := c.RunErrand(deploymentName, errandName, errandInstances, contextID, logger, taskReporter)
		Expect(actualTaskID).To(Equal(taskId))
		Expect(actualErr).NotTo(HaveOccurred())

		name, keepAlive, whenChanged, instances := fakeDeployment.RunErrandArgsForCall(0)
		Expect(name).To(Equal(errandName))
		Expect(keepAlive).To(BeFalse())
		Expect(whenChanged).To(BeFalse())
		Expect(instances).To(HaveLen(1))
		Expect(instances[0].Name()).To(Equal("errand_instance"))
		Expect(instances[0].IndexOrID()).To(Equal("4529480d-9770-4c32-b9bb-d936c0a908ca"))

	})

	It("invokes BOSH to queue up an errand with instances with group only when an instance group is configured", func() {
		errandInstances := []string{"errand_instance"}
		actualTaskID, actualErr := c.RunErrand(deploymentName, errandName, errandInstances, contextID, logger, taskReporter)
		Expect(actualTaskID).To(Equal(taskId))
		Expect(actualErr).NotTo(HaveOccurred())

		name, keepAlive, whenChanged, instances := fakeDeployment.RunErrandArgsForCall(0)
		Expect(name).To(Equal(errandName))
		Expect(keepAlive).To(BeFalse())
		Expect(whenChanged).To(BeFalse())
		Expect(instances).To(HaveLen(1))
		Expect(instances[0].Name()).To(Equal("errand_instance"))
		Expect(instances[0].IndexOrID()).To(BeEmpty())
	})

	It("returns an error when the errandInstance names are invalid", func() {
		errandInstances := []string{"some/invalid/errand"}

		_, actualErr := c.RunErrand(deploymentName, errandName, errandInstances, contextID, logger, taskReporter)
		Expect(actualErr).To(MatchError(ContainSubstring("Invalid instance name")))
	})

	It("errors when finding deployment fails", func() {
		fakeDirector.FindDeploymentReturns(nil, errors.New("some failure"))
		_, actualErr := c.RunErrand("", errandName, nil, contextID, logger, taskReporter)
		Expect(actualErr).To(MatchError(ContainSubstring("Could not find deployment")))
	})

	It("returns the error when bosh fails to queue up an errand", func() {
		fakeDeployment.RunErrandReturns([]director.ErrandResult{}, errors.New("some errand failure"))
		_, err := c.RunErrand(deploymentName, errandName, nil, contextID, logger, taskReporter)

		Expect(err).To(MatchError(ContainSubstring("Could not run errand")))
	})

})
