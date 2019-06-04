// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package lifecycle_tests

import (
	"fmt"
	"strings"

	"github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/bosh_helpers"

	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"

	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/test_helpers/cf_helpers"
)

var _ = Describe("lifecycle errand tests", func() {
	var serviceInstanceName, planName string

	const (
		colocatedPostDeployPlan = "lifecycle-colocated-post-deploy-plan"
		colocatedPreDeletePlan  = "lifecycle-colocated-pre-delete-plan"
		redisSmall              = "redis-small"
	)

	BeforeEach(func() {
		serviceInstanceName = uuid.New()[:7]
	})

	Describe("post-deploy", func() {
		BeforeEach(func() {
			planName = colocatedPostDeployPlan

			By("creating an instance")
			serviceInstanceName = "with-post-deploy-" + uuid.New()[:7]
			cf.CreateService(brokerInfo.ServiceName, planName, serviceInstanceName, "")
		})

		AfterEach(func() {
			By("deleting the service instance")
			cf.DeleteService(serviceInstanceName)
		})

		Context("for a plan with a colocated post-deploy errand", func() {
			It("runs the post-deploy errand after create", func() {
				boshTasks := bosh_helpers.TasksForDeployment(getServiceDeploymentName(serviceInstanceName))
				Expect(boshTasks).To(HaveLen(2))

				Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[0].Description).To(ContainSubstring("run errand"))

				Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[1].Description).To(ContainSubstring("create deployment"))
			})

			It("runs post-deploy errand after update", func() {
				By("updating the instance")
				cf.UpdateServiceWithArbitraryParams(serviceInstanceName, `{"maxclients": 101}`)

				boshTasks := bosh_helpers.TasksForDeployment(getServiceDeploymentName(serviceInstanceName))
				Expect(boshTasks).To(HaveLen(4))

				Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[0].Description).To(ContainSubstring("run errand"))

				Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[1].Description).To(ContainSubstring("create deployment"))

				Expect(boshTasks[2].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[2].Description).To(ContainSubstring("run errand"))

				Expect(boshTasks[3].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[3].Description).To(ContainSubstring("create deployment"))
			})

			Context("when changing to a plan without a post-deploy errand", func() {
				It("does not run post-deploy errand", func() {
					By("updating the instance")
					cf.UpdateServiceToPlan(serviceInstanceName, redisSmall)

					boshTasks := bosh_helpers.TasksForDeployment(getServiceDeploymentName(serviceInstanceName))
					Expect(boshTasks).To(HaveLen(3))

					Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
					Expect(boshTasks[0].Description).To(ContainSubstring("create deployment"))

					Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
					Expect(boshTasks[1].Description).To(ContainSubstring("run errand"))

					Expect(boshTasks[2].State).To(Equal(boshdirector.TaskDone))
					Expect(boshTasks[2].Description).To(ContainSubstring("create deployment"))
				})
			})
		})

		Context("for a plan without a post-deploy errand", func() {
			BeforeEach(func() {
				planName = redisSmall

				By("creating an instance")
				serviceInstanceName = "without-post-deploy-" + uuid.New()[:7]
				cf.CreateService(brokerInfo.ServiceName, planName, serviceInstanceName, "")
			})

			Context("when changing to a plan with a post-deploy errand", func() {
				It("runs the post-deploy errand", func() {
					By("updating the instance")
					cf.UpdateServiceToPlan(serviceInstanceName, colocatedPostDeployPlan)

					boshTasks := bosh_helpers.TasksForDeployment(getServiceDeploymentName(serviceInstanceName))
					Expect(boshTasks).To(HaveLen(3))

					Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
					Expect(boshTasks[0].Description).To(ContainSubstring("run errand"))

					Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
					Expect(boshTasks[1].Description).To(ContainSubstring("create deployment"))

					Expect(boshTasks[2].State).To(Equal(boshdirector.TaskDone))
					Expect(boshTasks[2].Description).To(ContainSubstring("create deployment"))
				})
			})
		})
	})

	Describe("pre-delete", func() {
		var deploymentName string

		JustBeforeEach(func() {
			By("creating an instance")
			cf.CreateService(brokerInfo.ServiceName, planName, serviceInstanceName, "")

			deploymentName = getServiceDeploymentName(serviceInstanceName)

			By("deleting the service instance")
			cf.DeleteService(serviceInstanceName)
		})

		Context("for a plan with colocated pre-delete errands", func() {
			BeforeEach(func() {
				planName = colocatedPreDeletePlan
			})
			JustBeforeEach(func() {
				cf.AwaitServiceDeletion(serviceInstanceName)
			})

			It("runs the pre-delete errand before the delete", func() {
				boshTasks := bosh_helpers.TasksForDeployment(deploymentName)

				Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[0].Description).To(ContainSubstring("delete deployment"))

				Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[1].Description).To(ContainSubstring("run errand"))
			})
		})
	})
})

func getServiceDeploymentName(serviceInstanceName string) string {
	getInstanceDetailsCmd := cf.Cf("service", serviceInstanceName, "--guid")
	Eventually(getInstanceDetailsCmd, cf.CfTimeout).Should(gexec.Exit(0))
	re := regexp.MustCompile("(?m)^[[:alnum:]]{8}-[[:alnum:]-]*$")
	serviceGUID := re.FindString(string(getInstanceDetailsCmd.Out.Contents()))
	serviceInstanceID := strings.TrimSpace(serviceGUID)
	return fmt.Sprintf("%s%s", "service-instance_", serviceInstanceID)
}
