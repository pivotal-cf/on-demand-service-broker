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
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/credhub"
	"github.com/pivotal-cf/on-demand-service-broker/task"
	"github.com/pivotal-cf/on-demand-service-broker/task/fakes"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("Deployer", func() {
	const boshTaskID = 42

	var (
		boshClient    *fakes.FakeBoshClient
		deployer      task.Deployer
		boshContextID string

		deployedManifest []byte
		deployError      error
		returnedTaskID   int

		planID            string
		previousPlanID    *string
		requestParams     map[string]interface{}
		copyParams        map[string]interface{}
		generatedManifest string
		oldManifest       []byte
		secretsMap        map[string]string
		configsMap        map[string]string
		boshConfigs       []boshdirector.BoshConfig

		manifestGenerator *fakes.FakeManifestGenerator
		odbSecrets        *fakes.FakeODBSecrets
		bulkSetter        *fakes.FakeBulkSetter
	)

	BeforeEach(func() {
		boshClient = new(fakes.FakeBoshClient)
		manifestGenerator = new(fakes.FakeManifestGenerator)
		odbSecrets = new(fakes.FakeODBSecrets)
		bulkSetter = new(fakes.FakeBulkSetter)
		deployer = task.NewDeployer(boshClient, manifestGenerator, odbSecrets, bulkSetter)

		planID = existingPlanID
		previousPlanID = nil

		requestParams = map[string]interface{}{
			"parameters": map[string]interface{}{"foo": "bar"},
			"context": map[string]interface{}{
				"platform": "cloudfoundry",
			},
		}

		generatedManifest = "name: a-manifest"
		boshContextID = ""

		manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{Manifest: generatedManifest}, nil)
		odbSecrets.ReplaceODBRefsStub = func(m string, s []broker.ManifestSecret) string {
			return m
		}
		secretsMap = map[string]string{
			"((foo))": "b4r",
			"o0p5":    "p4ssw0rd",
		}
		configsMap = map[string]string{
			"some-config-type": "some-config-content",
		}
		boshConfigs = []boshdirector.BoshConfig{
			boshdirector.BoshConfig{Type: "some-config-type", Name: "some-config-name", Content: "some-config-content"},
		}
	})

	Describe("Create", func() {
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
			copyParams = make(map[string]interface{})
			for k, v := range requestParams {
				copyParams[k] = v
			}
		})

		When("there are configs to be applied", func() {
			var (
				boshConfigs serviceadapter.BOSHConfigs
			)

			BeforeEach(func() {
				boshConfigs = serviceadapter.BOSHConfigs{
					"config-type": "config-content",
				}

				manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{
					Manifest: generatedManifest,
					Configs:  boshConfigs,
				}, nil)
			})

			It("applies the configs to bosh", func() {
				Expect(boshClient.UpdateConfigCallCount()).To(Equal(1))

				configType, configName, configContent, _ := boshClient.UpdateConfigArgsForCall(0)
				Expect(configType).To(Equal("config-type"))
				Expect(configName).To(Equal(deploymentName))
				Expect(configContent).To(Equal([]byte("config-content")))
			})

			When("fails to update bosh configs", func() {
				BeforeEach(func() {
					boshClient.UpdateConfigReturns(errors.New("some-error"))
				})

				It("returns an error", func() {
					Expect(deployError).To(MatchError(ContainSubstring("some-error")))
				})
			})

			When("bosh configs is disabled on the deployer", func() {
				BeforeEach(func() {
					deployer.DisableBoshConfigs = true
				})

				It("doesn't call UpdateConfig", func() {
					Expect(boshClient.UpdateConfigCallCount()).To(Equal(0))
				})
			})
		})

		When("there are secrets to be stored", func() {
			var (
				managedSecrets    serviceadapter.ODBManagedSecrets
				manifestSecrets   []broker.ManifestSecret
				manifestWithPaths string
			)

			BeforeEach(func() {
				managedSecrets = serviceadapter.ODBManagedSecrets{
					"secret_foo":         "value_of_foo",
					"another_credential": "value_of_that",
				}

				manifestSecrets = []broker.ManifestSecret{
					{Name: "secret_foo", Value: "value_of_foo", Path: "/odb/path/foo"},
					{Name: "another_credential", Value: "value_of_that", Path: "/odb/path/cred"},
				}

				manifestWithRefs := "name: ((odb_secret:secret_foo))\ncred: ((odb_secret:another_credential))"
				manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{
					Manifest:          manifestWithRefs,
					ODBManagedSecrets: managedSecrets,
				}, nil)

				manifestWithPaths = "name: ((/odb/path/foo))\ncred: ((/odb/path/cred))"
				odbSecrets.GenerateSecretPathsReturns(manifestSecrets)
				odbSecrets.ReplaceODBRefsReturns(manifestWithPaths)
			})

			It("stores the secrets on credhub", func() {
				Expect(odbSecrets.GenerateSecretPathsCallCount()).To(Equal(1))

				By("generating the secret paths")
				actualDeploymentName, _, actualSecrets := odbSecrets.GenerateSecretPathsArgsForCall(0)
				Expect(actualDeploymentName).To(Equal(deploymentName))
				Expect(actualSecrets).To(Equal(managedSecrets))

				By("calling bulkset")
				Expect(bulkSetter.BulkSetCallCount()).To(Equal(1))
				actualManifestSecrets := bulkSetter.BulkSetArgsForCall(0)
				Expect(actualManifestSecrets).To(Equal(manifestSecrets))

				By("substituting the odb_secret marker with the credhub path")
				deployedManifest, _, _, _ := boshClient.DeployArgsForCall(0)
				Expect(string(deployedManifest)).To(Equal(manifestWithPaths))
			})

			It("errors when fail to store the secret", func() {
				bulkSetter.BulkSetReturns(errors.New("what is this?"))
				_, _, deployError = deployer.Create(deploymentName, planID, requestParams, boshContextID, logger)
				Expect(deployError).To(MatchError(ContainSubstring("what is this?")))
			})

			When("Bosh credhub is not configured/enabled", func() {
				BeforeEach(func() {
					var b *credhub.Store
					deployer = task.NewDeployer(boshClient, manifestGenerator, odbSecrets, b)
				})

				It("doesn't error", func() {
					_, _, deployError = deployer.Create(deploymentName, planID, requestParams, boshContextID, logger)
					Expect(deployError).ToNot(HaveOccurred())

					Expect(bulkSetter.BulkSetCallCount()).To(Equal(0))
				})
			})
		})

		Context("when bosh deploys the release successfully", func() {
			BeforeEach(func() {
				By("not having any previous tasks")
				boshClient.GetTasksReturns([]boshdirector.BoshTask{}, nil)
				manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{Manifest: generatedManifest}, nil)
				boshClient.DeployReturns(42, nil)
			})

			It("checks tasks for the deployment", func() {
				Expect(boshClient.GetTasksCallCount()).To(Equal(1))
				actualDeploymentName, _ := boshClient.GetTasksArgsForCall(0)
				Expect(actualDeploymentName).To(Equal(deploymentName))
			})

			It("returns the bosh task ID", func() {
				Expect(returnedTaskID).To(Equal(boshTaskID))
			})

			It("Creates a bosh deployment using provided manifest", func() {
				Expect(boshClient.DeployCallCount()).To(Equal(1))
				deployedManifest, _, _, _ := boshClient.DeployArgsForCall(0)
				Expect(string(deployedManifest)).To(Equal(generatedManifest))
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
			})

			It("logs the bosh task ID returned by the director", func() {
				Expect(deployError).ToNot(HaveOccurred())
				Expect(returnedTaskID).To(Equal(42))
				Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("Bosh task ID for create deployment %s is %d", deploymentName, boshTaskID)))
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
				Expect(deployError).To(BeAssignableToTypeOf(broker.TaskInProgressError{}))

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
				Expect(deployError).To(BeAssignableToTypeOf(broker.TaskInProgressError{}))

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

	Describe("Upgrade", func() {
		BeforeEach(func() {
			oldManifest = []byte("---\nold-manifest-fetched-from-bosh: bar")
			previousPlanID = stringPointer(existingPlanID)

			boshClient.GetDeploymentReturns(oldManifest, true, nil)
			boshClient.GetTasksReturns([]boshdirector.BoshTask{}, nil)
			boshClient.DeployReturns(42, nil)
		})

		Context("when bosh deploys the release successfully", func() {
			BeforeEach(func() {
				By("not having any previous tasks")
				boshClient.GetDeploymentReturns(oldManifest, true, nil)
				boshClient.GetTasksReturns([]boshdirector.BoshTask{}, nil)
				boshClient.DeployReturns(42, nil)
			})

			It("starts upgrading successfully", func() {
				returnedTaskID, deployedManifest, deployError = deployer.Upgrade(
					deploymentName,
					plan,
					boshContextID,
					logger,
				)

				By("not returning an error")
				Expect(deployError).NotTo(HaveOccurred())

				By("returning the bosh task ID")
				Expect(returnedTaskID).To(Equal(boshTaskID))

				By("return the newly generated manifest")
				Expect(string(deployedManifest)).To(Equal(generatedManifest))

				By("checking that the deployment exists")
				Expect(boshClient.GetDeploymentCallCount()).To(Equal(1))
				actualDeploymentName, _ := boshClient.GetDeploymentArgsForCall(0)
				Expect(actualDeploymentName).To(Equal(deploymentName))

				By("checking tasks for the deployment")
				Expect(boshClient.GetTasksCallCount()).To(Equal(1))
				actualDeploymentName, _ = boshClient.GetTasksArgsForCall(0)
				Expect(actualDeploymentName).To(Equal(deploymentName))

				By("Creating a bosh deployment using generated manifest")
				Expect(boshClient.DeployCallCount()).To(Equal(1))
				deployedManifest, _, _, _ := boshClient.DeployArgsForCall(0)
				Expect(string(deployedManifest)).To(Equal(generatedManifest))

				By("logging the bosh task ID returned by the director")
				Expect(logBuffer.String()).To(ContainSubstring(fmt.Sprintf("Bosh task ID for upgrade deployment %s is %d", deploymentName, boshTaskID)))
			})

			Context("when bosh context ID is provided", func() {
				BeforeEach(func() {
					boshContextID = "bosh-context-id"
				})

				It("invokes boshdirector's Create with context ID", func() {
					returnedTaskID, deployedManifest, deployError = deployer.Upgrade(
						deploymentName,
						plan,
						boshContextID,
						logger,
					)

					Expect(boshClient.DeployCallCount()).To(Equal(1))
					_, actualBoshContextID, _, _ := boshClient.DeployArgsForCall(0)
					Expect(actualBoshContextID).To(Equal(boshContextID))
				})
			})
		})

		Context("when the deployment cannot be found", func() {
			BeforeEach(func() {
				boshClient.GetDeploymentReturns(nil, false, nil)
			})

			It("returns a deployment not found error", func() {
				returnedTaskID, deployedManifest, deployError = deployer.Upgrade(
					deploymentName,
					plan,
					boshContextID,
					logger,
				)

				Expect(deployError).To(MatchError(ContainSubstring("not found")))
				Expect(boshClient.DeployCallCount()).To(Equal(0))
			})
		})

		Context("when getting the deployment fails", func() {
			BeforeEach(func() {
				boshClient.GetDeploymentReturns(nil, false, errors.New("error getting deployment"))
			})

			It("returns a deployment not found error", func() {
				returnedTaskID, deployedManifest, deployError = deployer.Upgrade(
					deploymentName,
					plan,
					boshContextID,
					logger,
				)

				Expect(deployError).To(MatchError(errors.New("error getting deployment")))
				Expect(boshClient.DeployCallCount()).To(Equal(0))
			})
		})

		It("returns the bosh task ID and new manifest", func() {
			returnedTaskID, deployedManifest, deployError = deployer.Upgrade(
				deploymentName,
				plan,
				boshContextID,
				logger,
			)

			Expect(returnedTaskID).To(Equal(42))
			Expect(string(deployedManifest)).To(Equal(generatedManifest))
			Expect(deployError).NotTo(HaveOccurred())
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

			It("returns and logs an error", func() {
				returnedTaskID, deployedManifest, deployError = deployer.Upgrade(
					deploymentName,
					plan,
					boshContextID,
					logger,
				)

				Expect(deployError).To(BeAssignableToTypeOf(broker.TaskInProgressError{}))
				Expect(logBuffer.String()).To(ContainSubstring(
					fmt.Sprintf("deployment %s is still in progress: tasks %s\n",
						deploymentName,
						boshdirector.BoshTasks{queuedTask}.ToLog(),
					),
				))

				By("does not log the previous completed tasks for the deployment")
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

			It("returns and logs the error", func() {
				boshClient.GetTasksReturns([]boshdirector.BoshTask{
					inProgressTask,
					{State: boshdirector.TaskDone, ID: previousDoneBoshTaskID},
					{State: boshdirector.TaskError, ID: previousErrorBoshTaskID},
				}, nil)

				returnedTaskID, deployedManifest, deployError = deployer.Upgrade(
					deploymentName,
					plan,
					boshContextID,
					logger,
				)

				Expect(deployError).To(BeAssignableToTypeOf(broker.TaskInProgressError{}))
				Expect(logBuffer.String()).To(ContainSubstring(
					fmt.Sprintf("deployment %s is still in progress: tasks %s\n",
						deploymentName,
						boshdirector.BoshTasks{inProgressTask}.ToLog(),
					),
				))

				By("does not log the previous tasks for the deployment")
				Expect(logBuffer.String()).NotTo(ContainSubstring("done"))
				Expect(logBuffer.String()).NotTo(ContainSubstring("\"ID\":%d", previousDoneBoshTaskID))
				Expect(logBuffer.String()).NotTo(ContainSubstring("error"))
				Expect(logBuffer.String()).NotTo(ContainSubstring("\"ID\":%d", previousErrorBoshTaskID))
			})
		})

		Context("when the last bosh task for deployment fails to fetch", func() {
			It("wraps the error", func() {
				boshClient.GetTasksReturns(nil, errors.New("connection error"))

				returnedTaskID, deployedManifest, deployError = deployer.Upgrade(
					deploymentName,
					plan,
					boshContextID,
					logger,
				)

				Expect(deployError).To(MatchError(fmt.Sprintf("error getting tasks for deployment %s: connection error\n", deploymentName)))
			})
		})

		Context("when bosh fails to deploy the release", func() {
			It("wraps the error", func() {
				boshClient.GetTasksReturns([]boshdirector.BoshTask{{State: boshdirector.TaskDone}}, nil)
				boshClient.DeployReturns(0, errors.New("error deploying"))

				returnedTaskID, deployedManifest, deployError = deployer.Upgrade(
					deploymentName,
					plan,
					boshContextID,
					logger,
				)

				Expect(deployError).To(MatchError(ContainSubstring("error deploying")))
			})
		})

		Context("when there are bosh configs", func() {
			It("sends the old configs to service adapter", func() {
				boshClient.GetConfigsReturns(boshConfigs, nil)

				returnedTaskID, deployedManifest, deployError = deployer.Upgrade(
					deploymentName,
					plan,
					boshContextID,
					logger,
				)

				generateManifestProps, _ := manifestGenerator.GenerateManifestArgsForCall(0)
				Expect(generateManifestProps.PreviousConfigs).To(Equal(configsMap))
			})
		})

		Context("when getting bosh configs fails", func() {
			It("wraps the error", func() {
				boshClient.GetConfigsReturns(nil, errors.New("some-error"))

				returnedTaskID, deployedManifest, deployError = deployer.Upgrade(
					deploymentName,
					plan,
					boshContextID,
					logger,
				)

				Expect(deployError).To(MatchError(ContainSubstring("some-error")))
			})
		})

		Context("bosh configs is disabled on the deployer", func() {
			It("doesn't call UpdateConfig or GetConfigs", func() {
				deployer.DisableBoshConfigs = true
				boshClient.DeployReturns(boshTaskID, nil)

				returnedTaskID, deployedManifest, deployError = deployer.Upgrade(
					deploymentName,
					plan,
					boshContextID,
					logger,
				)

				Expect(deployError).NotTo(HaveOccurred())
				Expect(returnedTaskID).To(Equal(boshTaskID))
				Expect(boshClient.UpdateConfigCallCount()).To(Equal(0), "UpdateConfig shouldn't be called")
				Expect(boshClient.GetConfigsCallCount()).To(Equal(0), "GetConfigs shouldn't be called")
			})
		})
	})

	Describe("Update", func() {
		BeforeEach(func() {
			oldManifest = []byte("---\nname: a-manifest\nupdate:\n canaries: 5\n max_in_flight: 1")

			previousPlanID = stringPointer(existingPlanID)

			boshClient.GetTasksReturns([]boshdirector.BoshTask{{State: boshdirector.TaskDone}}, nil)
			boshClient.GetDeploymentReturns(oldManifest, true, nil)
		})

		It("pass the context to the service adapter", func() {
			params := map[string]interface{}{
				"parameters": map[string]interface{}{"foo": "bar"},
				"context": map[string]interface{}{
					"platform": "cloudfoundry",
				},
			}
			copyParams = make(map[string]interface{})
			for k, v := range params {
				copyParams[k] = v
			}
			_, _, err := deployer.Update(
				deploymentName,
				planID,
				params,
				previousPlanID,
				boshContextID,
				secretsMap,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(2))
			generateManifestProps, _ := manifestGenerator.GenerateManifestArgsForCall(1)
			Expect(generateManifestProps.RequestParams).To(Equal(copyParams))
		})

		Context("passing secret map", func() {
			It("manifest regeneration is passed the secrets map", func() {
				_, _, err := deployer.Update(
					deploymentName,
					planID,
					nil,
					previousPlanID,
					boshContextID,
					secretsMap,
					logger,
				)
				Expect(err).NotTo(HaveOccurred())

				Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(2))
				for i := 0; i < 2; i++ {
					generateManifestProps, _ := manifestGenerator.GenerateManifestArgsForCall(i)
					Expect(generateManifestProps.SecretsMap).To(Equal(secretsMap), fmt.Sprintf("call %d", i+1))
				}
			})
		})

		Context("and the manifest generator fails to generate the manifest the first time", func() {
			BeforeEach(func() {
				manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{}, errors.New("manifest fail"))
			})

			It("wraps the error", func() {
				returnedTaskID, deployedManifest, deployError = deployer.Update(
					deploymentName,
					planID,
					requestParams,
					previousPlanID,
					boshContextID,
					secretsMap,
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
						generateManifestProps task.GenerateManifestProperties,
						_ *log.Logger,
					) (serviceadapter.MarshalledGenerateManifest, error) {
						if len(requestParams) > 0 {
							return serviceadapter.MarshalledGenerateManifest{Manifest: generatedManifest}, nil
						}
						return serviceadapter.MarshalledGenerateManifest{Manifest: string(generateManifestProps.OldManifest)}, nil
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
						secretsMap,
						logger,
					)

					Expect(boshClient.GetTasksCallCount()).To(Equal(1))
					actualDeploymentName, _ := boshClient.GetTasksArgsForCall(0)
					Expect(actualDeploymentName).To(Equal(deploymentName))

					Expect(boshClient.GetDeploymentCallCount()).To(Equal(1))
					actualDeploymentName, _ = boshClient.GetDeploymentArgsForCall(0)
					Expect(actualDeploymentName).To(Equal(deploymentName))

					Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(2))

					generateManifestProps, _ := manifestGenerator.GenerateManifestArgsForCall(0)
					Expect(generateManifestProps.RequestParams).To(BeEmpty())

					generateManifestProps, _ = manifestGenerator.GenerateManifestArgsForCall(1)
					Expect(generateManifestProps.RequestParams).To(Equal(requestParams))

					Expect(boshClient.DeployCallCount()).To(Equal(1))
					deployedManifest, _, _, _ := boshClient.DeployArgsForCall(0)
					Expect(string(deployedManifest)).To(Equal(string(generatedManifest)))

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
							secretsMap,
							logger,
						)

						Expect(deployError).NotTo(HaveOccurred())
					})
				})
			})

			Context("and the manifest generator fails to generate the manifest the second time", func() {
				BeforeEach(func() {
					manifestGenerator.GenerateManifestStub = func(
						generateManifestProps task.GenerateManifestProperties,
						_ *log.Logger,
					) (serviceadapter.MarshalledGenerateManifest, error) {
						if len(requestParams) > 0 {
							return serviceadapter.MarshalledGenerateManifest{}, errors.New("manifest fail")
						}
						return serviceadapter.MarshalledGenerateManifest{Manifest: string(generateManifestProps.OldManifest)}, nil
					}
				})

				It("wraps the error", func() {
					returnedTaskID, deployedManifest, deployError = deployer.Update(
						deploymentName,
						planID,
						requestParams,
						previousPlanID,
						boshContextID,
						secretsMap,
						logger,
					)

					Expect(boshClient.DeployCallCount()).To(Equal(0))
					Expect(deployError).To(MatchError(ContainSubstring("manifest fail")))
				})
			})
		})

		Context("and there are pending changes", func() {
			BeforeEach(func() {
				manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{Manifest: "name: other-name"}, nil)
			})

			It("fails without deploying", func() {
				returnedTaskID, deployedManifest, deployError = deployer.Update(
					deploymentName,
					planID,
					requestParams,
					previousPlanID,
					boshContextID,
					secretsMap,
					logger,
				)

				Expect(deployError).To(HaveOccurred())
				Expect(deployError).To(BeAssignableToTypeOf(broker.PendingChangesNotAppliedError{}))
				Expect(boshClient.DeployCallCount()).To(BeZero())
			})
		})

		It("fails when previous manifest contains odb managed secret syntax", func() {
			manifestWithSecrets := serviceadapter.MarshalledGenerateManifest{
				Manifest: "name: a-manifest\nproperties:\n  password: ((odb_secret:the_password))",
				ODBManagedSecrets: serviceadapter.ODBManagedSecrets{
					"the_password": "foo",
				},
			}
			manifestWithInterpolatedSecrets := "name: a-manifest\nproperties:\n  password: ((/odb/foo/bar/the_password))"
			manifestSecrets := []broker.ManifestSecret{
				{Name: "the_password", Value: "foo", Path: "/odb/foo/bar/the_password"},
			}

			manifestGenerator.GenerateManifestReturns(manifestWithSecrets, nil)
			boshClient.GetDeploymentReturns([]byte(manifestWithInterpolatedSecrets), true, nil)
			odbSecrets.GenerateSecretPathsReturns(manifestSecrets)
			odbSecrets.ReplaceODBRefsReturns(manifestWithInterpolatedSecrets)

			_, deployedManifest, deployError = deployer.Update(
				deploymentName,
				planID,
				requestParams,
				previousPlanID,
				boshContextID,
				secretsMap,
				logger,
			)

			Expect(deployError).To(MatchError("There are pending changes"))
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
					secretsMap,
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
					secretsMap,
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
					secretsMap,
					logger,
				)

				Expect(deployError).To(MatchError(errors.New("error getting deployment")))
				Expect(boshClient.DeployCallCount()).To(Equal(0))
			})
		})

		Context("when there are bosh configs", func() {
			BeforeEach(func() {
				boshClient.GetConfigsReturns(boshConfigs, nil)
			})

			JustBeforeEach(func() {
				returnedTaskID, deployedManifest, deployError = deployer.Update(
					deploymentName,
					planID,
					requestParams,
					previousPlanID,
					boshContextID,
					secretsMap,
					logger,
				)
			})

			It("sends the old configs to service adapter", func() {
				generateManifestProps, _ := manifestGenerator.GenerateManifestArgsForCall(0)
				Expect(generateManifestProps.PreviousConfigs).To(Equal(configsMap))
			})
		})

		Context("when getting bosh configs fails", func() {
			BeforeEach(func() {
				boshClient.GetConfigsReturns(nil, errors.New("some-error"))
			})

			JustBeforeEach(func() {
				returnedTaskID, deployedManifest, deployError = deployer.Update(
					deploymentName,
					planID,
					requestParams,
					previousPlanID,
					boshContextID,
					secretsMap,
					logger,
				)
			})

			It("wraps the error", func() {
				Expect(deployError).To(MatchError(ContainSubstring("some-error")))
			})
		})

		When("bosh configs is disabled on the deployer", func() {
			BeforeEach(func() {
				deployer.DisableBoshConfigs = true
				boshClient.DeployReturns(boshTaskID, nil)
			})

			It("doesn't call UpdateConfig or GetConfigs", func() {
				returnedTaskID, _, deployError = deployer.Update(
					deploymentName,
					planID,
					requestParams,
					previousPlanID,
					boshContextID,
					secretsMap,
					logger,
				)

				Expect(deployError).NotTo(HaveOccurred())
				Expect(returnedTaskID).To(Equal(boshTaskID))
				Expect(boshClient.UpdateConfigCallCount()).To(Equal(0), "UpdateConfig shouldn't be called")
				Expect(boshClient.GetConfigsCallCount()).To(Equal(0), "GetConfigs shouldn't be called")
			})
		})

		It("ignores the update block when the manifest generator generates a new manifest with a different update block", func() {
			previousPlanID = stringPointer(existingPlanID)

			generatedManifest := []byte("---\nname: a-manifest\nupdate:\n canaries: 2\n max_in_flight: 1")

			requestParams = map[string]interface{}{"foo": "bar"}

			manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{Manifest: string(generatedManifest)}, nil)
			boshClient.DeployReturns(42, nil)

			returnedTaskID, deployedManifest, deployError = deployer.Update(
				deploymentName,
				planID,
				requestParams,
				previousPlanID,
				boshContextID,
				secretsMap,
				logger,
			)

			Expect(deployError).To(BeNil())

			Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(2))
			generateManifestProps, _ := manifestGenerator.GenerateManifestArgsForCall(1)
			Expect(generateManifestProps.RequestParams).To(Equal(requestParams))

			manifestToDeploy, _, _, _ := boshClient.DeployArgsForCall(0)
			Expect(string(manifestToDeploy)).To(Equal(string(generatedManifest)))
		})

		It("ignores the update block in an instance_group when the manifest generator generates a new manifest with a different update block", func() {
			previousPlanID = stringPointer(existingPlanID)
			oldManifest = []byte(`
---
name: a-manifest
instance_groups:
- name: foo
update:
  canaries: 5
  max_in_flight: 1
`)

			boshClient.GetDeploymentReturns(oldManifest, true, nil)
			generatedManifest := `
name: a-manifest
instance_groups:
- name: foo
  update:
    canaries: 4
    max_in_flight: 10
`
			requestParams = map[string]interface{}{"foo": "bar"}

			manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{Manifest: string(generatedManifest)}, nil)
			boshClient.DeployReturns(42, nil)

			returnedTaskID, deployedManifest, deployError = deployer.Update(
				deploymentName,
				planID,
				requestParams,
				previousPlanID,
				boshContextID,
				secretsMap,
				logger,
			)

			Expect(deployError).To(BeNil())

			Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(2))
			generateManifestProps, _ := manifestGenerator.GenerateManifestArgsForCall(1)
			Expect(generateManifestProps.RequestParams).To(Equal(requestParams))

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
			manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{Manifest: string(generatedManifest)}, nil)

			_, _, deployError = deployer.Update(
				deploymentName,
				planID,
				requestParams,
				previousPlanID,
				boshContextID,
				secretsMap,
				logger,
			)

			Expect(deployError).To(HaveOccurred())
			Expect(deployError).To(BeAssignableToTypeOf(broker.PendingChangesNotAppliedError{}))
			Expect(boshClient.DeployCallCount()).To(BeZero())
		})

		It("detects changes to the features block in a manifest and prevents deployment", func() {
			oldManifest = []byte(`---
features:
  use_short_dns_addresses: true
`)
			boshClient.GetDeploymentReturns(oldManifest, true, nil)
			previousPlanID = stringPointer(existingPlanID)

			generatedManifest := []byte(`---
features:
  use_short_dns_addresses: false
`)
			manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{Manifest: string(generatedManifest)}, nil)

			_, _, deployError = deployer.Update(
				deploymentName,
				planID,
				requestParams,
				previousPlanID,
				boshContextID,
				secretsMap,
				logger,
			)

			Expect(deployError).To(HaveOccurred())
			Expect(deployError).To(BeAssignableToTypeOf(broker.PendingChangesNotAppliedError{}))
			Expect(boshClient.DeployCallCount()).To(BeZero())
		})

		It("detects 'extra' changes to the features block in a manifest and prevents deployment", func() {
			oldManifest = []byte(`---
features:
  some_undocumented_feature: 41
`)
			boshClient.GetDeploymentReturns(oldManifest, true, nil)
			previousPlanID = stringPointer(existingPlanID)

			generatedManifest := []byte(`---
features:
  some_undocumented_feature: 42
`)
			manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{Manifest: string(generatedManifest)}, nil)

			_, _, deployError = deployer.Update(
				deploymentName,
				planID,
				requestParams,
				previousPlanID,
				boshContextID,
				secretsMap,
				logger,
			)

			Expect(deployError).To(HaveOccurred())
			Expect(deployError).To(BeAssignableToTypeOf(broker.PendingChangesNotAppliedError{}))
			Expect(boshClient.DeployCallCount()).To(BeZero())
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
			manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{Manifest: string(generatedManifest)}, nil)

			_, _, deployError = deployer.Update(
				deploymentName,
				planID,
				requestParams,
				previousPlanID,
				boshContextID,
				secretsMap,
				logger,
			)

			Expect(deployError).To(HaveOccurred())
			Expect(deployError).To(BeAssignableToTypeOf(broker.PendingChangesNotAppliedError{}))
			Expect(boshClient.DeployCallCount()).To(BeZero())
		})

		It("returns an error when old manifest contains invalid YAML", func() {
			previousPlanID = stringPointer(existingPlanID)

			oldManifestWithInvalidYAML := []byte("{")
			generatedManifest := []byte("---\nupdate:\n canaries: 2\n max_in_flight: 1")

			requestParams = map[string]interface{}{"foo": "bar"}

			boshClient.GetDeploymentReturns(oldManifestWithInvalidYAML, true, nil)
			manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{Manifest: string(generatedManifest)}, nil)
			boshClient.DeployReturns(42, nil)

			returnedTaskID, deployedManifest, deployError = deployer.Update(
				deploymentName,
				planID,
				requestParams,
				previousPlanID,
				boshContextID,
				secretsMap,
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
			manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{Manifest: string(generatedManifestWithInvalidYAML)}, nil)
			boshClient.DeployReturns(42, nil)

			returnedTaskID, deployedManifest, deployError = deployer.Update(
				deploymentName,
				planID,
				requestParams,
				previousPlanID,
				boshContextID,
				secretsMap,
				logger,
			)

			Expect(deployError.Error()).To(ContainSubstring("error detecting change in manifest, unable to unmarshal manifest"))
			Expect(manifestGenerator.GenerateManifestCallCount()).To(Equal(1))
		})
	})

	Describe("Recreate", func() {
		var err error

		It("calls BOSH recreate", func() {
			boshClient.GetTasksReturns([]boshdirector.BoshTask{}, nil)
			boshClient.RecreateReturns(42, nil)

			returnedTaskID, err = deployer.Recreate(deploymentName, planID, boshContextID, logger)

			Expect(err).NotTo(HaveOccurred())
			Expect(boshClient.RecreateCallCount()).To(Equal(1), "recreate was not called once")
			Expect(logBuffer.String()).To(ContainSubstring("Submitted BOSH recreate with task ID"))
		})

		It("fails when it can't get bosh in progess tasks", func() {
			boshClient.GetTasksReturns([]boshdirector.BoshTask{}, fmt.Errorf("boom!"))
			_, err = deployer.Recreate(deploymentName, planID, boshContextID, logger)
			Expect(err).To(MatchError(ContainSubstring("error getting tasks for deployment")))
		})

		It("fails if an operation is in progress", func() {
			boshClient.GetTasksReturns([]boshdirector.BoshTask{{
				State: "processing",
			}}, nil)

			_, err = deployer.Recreate(deploymentName, planID, boshContextID, logger)
			Expect(err).To(MatchError("task in progress"))
		})

		It("fails when bosh recreate returns an error", func() {
			boshClient.GetTasksReturns([]boshdirector.BoshTask{}, nil)
			boshClient.RecreateReturns(0, errors.New("zork"))

			_, err = deployer.Recreate(deploymentName, planID, boshContextID, logger)
			Expect(err).To(MatchError(ContainSubstring("zork")))

			Expect(logBuffer.String()).To(ContainSubstring("failed to recreate deployment"))
		})

	})
})

func stringPointer(s string) *string {
	return &s
}
