// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package lifecycle_tests

import (
	"fmt"
	"strings"

	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	cf "github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

var _ = Describe("lifecycle errand tests", func() {
	var serviceInstanceName, planName string

	const (
		postDeployPlan          = "lifecycle-post-deploy-plan"
		postDeployFailsPlan     = "lifecycle-failing-health-check-plan"
		cleanupPlan             = "lifecycle-cleanup-data-plan"
		cleanupFailsPlan        = "lifecycle-failing-cleanup-data-plan"
		colocatedPostDeployPlan = "lifecycle-colocated-post-deploy-plan"
		colocatedPreDeletePlan  = "lifecycle-colocated-pre-delete-plan"
		dedicatedVMPlan         = "dedicated-vm"
	)

	BeforeEach(func() {
		serviceInstanceName = uuid.New()[:7]
	})

	Describe("post-deploy", func() {

		AfterEach(func() {
			By("deleting the service instance")
			deleteServiceSession := cf.Cf("delete-service", serviceInstanceName, "-f")
			Eventually(deleteServiceSession, cf.CfTimeout).Should(gexec.Exit(0))
			cf.AwaitServiceDeletion(serviceInstanceName)
		})

		Context("for a plan with a colocated post-deploy errand", func() {
			It("runs the post-deploy errand after create", func() {
				planName = colocatedPostDeployPlan

				By("creating an instance")
				createServiceSession := cf.Cf("create-service", serviceOffering, planName, serviceInstanceName)
				Eventually(createServiceSession, cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceCreation(serviceInstanceName)

				boshTasks := boshClient.GetTasksForDeployment(getServiceDeploymentName(serviceInstanceName))
				Expect(boshTasks).To(HaveLen(2))

				Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[0].Description).To(ContainSubstring("run errand"))

				Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[1].Description).To(ContainSubstring("create deployment"))
			})
		})

		Context("for a plan with a post-deploy errand", func() {
			Context("when post-deploy errand succeeds", func() {
				BeforeEach(func() {
					planName = postDeployPlan

					By("creating an instance")
					createServiceSession := cf.Cf("create-service", serviceOffering, planName, serviceInstanceName)
					Eventually(createServiceSession, cf.CfTimeout).Should(gexec.Exit(0))
					cf.AwaitServiceCreation(serviceInstanceName)
				})

				It("runs post-deploy errand after create", func() {
					boshTasks := boshClient.GetTasksForDeployment(getServiceDeploymentName(serviceInstanceName))
					Expect(boshTasks).To(HaveLen(2))

					Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
					Expect(boshTasks[0].Description).To(ContainSubstring("run errand"))

					Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
					Expect(boshTasks[1].Description).To(ContainSubstring("create deployment"))
				})

				It("runs post-deploy errand after update", func() {
					By("updating the instance")
					updateServiceSession := cf.Cf("update-service", serviceInstanceName, "-c", `{"maxclients": 101}`)
					Eventually(updateServiceSession, cf.CfTimeout).Should(gexec.Exit(0))
					cf.AwaitServiceUpdate(serviceInstanceName)

					boshTasks := boshClient.GetTasksForDeployment(getServiceDeploymentName(serviceInstanceName))
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
						updateServiceSession := cf.Cf("update-service", serviceInstanceName, "-p", dedicatedVMPlan)
						Eventually(updateServiceSession, cf.CfTimeout).Should(gexec.Exit(0))
						cf.AwaitServiceUpdate(serviceInstanceName)

						boshTasks := boshClient.GetTasksForDeployment(getServiceDeploymentName(serviceInstanceName))
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

			Context("when post-deploy errand fails", func() {
				It("fails to create service", func() {
					planName = postDeployFailsPlan

					By("creating the instance")
					createServiceSession := cf.Cf("create-service", serviceOffering, planName, serviceInstanceName)
					Eventually(createServiceSession, cf.CfTimeout).Should(gexec.Exit(0))
					cf.AwaitServiceCreationFailure(serviceInstanceName)

					boshTasks := boshClient.GetTasksForDeployment(getServiceDeploymentName(serviceInstanceName))
					Expect(boshTasks).To(HaveLen(2))

					Expect(boshTasks[0].State).To(Equal(boshdirector.TaskError))
					Expect(boshTasks[0].Description).To(ContainSubstring("run errand"))

					Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
					Expect(boshTasks[1].Description).To(ContainSubstring("create deployment"))
				})
			})
		})

		Context("for a plan without a post-deploy errand", func() {
			BeforeEach(func() {
				planName = dedicatedVMPlan

				By("creating an instance")
				createServiceSession := cf.Cf("create-service", serviceOffering, planName, serviceInstanceName)
				Eventually(createServiceSession, cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceCreation(serviceInstanceName)
			})

			It("does not run post-deploy errand after create", func() {
				boshTasks := boshClient.GetTasksForDeployment(getServiceDeploymentName(serviceInstanceName))
				Expect(boshTasks).To(HaveLen(1))

				Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[0].Description).To(ContainSubstring("create deployment"))
			})

			It("does not run post-deploy errand after update", func() {
				By("updating the instance")
				updateServiceSession := cf.Cf("update-service", serviceInstanceName, "-c", `{"maxclients": 101}`)
				Eventually(updateServiceSession, cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceUpdate(serviceInstanceName)

				boshTasks := boshClient.GetTasksForDeployment(getServiceDeploymentName(serviceInstanceName))
				Expect(boshTasks).To(HaveLen(2))

				Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[0].Description).To(ContainSubstring("create deployment"))

				Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[1].Description).To(ContainSubstring("create deployment"))
			})

			Context("when changing to a plan with a post-deploy errand", func() {
				It("runs the post-deploy errand", func() {
					By("updating the instance")
					updateServiceSession := cf.Cf("update-service", serviceInstanceName, "-p", postDeployPlan)
					Eventually(updateServiceSession, cf.CfTimeout).Should(gexec.Exit(0))
					cf.AwaitServiceUpdate(serviceInstanceName)

					boshTasks := boshClient.GetTasksForDeployment(getServiceDeploymentName(serviceInstanceName))
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
			createServiceSession := cf.Cf("create-service", serviceOffering, planName, serviceInstanceName)
			Eventually(createServiceSession, cf.CfTimeout).Should(gexec.Exit(0))
			cf.AwaitServiceCreation(serviceInstanceName)

			deploymentName = getServiceDeploymentName(serviceInstanceName)

			By("deleting the service instance")
			deleteServiceSession := cf.Cf("delete-service", "-f", serviceInstanceName)
			Eventually(deleteServiceSession, cf.CfTimeout).Should(gexec.Exit(0))
		})

		Context("for a plan with a colocated pre-delete errand", func() {
			BeforeEach(func() {
				planName = colocatedPreDeletePlan
			})
			JustBeforeEach(func() {
				cf.AwaitServiceDeletion(serviceInstanceName)
			})

			It("runs the pre-delete errand before the delete", func() {
				boshTasks := boshClient.GetTasksForDeployment(deploymentName)

				Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[0].Description).To(ContainSubstring("delete deployment"))

				Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[1].Description).To(ContainSubstring("run errand"))

			})
		})

		Context("when pre-delete errand succeeds", func() {
			BeforeEach(func() {
				planName = cleanupPlan
			})

			JustBeforeEach(func() {
				cf.AwaitServiceDeletion(serviceInstanceName)
			})

			It("runs pre-delete errand before delete", func() {
				boshTasks := boshClient.GetTasksForDeployment(deploymentName)

				Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[0].Description).To(ContainSubstring("delete deployment"))

				Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[1].Description).To(ContainSubstring("run errand"))
			})
		})

		Context("when pre-delete errand fails", func() {
			BeforeEach(func() {
				planName = cleanupFailsPlan
			})

			JustBeforeEach(func() {
				cf.AwaitServiceDeletionFailure(serviceInstanceName)
			})

			AfterEach(func() {
				updateServiceSession := cf.Cf("update-service", serviceInstanceName, "-p", dedicatedVMPlan)
				Eventually(updateServiceSession, cf.CfTimeout).Should(gexec.Exit(0))
				cf.AwaitServiceUpdate(serviceInstanceName)

				deleteServiceSession := cf.Cf("delete-service", serviceInstanceName, "-f")
				Eventually(deleteServiceSession, cf.CfTimeout).
					Should(gexec.Exit(0))
				cf.AwaitServiceDeletion(serviceInstanceName)
			})

			It("does not delete the service", func() {
				By("ensuring the service instance exists")
				serviceSession := cf.Cf("service", serviceInstanceName)
				Eventually(serviceSession, cf.CfTimeout).Should(gexec.Exit(0))

				By("ensuring only the errand bosh task ran")
				boshTasks := boshClient.GetTasksForDeployment(deploymentName)

				Expect(boshTasks[0].State).To(Equal(boshdirector.TaskError))
				Expect(boshTasks[0].Description).To(ContainSubstring("run errand"))
			})
		})

		Context("when pre-delete errand not configured", func() {
			BeforeEach(func() {
				planName = dedicatedVMPlan
			})

			JustBeforeEach(func() {
				cf.AwaitServiceDeletion(serviceInstanceName)
			})

			It("only runs delete deployment (after create)", func() {
				boshTasks := boshClient.GetTasksForDeployment(deploymentName)

				Expect(boshTasks[0].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[0].Description).To(ContainSubstring("delete deployment"))

				Expect(boshTasks[1].State).To(Equal(boshdirector.TaskDone))
				Expect(boshTasks[1].Description).To(ContainSubstring("create deployment"))
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
