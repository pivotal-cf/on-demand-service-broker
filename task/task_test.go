// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package task_test

import (
	"errors"
	"fmt"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/task"
	"github.com/pivotal-cf/on-demand-service-broker/task/fakes"
)

type deployer interface {
	Create(deploymentName, planID string, requestParams map[string]interface{}, boshContextID string, logger *log.Logger) (int, []byte, error)
	Update(deploymentName, planID string, requestParams map[string]interface{}, previousPlanID *string, boshContextID string, logger *log.Logger) (int, []byte, error)
	Upgrade(deploymentName, planID string, previousPlanID *string, boshContextID string, logger *log.Logger) (int, []byte, error)
}

var _ = Describe("Deployer", func() {
	const boshTaskID = 42

	var (
		boshClient    *fakes.FakeBoshClient
		deployer      deployer
		boshContextID string

		deployedManifest []byte
		deployError      error
		returnedTaskID   int

		planID         string
		previousPlanID *string
		requestParams  map[string]interface{}
		manifest       = []byte("---\nproperties:\n foo: bar")
		oldManifest    []byte

		manifestGenerator *fakes.FakeManifestGenerator
	)

	BeforeEach(func() {
		boshClient = new(fakes.FakeBoshClient)
		manifestGenerator = new(fakes.FakeManifestGenerator)
		deployer = task.NewDeployer(boshClient, manifestGenerator)

		planID = existingPlanID
		previousPlanID = nil

		requestParams = map[string]interface{}{
			"parameters": map[string]interface{}{"foo": "bar"},
		}
		oldManifest = nil
		boshContextID = ""
	})

	Describe("Create()", func() {
		JustBeforeEach(func() {
			returnedTaskID, deployedManifest, deployError = deployer.Create(
				deploymentName,
				planID,
				requestParams,
				boshContextID,
				logger,
			)
		})

		BeforeEach(func() {
			oldManifest = nil
			previousPlanID = nil
		})

		Context("when bosh deploys the release successfully", func() {
			BeforeEach(func() {
				By("not having any previous tasks")
				boshClient.GetTasksReturns([]boshdirector.BoshTask{}, nil)
				manifestGenerator.GenerateManifestReturns(manifest, nil)
				boshClient.DeployReturns(42, nil)
			})

			It("checks tasks for the deployment", func() {
				Expect(boshClient.GetTasksCallCount()).To(Equal(1))
				actualDeploymentName, _ := boshClient.GetTasksArgsForCall(0)
				Expect(actualDeploymentName).To(Equal(deploymentName))
			})

			It("calls generate manifest", func() {
				Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(1))
			})

			It("calls new manifest with correct params", func() {
				Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(1))
				passedDeploymentName, passedPlanID, passedRequestParams, passedPreviousManifest, passedPreviousPlanID, _ := manifestGenerator.GenerateManifestArgsForCall(0)

				Expect(passedDeploymentName).To(Equal(deploymentName))
				Expect(passedPlanID).To(Equal(planID))
				Expect(passedRequestParams).To(Equal(requestParams))
				Expect(passedPreviousManifest).To(Equal(oldManifest))
				Expect(passedPreviousPlanID).To(Equal(previousPlanID))
			})

			It("returns the bosh task ID", func() {
				Expect(returnedTaskID).To(Equal(boshTaskID))
			})

			It("Creates a bosh deployment using generated manifest", func() {
				Expect(boshClient.DeployCallCount()).To(Equal(1))
				deployedManifest, _, _, _ := boshClient.DeployArgsForCall(0)
				Expect(deployedManifest).To(Equal(manifest))
			})

			It("return the newly generated manifest", func() {
				Expect(deployedManifest).To(Equal(manifest))
			})

			It("does not return an error", func() {
				Expect(deployError).NotTo(HaveOccurred())
			})

			Context("when bosh context ID is provided", func() {
				BeforeEach(func() {
					boshContextID = "bosh-context-id"
				})

				It("invokes boshdirector's Create with context ID", func() {
					Expect(boshClient.DeployCallCount()).To(Equal(1))
					_, actualBoshContextID, _, _ := boshClient.DeployArgsForCall(0)
					Expect(actualBoshContextID).To(Equal(boshContextID))
				})
			})
		})

		Context("logging", func() {
			BeforeEach(func() {
				boshClient.DeployReturns(42, nil)
				boshClient.GetTasksReturns([]boshdirector.BoshTask{{State: boshdirector.TaskDone}}, nil)

				oldManifest = nil
				manifestGenerator.GenerateManifestReturns(manifest, nil)
			})

			It("logs the bosh task ID returned by the director", func() {
				Expect(deployError).ToNot(HaveOccurred())
				Expect(returnedTaskID).To(Equal(42))
				Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("Bosh task ID for create deployment %s is %d", deploymentName, boshTaskID)))
			})
		})

		Context("when manifest generator returns an error", func() {
			BeforeEach(func() {
				manifestGenerator.GenerateManifestReturns(nil, errors.New("error generating manifest"))
				boshClient.GetTasksReturns([]boshdirector.BoshTask{{State: boshdirector.TaskDone}}, nil)
				requestParams = map[string]interface{}{"foo": "bar"}
			})

			It("checks tasks for the deployment", func() {
				Expect(boshClient.GetTasksCallCount()).To(Equal(1))
				actualDeploymentName, _ := boshClient.GetTasksArgsForCall(0)
				Expect(actualDeploymentName).To(Equal(deploymentName))
			})

			It("does not deploy", func() {
				Expect(boshClient.DeployCallCount()).To(BeZero())
			})

			It("returns an error", func() {
				Expect(deployError).To(HaveOccurred())
				Expect(deployError).To(MatchError(ContainSubstring("error generating manifest")))
			})
		})

		Context("when the last bosh task for deployment is queued", func() {
			var previousDoneBoshTaskID = 41
			var previousErrorBoshTaskID = 40

			BeforeEach(func() {
				boshClient.GetTasksReturns([]boshdirector.BoshTask{
					{State: boshdirector.TaskQueued, ID: boshTaskID},
					{State: boshdirector.TaskDone, ID: previousDoneBoshTaskID},
					{State: boshdirector.TaskError, ID: previousErrorBoshTaskID},
				}, nil)
			})

			It("fails because deployment is still in progress", func() {
				Expect(deployError).To(BeAssignableToTypeOf(task.TaskInProgressError{}))

				Expect(logBuffer.String()).To(SatisfyAll(
					ContainSubstring(fmt.Sprintf("deployment %s is still in progress", deploymentName)),
					ContainSubstring("\"ID\":%d", boshTaskID),
					Not(ContainSubstring("done")),
					Not(ContainSubstring("\"ID\":%d", previousDoneBoshTaskID)),
					Not(ContainSubstring("error")),
					Not(ContainSubstring("\"ID\":%d", previousErrorBoshTaskID)),
				))
			})
		})

		Context("when the last bosh task for deployment is processing", func() {
			var previousDoneBoshTaskID = 41
			var previousErrorBoshTaskID = 40

			BeforeEach(func() {
				boshClient.GetTasksReturns([]boshdirector.BoshTask{
					{State: boshdirector.TaskProcessing, ID: boshTaskID},
					{State: boshdirector.TaskDone, ID: previousDoneBoshTaskID},
					{State: boshdirector.TaskError, ID: previousErrorBoshTaskID},
				}, nil)
			})

			It("fails because deployment is still in progress", func() {
				Expect(deployError).To(BeAssignableToTypeOf(task.TaskInProgressError{}))

				Expect(logBuffer.String()).To(SatisfyAll(
					ContainSubstring(fmt.Sprintf("deployment %s is still in progress", deploymentName)),
					ContainSubstring("\"ID\":%d", boshTaskID),
					Not(ContainSubstring("done")),
					Not(ContainSubstring("\"ID\":%d", previousDoneBoshTaskID)),
					Not(ContainSubstring("error")),
					Not(ContainSubstring("\"ID\":%d", previousErrorBoshTaskID)),
				))
			})
		})

		Context("when the last bosh task for deployment fails to fetch", func() {
			BeforeEach(func() {
				boshClient.GetTasksReturns(nil, errors.New("connection error"))
			})

			It("wraps the error", func() {
				Expect(deployError).To(MatchError(fmt.Sprintf("error getting tasks for deployment %s: connection error\n", deploymentName)))
			})
		})

		Context("when bosh fails to deploy the release", func() {
			BeforeEach(func() {
				boshClient.GetTasksReturns([]boshdirector.BoshTask{{State: boshdirector.TaskDone}}, nil)
				boshClient.DeployReturns(0, errors.New("error deploying"))
			})

			It("wraps the error", func() {
				Expect(deployError).To(MatchError(ContainSubstring("error deploying")))
			})
		})
	})

	Describe("Upgrade()", func() {
		JustBeforeEach(func() {
			returnedTaskID, deployedManifest, deployError = deployer.Upgrade(
				deploymentName,
				planID,
				previousPlanID,
				boshContextID,
				logger,
			)
		})

		BeforeEach(func() {
			oldManifest = []byte("---\nold-manifest-fetched-from-bosh: bar")
			previousPlanID = stringPointer(existingPlanID)

			boshClient.GetDeploymentReturns(oldManifest, true, nil)
			boshClient.GetTasksReturns([]boshdirector.BoshTask{}, nil)
			manifestGenerator.GenerateManifestReturns(manifest, nil)
			boshClient.DeployReturns(42, nil)
		})

		Context("when bosh deploys the release successfully", func() {
			BeforeEach(func() {
				By("not having any previous tasks")
				boshClient.GetDeploymentReturns(oldManifest, true, nil)
				boshClient.GetTasksReturns([]boshdirector.BoshTask{}, nil)
				manifestGenerator.GenerateManifestReturns(manifest, nil)
				boshClient.DeployReturns(42, nil)
			})

			It("checks that the deployment exists", func() {
				Expect(boshClient.GetDeploymentCallCount()).To(Equal(1))
				actualDeploymentName, _ := boshClient.GetDeploymentArgsForCall(0)
				Expect(actualDeploymentName).To(Equal(deploymentName))
			})

			It("checks tasks for the deployment", func() {
				Expect(boshClient.GetTasksCallCount()).To(Equal(1))
				actualDeploymentName, _ := boshClient.GetTasksArgsForCall(0)
				Expect(actualDeploymentName).To(Equal(deploymentName))
			})

			It("calls generate manifest", func() {
				Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(1))
			})

			It("calls new manifest with correct params", func() {
				Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(1))
				passedDeploymentName, passedPlanID, passedRequestParams, passedPreviousManifest, passedPreviousPlanID, _ := manifestGenerator.GenerateManifestArgsForCall(0)

				Expect(passedDeploymentName).To(Equal(deploymentName))
				Expect(passedPlanID).To(Equal(planID))
				Expect(passedRequestParams).To(BeNil())
				Expect(passedPreviousManifest).To(Equal(oldManifest))
				Expect(passedPreviousPlanID).To(Equal(previousPlanID))
			})

			It("returns the bosh task ID", func() {
				Expect(returnedTaskID).To(Equal(boshTaskID))
			})

			It("Creates a bosh deployment using generated manifest", func() {
				Expect(boshClient.DeployCallCount()).To(Equal(1))
				deployedManifest, _, _, _ := boshClient.DeployArgsForCall(0)
				Expect(deployedManifest).To(Equal(manifest))
			})

			It("return the newly generated manifest", func() {
				Expect(deployedManifest).To(Equal(manifest))
			})

			It("does not return an error", func() {
				Expect(deployError).NotTo(HaveOccurred())
			})

			Context("when bosh context ID is provided", func() {
				BeforeEach(func() {
					boshContextID = "bosh-context-id"
				})

				It("invokes boshdirector's Create with context ID", func() {
					Expect(boshClient.DeployCallCount()).To(Equal(1))
					_, actualBoshContextID, _, _ := boshClient.DeployArgsForCall(0)
					Expect(actualBoshContextID).To(Equal(boshContextID))
				})
			})
		})

		Context("logging", func() {
			BeforeEach(func() {
				boshClient.DeployReturns(42, nil)
				boshClient.GetTasksReturns([]boshdirector.BoshTask{{State: boshdirector.TaskDone}}, nil)
			})

			It("logs the bosh task ID returned by the director", func() {
				Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("Bosh task ID for upgrade deployment %s is %d", deploymentName, boshTaskID)))
			})
		})

		Context("when the deployment cannot be found", func() {
			BeforeEach(func() {
				boshClient.GetDeploymentReturns(nil, false, nil)
			})

			It("returns a deployment not found error", func() {
				Expect(deployError).To(MatchError(ContainSubstring("not found")))
				Expect(boshClient.DeployCallCount()).To(Equal(0))
			})
		})

		Context("when getting the deployment fails", func() {
			BeforeEach(func() {
				boshClient.GetDeploymentReturns(nil, false, errors.New("error getting deployment"))
			})

			It("returns a deployment not found error", func() {
				Expect(deployError).To(MatchError(errors.New("error getting deployment")))
				Expect(boshClient.DeployCallCount()).To(Equal(0))
			})
		})

		It("does not check for pending changes", func() {
			Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(1))
		})

		It("returns the bosh task ID and new manifest", func() {
			Expect(returnedTaskID).To(Equal(42))
			Expect(deployedManifest).To(Equal(manifest))
			Expect(deployError).NotTo(HaveOccurred())
		})

		Context("when manifest generator returns an error", func() {
			BeforeEach(func() {
				manifestGenerator.GenerateManifestReturns(nil, errors.New("error generating manifest"))
				boshClient.GetTasksReturns([]boshdirector.BoshTask{{State: boshdirector.TaskDone}}, nil)
				requestParams = map[string]interface{}{"foo": "bar"}
			})

			It("checks tasks for the deployment", func() {
				Expect(boshClient.GetTasksCallCount()).To(Equal(1))
				actualDeploymentName, _ := boshClient.GetTasksArgsForCall(0)
				Expect(actualDeploymentName).To(Equal(deploymentName))
			})

			It("does not deploy", func() {
				Expect(boshClient.DeployCallCount()).To(BeZero())
			})

			It("returns an error", func() {
				Expect(deployError).To(HaveOccurred())
				Expect(deployError).To(MatchError(ContainSubstring("error generating manifest")))
			})
		})

		Context("when the last bosh task for deployment is queued", func() {
			var previousDoneBoshTaskID = 41
			var previousErrorBoshTaskID = 40

			var queuedTask = boshdirector.BoshTask{State: boshdirector.TaskQueued, ID: boshTaskID}

			BeforeEach(func() {
				boshClient.GetTasksReturns([]boshdirector.BoshTask{
					queuedTask,
					{State: boshdirector.TaskDone, ID: previousDoneBoshTaskID},
					{State: boshdirector.TaskError, ID: previousErrorBoshTaskID},
				}, nil)
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring(
					fmt.Sprintf("deployment %s is still in progress: tasks %s\n",
						deploymentName,
						boshdirector.BoshTasks{queuedTask}.ToLog(),
					),
				))
			})

			It("returns an error", func() {
				Expect(deployError).To(BeAssignableToTypeOf(task.TaskInProgressError{}))
			})

			It("does not log the previous completed tasks for the deployment", func() {
				Expect(logBuffer.String()).NotTo(ContainSubstring("done"))
				Expect(logBuffer.String()).NotTo(ContainSubstring("\"ID\":%d", previousDoneBoshTaskID))
				Expect(logBuffer.String()).NotTo(ContainSubstring("error"))
				Expect(logBuffer.String()).NotTo(ContainSubstring("\"ID\":%d", previousErrorBoshTaskID))
			})
		})

		Context("when the last bosh task for deployment is processing", func() {
			var previousDoneBoshTaskID = 41
			var previousErrorBoshTaskID = 40

			var inProgressTask = boshdirector.BoshTask{State: boshdirector.TaskProcessing, ID: boshTaskID}

			BeforeEach(func() {
				boshClient.GetTasksReturns([]boshdirector.BoshTask{
					inProgressTask,
					{State: boshdirector.TaskDone, ID: previousDoneBoshTaskID},
					{State: boshdirector.TaskError, ID: previousErrorBoshTaskID},
				}, nil)
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring(
					fmt.Sprintf("deployment %s is still in progress: tasks %s\n",
						deploymentName,
						boshdirector.BoshTasks{inProgressTask}.ToLog(),
					),
				))
			})

			It("returns an error", func() {
				Expect(deployError).To(BeAssignableToTypeOf(task.TaskInProgressError{}))
			})

			It("does not log the previous tasks for the deployment", func() {
				Expect(logBuffer.String()).NotTo(ContainSubstring("done"))
				Expect(logBuffer.String()).NotTo(ContainSubstring("\"ID\":%d", previousDoneBoshTaskID))
				Expect(logBuffer.String()).NotTo(ContainSubstring("error"))
				Expect(logBuffer.String()).NotTo(ContainSubstring("\"ID\":%d", previousErrorBoshTaskID))
			})
		})

		Context("when the last bosh task for deployment fails to fetch", func() {
			BeforeEach(func() {
				boshClient.GetTasksReturns(nil, errors.New("connection error"))
			})

			It("wraps the error", func() {
				Expect(deployError).To(MatchError(fmt.Sprintf("error getting tasks for deployment %s: connection error\n", deploymentName)))
			})
		})

		Context("when bosh fails to deploy the release", func() {
			BeforeEach(func() {
				boshClient.GetTasksReturns([]boshdirector.BoshTask{{State: boshdirector.TaskDone}}, nil)
				boshClient.DeployReturns(0, errors.New("error deploying"))
			})

			It("wraps the error", func() {
				Expect(deployError).To(MatchError(ContainSubstring("error deploying")))
			})
		})
	})

	Describe("Update()", func() {
		BeforeEach(func() {
			oldManifest = []byte("---\nupdate:\n canaries: 5\n max_in_flight: 1")

			previousPlanID = stringPointer(existingPlanID)

			boshClient.GetTasksReturns([]boshdirector.BoshTask{{State: boshdirector.TaskDone}}, nil)
			boshClient.GetDeploymentReturns(oldManifest, true, nil)
		})

		Context("and the manifest generator fails to generate the manifest the first time", func() {
			BeforeEach(func() {
				manifestGenerator.GenerateManifestReturns(nil, errors.New("manifest fail"))
			})

			It("wraps the error", func() {
				returnedTaskID, deployedManifest, deployError = deployer.Update(
					deploymentName,
					planID,
					requestParams,
					previousPlanID,
					boshContextID,
					logger,
				)

				Expect(deployError).To(MatchError(ContainSubstring("manifest fail")))
			})
		})

		Context("and there are no pending changes", func() {
			Context("and manifest generation succeeds", func() {
				BeforeEach(func() {
					requestParams = map[string]interface{}{"foo": "bar"}
					manifestGenerator.GenerateManifestStub = func(
						_, _ string,
						requestParams map[string]interface{},
						previousManifest []byte,
						_ *string,
						_ *log.Logger,
					) (task.RawBoshManifest, error) {
						if len(requestParams) > 0 {
							return manifest, nil
						}
						return previousManifest, nil
					}

					boshClient.DeployReturns(42, nil)
				})

				It("deploys successfully", func() {
					returnedTaskID, deployedManifest, deployError = deployer.Update(
						deploymentName,
						planID,
						requestParams,
						previousPlanID,
						boshContextID,
						logger,
					)

					Expect(boshClient.GetTasksCallCount()).To(Equal(1))
					actualDeploymentName, _ := boshClient.GetTasksArgsForCall(0)
					Expect(actualDeploymentName).To(Equal(deploymentName))

					Expect(boshClient.GetDeploymentCallCount()).To(Equal(1))
					actualDeploymentName, _ = boshClient.GetDeploymentArgsForCall(0)
					Expect(actualDeploymentName).To(Equal(deploymentName))

					Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(2))

					_, _, passedRequestParams, _, _, _ := manifestGenerator.GenerateManifestArgsForCall(0)
					Expect(passedRequestParams).To(BeEmpty())

					_, _, passedRequestParams, _, _, _ = manifestGenerator.GenerateManifestArgsForCall(1)
					Expect(passedRequestParams).To(Equal(requestParams))

					Expect(boshClient.DeployCallCount()).To(Equal(1))
					deployedManifest, _, _, _ := boshClient.DeployArgsForCall(0)
					Expect(string(deployedManifest)).To(Equal(string(manifest)))

					Expect(returnedTaskID).To(Equal(boshTaskID))
				})

				Context("and there are no parameters configured", func() {
					It("deploys successfully", func() {
						requestParams = map[string]interface{}{}

						returnedTaskID, deployedManifest, deployError = deployer.Update(
							deploymentName,
							planID,
							requestParams,
							previousPlanID,
							boshContextID,
							logger,
						)

						Expect(deployError).NotTo(HaveOccurred())
					})
				})
			})

			Context("and the manifest generator fails to generate the manifest the second time", func() {
				BeforeEach(func() {
					manifestGenerator.GenerateManifestStub = func(
						_, _ string,
						requestParams map[string]interface{},
						previousManifest []byte,
						_ *string,
						_ *log.Logger,
					) (task.RawBoshManifest, error) {
						if len(requestParams) > 0 {
							return nil, errors.New("manifest fail")
						}
						return previousManifest, nil
					}
				})

				It("wraps the error", func() {
					returnedTaskID, deployedManifest, deployError = deployer.Update(
						deploymentName,
						planID,
						requestParams,
						previousPlanID,
						boshContextID,
						logger,
					)

					Expect(boshClient.DeployCallCount()).To(Equal(0))
					Expect(deployError).To(MatchError(ContainSubstring("manifest fail")))
				})
			})
		})

		Context("and there are pending changes", func() {
			BeforeEach(func() {
				manifestGenerator.GenerateManifestReturns(manifest, nil)
			})

			It("fails without deploying", func() {
				returnedTaskID, deployedManifest, deployError = deployer.Update(
					deploymentName,
					planID,
					requestParams,
					previousPlanID,
					boshContextID,
					logger,
				)

				Expect(deployError).To(HaveOccurred())
				Expect(deployError).To(BeAssignableToTypeOf(task.PendingChangesNotAppliedError{}))
				Expect(boshClient.DeployCallCount()).To(BeZero())
			})
		})

		Context("when the deployment cannot be found", func() {
			BeforeEach(func() {
				boshClient.GetDeploymentReturns(nil, false, nil)
			})

			It("returns a deployment not found error", func() {
				returnedTaskID, deployedManifest, deployError = deployer.Update(
					deploymentName,
					planID,
					requestParams,
					previousPlanID,
					boshContextID,
					logger,
				)

				Expect(deployError).To(MatchError(ContainSubstring("not found")))
				Expect(boshClient.DeployCallCount()).To(Equal(0))
			})
		})

		Context("and when the last bosh task for deployment fails to fetch", func() {
			BeforeEach(func() {
				boshClient.GetTasksReturns(nil, errors.New("connection error"))
			})

			It("wraps the error", func() {
				returnedTaskID, deployedManifest, deployError = deployer.Update(
					deploymentName,
					planID,
					requestParams,
					previousPlanID,
					boshContextID,
					logger,
				)

				Expect(deployError).To(MatchError(fmt.Sprintf("error getting tasks for deployment %s: connection error\n", deploymentName)))
			})
		})

		Context("when getting the deployment fails", func() {
			BeforeEach(func() {
				boshClient.GetDeploymentReturns(nil, false, errors.New("error getting deployment"))
			})

			It("returns a deployment not found error", func() {
				returnedTaskID, deployedManifest, deployError = deployer.Update(
					deploymentName,
					planID,
					requestParams,
					previousPlanID,
					boshContextID,
					logger,
				)

				Expect(deployError).To(MatchError(errors.New("error getting deployment")))
				Expect(boshClient.DeployCallCount()).To(Equal(0))
			})
		})

		It("ignores the update block when the manifest generator generates a new manifest with a different update block", func() {
			previousPlanID = stringPointer(existingPlanID)

			generatedManifest := []byte("---\nupdate:\n canaries: 2\n max_in_flight: 1")

			requestParams = map[string]interface{}{"foo": "bar"}

			manifestGenerator.GenerateManifestReturns(generatedManifest, nil)
			boshClient.DeployReturns(42, nil)

			returnedTaskID, deployedManifest, deployError = deployer.Update(
				deploymentName,
				planID,
				requestParams,
				previousPlanID,
				boshContextID,
				logger,
			)

			Expect(deployError).To(BeNil())

			Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(2))
			_, _, passedRequestParams, _, _, _ := manifestGenerator.GenerateManifestArgsForCall(1)
			Expect(passedRequestParams).To(Equal(requestParams))

			manifestToDeploy, _, _, _ := boshClient.DeployArgsForCall(0)
			Expect(string(manifestToDeploy)).To(Equal(string(generatedManifest)))
		})

		It("detects changes to the tags block in a manifest and prevents deployment", func() {
			oldManifest = []byte(`---
tags:
  product: another-tag
`)
			boshClient.GetDeploymentReturns(oldManifest, true, nil)
			previousPlanID = stringPointer(existingPlanID)

			generatedManifest := []byte(`---
tags:
  product: some-tag
`)
			manifestGenerator.GenerateManifestReturns(generatedManifest, nil)

			_, _, deployError = deployer.Update(
				deploymentName,
				planID,
				requestParams,
				previousPlanID,
				boshContextID,
				logger,
			)

			Expect(deployError).To(HaveOccurred())
			Expect(deployError).To(BeAssignableToTypeOf(task.PendingChangesNotAppliedError{}))
			Expect(boshClient.DeployCallCount()).To(BeZero())
		})

		BeforeEach(func() {
		})

		It("detects changes to the env block in a manifest instance group and prevents deployment", func() {
			oldManifest = []byte(`---
instance_groups:
- name: hello
  env:
    bosh:
      password: password
    some_other_key: skeleton
`)
			boshClient.GetDeploymentReturns(oldManifest, true, nil)
			previousPlanID = stringPointer(existingPlanID)

			generatedManifest := []byte(`---
instance_groups:
- name: hello
  env:
    bosh:
      password: passwerd
    some_other_key: a_major
`)
			manifestGenerator.GenerateManifestReturns(generatedManifest, nil)

			_, _, deployError = deployer.Update(
				deploymentName,
				planID,
				requestParams,
				previousPlanID,
				boshContextID,
				logger,
			)

			Expect(deployError).To(HaveOccurred())
			Expect(deployError).To(BeAssignableToTypeOf(task.PendingChangesNotAppliedError{}))
			Expect(boshClient.DeployCallCount()).To(BeZero())
		})

		It("returns an error when old manifest contains invalid YAML", func() {
			previousPlanID = stringPointer(existingPlanID)

			oldManifestWithInvalidYAML := []byte("{")
			generatedManifest := []byte("---\nupdate:\n canaries: 2\n max_in_flight: 1")

			requestParams = map[string]interface{}{"foo": "bar"}

			boshClient.GetDeploymentReturns(oldManifestWithInvalidYAML, true, nil)
			manifestGenerator.GenerateManifestReturns(generatedManifest, nil)
			boshClient.DeployReturns(42, nil)

			returnedTaskID, deployedManifest, deployError = deployer.Update(
				deploymentName,
				planID,
				requestParams,
				previousPlanID,
				boshContextID,
				logger,
			)

			Expect(deployError.Error()).To(ContainSubstring("error detecting change in manifest, unable to unmarshal manifest"))
			Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(1))
		})

		It("returns an error when the generated manifest returns invalid YAML", func() {
			previousPlanID = stringPointer(existingPlanID)

			oldManifest := []byte("---\nupdate:\n canaries: 5\n max_in_flight: 1")
			generatedManifestWithInvalidYAML := []byte("{")

			requestParams = map[string]interface{}{"foo": "bar"}

			boshClient.GetDeploymentReturns(oldManifest, true, nil)
			manifestGenerator.GenerateManifestReturns(generatedManifestWithInvalidYAML, nil)
			boshClient.DeployReturns(42, nil)

			returnedTaskID, deployedManifest, deployError = deployer.Update(
				deploymentName,
				planID,
				requestParams,
				previousPlanID,
				boshContextID,
				logger,
			)

			Expect(deployError.Error()).To(ContainSubstring("error detecting change in manifest, unable to unmarshal manifest"))
			Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(1))
		})

	})
})

func stringPointer(s string) *string {
	return &s
}
