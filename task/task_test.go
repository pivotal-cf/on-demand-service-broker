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
	"github.com/pivotal-cf/on-demand-service-broker/boshclient"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/task"
	"github.com/pivotal-cf/on-demand-service-broker/task/fakes"
)

type deployer interface {
	Create(deploymentName, planID string, requestParams map[string]interface{}, boshContextID string, logger *log.Logger) (int, []byte, error)
	Update(deploymentName, planID string, applyPendingChanges bool, requestParams map[string]interface{}, previousPlanID *string, boshContextID string, logger *log.Logger) (int, []byte, error)
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
		manifest       = []byte("---\nmanifest: deployment")
		oldManifest    []byte

		manifestGenerator *fakes.FakeManifestGenerator
		featureFlags      *fakes.FakeFeatureFlags
	)

	BeforeEach(func() {
		boshClient = new(fakes.FakeBoshClient)
		manifestGenerator = new(fakes.FakeManifestGenerator)
		featureFlags = new(fakes.FakeFeatureFlags)
		featureFlags.CFUserTriggeredUpgradesReturns(false)

		deployer = task.NewDeployer(boshClient, manifestGenerator, featureFlags)

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
				boshClient.GetTasksReturns([]boshclient.BoshTask{}, nil)
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
				deployedManifest, _, _ := boshClient.DeployArgsForCall(0)
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

				It("invokes boshclient's Create with context ID", func() {
					Expect(boshClient.DeployCallCount()).To(Equal(1))
					_, actualBoshContextID, _ := boshClient.DeployArgsForCall(0)
					Expect(actualBoshContextID).To(Equal(boshContextID))
				})
			})
		})

		Context("logging", func() {
			BeforeEach(func() {
				boshClient.DeployReturns(42, nil)
				boshClient.GetTasksReturns([]boshclient.BoshTask{{State: boshclient.BoshTaskDone}}, nil)

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
				boshClient.GetTasksReturns([]boshclient.BoshTask{{State: boshclient.BoshTaskDone}}, nil)
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
				boshClient.GetTasksReturns([]boshclient.BoshTask{
					{State: boshclient.BoshTaskQueued, ID: boshTaskID},
					{State: boshclient.BoshTaskDone, ID: previousDoneBoshTaskID},
					{State: boshclient.BoshTaskError, ID: previousErrorBoshTaskID},
				}, nil)
			})

			It("returns an error for the operator", func() {
				Expect(deployError).To(MatchError(ContainSubstring(fmt.Sprintf("deployment %s is still in progress", deploymentName))))
				Expect(deployError).To(MatchError(ContainSubstring("{\"ID\":%d", boshTaskID)))
			})

			It("returns an error for the CF user", func() {
				Expect(deployError).To(BeAssignableToTypeOf(broker.DisplayableError{}))
				displayableErr, _ := deployError.(broker.DisplayableError)
				Expect(displayableErr.ErrorForCFUser()).To(MatchError("An operation is in progress for your service instance. Please try again later."))
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

			BeforeEach(func() {
				boshClient.GetTasksReturns([]boshclient.BoshTask{
					{State: boshclient.BoshTaskProcessing, ID: boshTaskID},
					{State: boshclient.BoshTaskDone, ID: previousDoneBoshTaskID},
					{State: boshclient.BoshTaskError, ID: previousErrorBoshTaskID},
				}, nil)
			})

			It("returns an error", func() {
				Expect(deployError).To(MatchError(ContainSubstring(fmt.Sprintf("deployment %s is still in progress", deploymentName))))
				Expect(deployError).To(MatchError(ContainSubstring("\"ID\":%d", boshTaskID)))
			})

			It("returns an error for the CF user", func() {
				Expect(deployError).To(BeAssignableToTypeOf(broker.DisplayableError{}))
				displayableErr, _ := deployError.(broker.DisplayableError)
				Expect(displayableErr.ErrorForCFUser()).To(MatchError("An operation is in progress for your service instance. Please try again later."))
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
				boshClient.GetTasksReturns([]boshclient.BoshTask{{State: boshclient.BoshTaskDone}}, nil)
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
			boshClient.GetTasksReturns([]boshclient.BoshTask{}, nil)
			manifestGenerator.GenerateManifestReturns(manifest, nil)
			boshClient.DeployReturns(42, nil)
		})

		Context("when bosh deploys the release successfully", func() {
			BeforeEach(func() {
				By("not having any previous tasks")
				boshClient.GetDeploymentReturns(oldManifest, true, nil)
				boshClient.GetTasksReturns([]boshclient.BoshTask{}, nil)
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
				deployedManifest, _, _ := boshClient.DeployArgsForCall(0)
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

				It("invokes boshclient's Create with context ID", func() {
					Expect(boshClient.DeployCallCount()).To(Equal(1))
					_, actualBoshContextID, _ := boshClient.DeployArgsForCall(0)
					Expect(actualBoshContextID).To(Equal(boshContextID))
				})
			})
		})

		Context("logging", func() {
			BeforeEach(func() {
				boshClient.DeployReturns(42, nil)
				boshClient.GetTasksReturns([]boshclient.BoshTask{{State: boshclient.BoshTaskDone}}, nil)
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
				boshClient.GetTasksReturns([]boshclient.BoshTask{{State: boshclient.BoshTaskDone}}, nil)
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

			var queuedTask = boshclient.BoshTask{State: boshclient.BoshTaskQueued, ID: boshTaskID}

			BeforeEach(func() {
				boshClient.GetTasksReturns([]boshclient.BoshTask{
					queuedTask,
					{State: boshclient.BoshTaskDone, ID: previousDoneBoshTaskID},
					{State: boshclient.BoshTaskError, ID: previousErrorBoshTaskID},
				}, nil)
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring(
					fmt.Sprintf("deployment %s is still in progress: tasks %s\n",
						deploymentName,
						boshclient.BoshTasks{queuedTask}.ToLog(),
					),
				))
			})

			It("returns an error", func() {
				Expect(deployError).To(MatchError("An operation is in progress for your service instance. Please try again later."))
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

			var inProgressTask = boshclient.BoshTask{State: boshclient.BoshTaskProcessing, ID: boshTaskID}

			BeforeEach(func() {
				boshClient.GetTasksReturns([]boshclient.BoshTask{
					inProgressTask,
					{State: boshclient.BoshTaskDone, ID: previousDoneBoshTaskID},
					{State: boshclient.BoshTaskError, ID: previousErrorBoshTaskID},
				}, nil)
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring(
					fmt.Sprintf("deployment %s is still in progress: tasks %s\n",
						deploymentName,
						boshclient.BoshTasks{inProgressTask}.ToLog(),
					),
				))
			})

			It("returns an error", func() {
				Expect(deployError).To(MatchError("An operation is in progress for your service instance. Please try again later."))
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
				boshClient.GetTasksReturns([]boshclient.BoshTask{{State: boshclient.BoshTaskDone}}, nil)
				boshClient.DeployReturns(0, errors.New("error deploying"))
			})

			It("wraps the error", func() {
				Expect(deployError).To(MatchError(ContainSubstring("error deploying")))
			})
		})
	})

	Describe("Update()", func() {
		var applyPendingChanges bool

		JustBeforeEach(func() {
			returnedTaskID, deployedManifest, deployError = deployer.Update(
				deploymentName,
				planID,
				applyPendingChanges,
				requestParams,
				previousPlanID,
				boshContextID,
				logger,
			)
		})

		BeforeEach(func() {
			applyPendingChanges = false
			oldManifest = []byte("---\nold-manifest-fetched-from-bosh: bar")
			previousPlanID = stringPointer(existingPlanID)

			boshClient.GetDeploymentReturns(oldManifest, true, nil)
			boshClient.GetTasksReturns([]boshclient.BoshTask{{State: boshclient.BoshTaskDone}}, nil)
		})

		Context("and the manifest generator fails to generate the manifest the first time", func() {
			BeforeEach(func() {
				manifestGenerator.GenerateManifestReturns(nil, errors.New("manifest fail"))
			})

			It("wraps the error", func() {
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
					) (task.BoshManifest, error) {
						if len(requestParams) > 0 {
							return manifest, nil
						}
						return previousManifest, nil
					}

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

				It("generate manifest without arbitrary params", func() {
					Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(2))
					_, _, passedRequestParams, _, _, _ := manifestGenerator.GenerateManifestArgsForCall(0)
					Expect(passedRequestParams).To(BeEmpty())
				})

				It("generates new manifest with arbitrary params", func() {
					Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(2))
					_, _, passedRequestParams, _, _, _ := manifestGenerator.GenerateManifestArgsForCall(1)
					Expect(passedRequestParams).To(Equal(requestParams))
				})

				It("Creates a bosh deployment from the generated manifest", func() {
					Expect(boshClient.DeployCallCount()).To(Equal(1))
					deployedManifest, _, _ := boshClient.DeployArgsForCall(0)
					Expect(string(deployedManifest)).To(Equal(string(manifest)))
				})

				It("returns the bosh task ID", func() {
					Expect(returnedTaskID).To(Equal(boshTaskID))
				})

				Context("and there are no parameters configured", func() {
					BeforeEach(func() {
						requestParams = map[string]interface{}{}
					})

					It("doesn't return an error", func() {
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
					) (task.BoshManifest, error) {
						if len(requestParams) > 0 {
							return nil, errors.New("manifest fail")
						}
						return previousManifest, nil
					}
				})

				It("wraps the error", func() {
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
				Expect(deployError).To(Equal(broker.NewTaskError(errors.New("pending changes detected"), broker.ApplyChangesWithPendingChanges)))
				Expect(boshClient.DeployCallCount()).To(BeZero())
			})

			Describe("when apply-changes is", func() {
				BeforeEach(func() {
					featureFlags.CFUserTriggeredUpgradesReturns(true)
				})

				Context("set to true", func() {
					BeforeEach(func() {
						requestParams["parameters"] = map[string]interface{}{}
						applyPendingChanges = true
					})

					It("performs deploy", func() {
						Expect(deployError).NotTo(HaveOccurred())
						Expect(boshClient.DeployCallCount()).To(Equal(1))
					})

					Context("and user triggered upgrades are disabled", func() {
						BeforeEach(func() {
							featureFlags.CFUserTriggeredUpgradesReturns(false)
						})

						It("fails without deploying", func() {
							Expect(deployError).To(Equal(broker.NewApplyChangesNotPermittedError(errors.New("'cf_user_triggered_upgrades' feature is disabled"))))
							Expect(boshClient.DeployCallCount()).To(BeZero())
						})
					})

					Context("when changing plans", func() {
						BeforeEach(func() {
							planID = secondPlanID
						})

						It("fails without deploying", func() {
							Expect(deployError).To(Equal(broker.NewTaskError(errors.New("update called with apply-changes and a plan change"), broker.ApplyChangesWithPlanChange)))
							Expect(boshClient.DeployCallCount()).To(BeZero())
						})
					})

					Context("when passing in arbitrary params", func() {
						BeforeEach(func() {
							requestParams["parameters"] = map[string]interface{}{
								"foo": "bar",
							}
						})

						It("fails without deploying", func() {
							Expect(deployError).To(Equal(broker.NewTaskError(errors.New("update called with apply-changes and arbitrary parameters set"), broker.ApplyChangesWithParams)))
							Expect(boshClient.DeployCallCount()).To(BeZero())
						})
					})
				})

				Context("set to false", func() {
					It("fails without deploying", func() {
						Expect(deployError).To(Equal(broker.NewTaskError(errors.New("pending changes detected"), broker.ApplyChangesWithPendingChanges)))
						Expect(boshClient.DeployCallCount()).To(BeZero())
					})
				})

				Context("not set", func() {
					BeforeEach(func() {
						requestParams["parameters"] = map[string]interface{}{"foo": "bar"}
					})

					It("fails without deploying", func() {
						Expect(deployError).To(Equal(broker.NewTaskError(errors.New("pending changes detected"), broker.ApplyChangesWithPendingChanges)))
						Expect(boshClient.DeployCallCount()).To(BeZero())
					})
				})
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

		Context("and when the last bosh task for deployment fails to fetch", func() {
			BeforeEach(func() {
				boshClient.GetTasksReturns(nil, errors.New("connection error"))
			})

			It("wraps the error", func() {
				Expect(deployError).To(MatchError(fmt.Sprintf("error getting tasks for deployment %s: connection error\n", deploymentName)))
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
	})
})

func stringPointer(s string) *string {
	return &s
}
