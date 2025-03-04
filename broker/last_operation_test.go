// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"code.cloudfoundry.org/brokerapi/v13/domain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
)

var _ = Describe("LastOperation", func() {
	Context("failures", func() {
		var (
			instanceID    = "a-useful-instance"
			pollDetails   domain.PollDetails
			operationData string

			lastOpErr error
			opResult  domain.LastOperation
		)

		JustBeforeEach(func() {
			b = createDefaultBroker()
			pollDetails = domain.PollDetails{
				OperationData: operationData,
			}
			opResult, lastOpErr = b.LastOperation(context.Background(), instanceID, pollDetails)
		})

		Context("when task cannot be retrieved from BOSH", func() {
			BeforeEach(func() {
				operationData = `{"BoshTaskID": 42, "OperationType": "create"}`
				boshClient.GetTaskReturns(boshdirector.BoshTask{}, errors.New("something went wrong!"))
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("error retrieving tasks from bosh, for deployment 'service-instance_a-useful-instance': something went wrong!"))
			})

			Describe("returned error", func() {
				It("includes a standard message", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information:",
					)))
				})

				It("includes the broker request id", func() {
					Expect(lastOpErr).To(MatchError(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					)))
				})

				It("includes the service name", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						"service: a-cool-redis-service",
					)))
				})

				It("includes the service instance guid", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					)))
				})

				It("includes the operation type", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						"operation: create",
					)))
				})

				It("includes the bosh task id", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						fmt.Sprintf("task-id: %d", 42),
					)))
				})
			})
		})

		Context("when there is no operation data present in the request", func() {
			BeforeEach(func() {
				operationData = ""
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("Request missing operation data, please check your Cloud Foundry version is v238+"))
			})

			Describe("returned error", func() {
				It("includes a standard message", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information:",
					)))
				})

				It("includes the broker request id", func() {
					Expect(lastOpErr).To(MatchError(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					)))
				})

				It("includes the service name", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						"service: a-cool-redis-service",
					)))
				})

				It("includes the service instance guid", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					)))
				})

				It("does not include the operation type", func() {
					Expect(lastOpErr).NotTo(MatchError(ContainSubstring(
						"operation:",
					)))
				})

				It("does NOT include the bosh task id", func() {
					Expect(lastOpErr).NotTo(MatchError(ContainSubstring(
						"task-id:",
					)))
				})
			})
		})

		Context("when there is no bosh task ID in the operation data", func() {
			BeforeEach(func() {
				operationData = `{"OperationType": "create"}`
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("no task ID found in operation data"))
			})

			Describe("returned error", func() {
				It("includes a standard message", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information:",
					)))
				})

				It("includes the broker request id", func() {
					Expect(lastOpErr).To(MatchError(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					)))
				})

				It("includes the service name", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						"service: a-cool-redis-service",
					)))
				})

				It("includes the service instance guid", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					)))
				})

				It("includes the operation type", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						"operation: create",
					)))
				})

				It("does NOT include the bosh task id", func() {
					Expect(lastOpErr).NotTo(MatchError(ContainSubstring(
						"task-id:",
					)))
				})
			})
		})

		Context("when operation data cannot be parsed", func() {
			BeforeEach(func() {
				operationData = "I'm not JSON"
			})

			It("logs the error", func() {
				Expect(logBuffer.String()).To(ContainSubstring("operation data cannot be parsed"))
			})

			Describe("returned error", func() {
				It("includes a standard message", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						"There was a problem completing your request. Please contact your operations team providing the following information:",
					)))
				})

				It("includes the broker request id", func() {
					Expect(lastOpErr).To(MatchError(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					)))
				})

				It("includes the service name", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						"service: a-cool-redis-service",
					)))
				})

				It("includes the service instance guid", func() {
					Expect(lastOpErr).To(MatchError(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					)))
				})

				It("does not include the operation type", func() {
					Expect(lastOpErr).NotTo(MatchError(ContainSubstring(
						"operation:",
					)))
				})

				It("does NOT include the bosh task id", func() {
					Expect(lastOpErr).NotTo(MatchError(ContainSubstring(
						"task-id:",
					)))
				})
			})
		})

		Context("the broker is configured to expose operational errors", func() {
			BeforeEach(func() {
				brokerConfig.ExposeOperationalErrors = true
				boshClient.GetTaskReturns(boshdirector.BoshTask{State: boshdirector.TaskError, Description: "some task", Result: "bosh error"}, nil)
				operationData = `{"BoshTaskID": 42, "OperationType": "create"}`
			})

			It("exposes the error", func() {
				Expect(opResult.Description).To(ContainSubstring("bosh error"))
			})
		})
	})

	Context("when the task can be retrieved", func() {
		var (
			instanceID = "not-relevant"
			taskID     = 199
		)

		type testCase struct {
			ActualBoshTask      boshdirector.BoshTask
			ActualOperationType broker.OperationType
			LogContains         string

			ExpectedLastOperation                 domain.LastOperation
			ExpectedLastOperationState            domain.LastOperationState
			ExpectedLastOperationDescription      string
			ExpectedLastOperationDescriptionParts []string
		}

		testLastOperation := func(testCase testCase) func() {
			return func() {
				var (
					actualLastOperation      domain.LastOperation
					actualLastOperationError error
				)

				JustBeforeEach(func() {
					var err error
					operationData, err := json.Marshal(broker.OperationData{OperationType: testCase.ActualOperationType, BoshTaskID: taskID})
					Expect(err).NotTo(HaveOccurred())

					boshClient.GetTaskReturns(testCase.ActualBoshTask, nil)
					b = createDefaultBroker()
					pollDetails := domain.PollDetails{
						OperationData: string(operationData),
					}
					actualLastOperation, actualLastOperationError = b.LastOperation(context.Background(), instanceID, pollDetails)
				})

				It("succeeds", func() {
					Expect(boshClient.GetTaskCallCount()).To(Equal(1))
					actualTaskID, _ := boshClient.GetTaskArgsForCall(0)
					Expect(actualTaskID).To(Equal(taskID))
					Expect(actualLastOperationError).NotTo(HaveOccurred())

					Expect(actualLastOperation.State).To(Equal(
						testCase.ExpectedLastOperationState,
					))

					if testCase.ExpectedLastOperationDescription != "" {
						Expect(actualLastOperation.Description).To(Equal(
							testCase.ExpectedLastOperationDescription,
						))
					}

					for _, part := range testCase.ExpectedLastOperationDescriptionParts {
						Expect(actualLastOperation.Description).To(MatchRegexp(part))
					}

					Expect(logBuffer.String()).To(SatisfyAll(
						ContainSubstring(testCase.LogContains),
						ContainSubstring(fmt.Sprintf(
							"BOSH task ID %d status: %s %s deployment for instance %s: Description: %s Result: %s\n",
							taskID,
							testCase.ActualBoshTask.State,
							testCase.ActualOperationType,
							instanceID,
							testCase.ActualBoshTask.Description,
							testCase.ActualBoshTask.Result,
						))))

					Expect(fakeSecretManager.DeleteSecretsForInstanceCallCount()).To(Equal(0), "delete secrets should not be called")

					Expect(fakeTelemetryLogger.LogInstancesCallCount()).To(Equal(1), "telemetry logger should be called once")
					_, item, operation := fakeTelemetryLogger.LogInstancesArgsForCall(0)

					Expect(item).To(Equal("instance"))
					Expect(operation).To(Equal(string(testCase.ActualOperationType)))
				})
			}
		}

		Describe("while creating", func() {
			Describe("last operation is Processing",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskProcessing, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeCreate,

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance provisioning in progress",
				}),
			)

			Describe("last operation is Queued",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskQueued, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeCreate,

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance provisioning in progress",
				}),
			)

			Describe("last operation is Error",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskError,
						Result:      "result from error",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeCreate,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance provisioning failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: create",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is unrecognised",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       "not the state you were looking for",
						Result:      "who knows",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeCreate,
					LogContains:         "who knows",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance provisioning failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: create",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Cancelled",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskCancelled,
						Result:      "result from error",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeCreate,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance provisioning failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: create",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Cancelling",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskCancelling,
						Result:      "result from error",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeCreate,
					LogContains:         "result from error",

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance provisioning in progress",
				}),
			)

			Describe("last operation is Timed out",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskTimeout,
						Result:      "result from error",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeCreate,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance provisioning failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: create",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Successful",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskDone, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeCreate,

					ExpectedLastOperationState:       domain.Succeeded,
					ExpectedLastOperationDescription: "Instance provisioning completed",
				}),
			)
		})

		Describe("while deleting", func() {
			var operationData []byte

			BeforeEach(func() {
				boshClient.GetDeploymentReturns([]byte("mani"), true, nil)
				var err error
				operationData, err = json.Marshal(broker.OperationData{
					OperationType: broker.OperationTypeDelete,
					BoshTaskID:    taskID,
				})
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("last operation is Processing",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskProcessing, Description: "it's a task" + "-" + instanceID, ID: taskID},
					ActualOperationType: broker.OperationTypeDelete,

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance deletion in progress",
				}),
			)

			Describe("last operation is Queued",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskQueued, Description: "it's a task" + "-" + instanceID, ID: taskID},
					ActualOperationType: broker.OperationTypeDelete,

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance deletion in progress",
				}),
			)

			Describe("last operation is Error",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskError,
						Result:      "result from error",
						Description: "it's a task" + "-" + instanceID,
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeDelete,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance deletion failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: delete",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Cancelled",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskCancelled,
						Result:      "result from error",
						Description: "it's a task" + "-" + instanceID,
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeDelete,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance deletion failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: delete",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Cancelling",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskCancelling,
						Result:      "result from error",
						Description: "it's a task" + "-" + instanceID,
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeDelete,
					LogContains:         "result from error",

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance deletion in progress",
				}),
			)

			Describe("last operation is Timed out",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskTimeout,
						Result:      "result from error",
						Description: "it's a task" + "-" + instanceID,
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeDelete,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance deletion failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: delete",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is unrecognised",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       "not the state you were looking for",
						Result:      "who knows",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeDelete,
					LogContains:         "who knows",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance deletion failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: delete",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Successful", func() {
				BeforeEach(func() {
					boshClient.GetTaskReturns(boshdirector.BoshTask{
						State:       boshdirector.TaskDone,
						Description: "it's a task" + "-" + instanceID,
						ID:          taskID,
					}, nil)

					boshClient.GetConfigsReturns([]boshdirector.BoshConfig{
						{
							Type: "some-config-type",
							Name: "some-config-name",
						},
					}, nil)
				})

				It("cleans up configs and returns success", func() {
					b = createDefaultBroker()

					pollDetails := domain.PollDetails{
						OperationData: string(operationData),
					}
					actualLastOperation, actualLastOperationError := b.LastOperation(context.Background(), instanceID, pollDetails)

					Expect(actualLastOperationError).NotTo(HaveOccurred())
					Expect(actualLastOperation.State).To(Equal(domain.Succeeded))
					Expect(actualLastOperation.Description).To(Equal("Instance deletion completed"))

					Expect(boshClient.GetTaskCallCount()).To(Equal(1))
					actualTaskID, _ := boshClient.GetTaskArgsForCall(0)
					Expect(actualTaskID).To(Equal(taskID))

					By("deleting configs")
					Expect(boshClient.DeleteConfigsCallCount()).To(Equal(1), "expected to call delete config")

					By("logging the deployment, type: delete, state: done")
					expectedLogMessage := fmt.Sprintf(
						"BOSH task ID %d status: %s %s deployment for instance %s: Description: %s",
						taskID,
						boshdirector.TaskDone,
						broker.OperationTypeDelete,
						instanceID,
						"it's a task"+"-"+instanceID,
					)
					Expect(logBuffer.String()).To(ContainSubstring(expectedLogMessage))
				})

				It("returns failed status and logs detail when deleting configs fails", func() {
					operationData, err := json.Marshal(broker.OperationData{
						OperationType: broker.OperationTypeDelete,
						BoshTaskID:    taskID,
					})
					Expect(err).NotTo(HaveOccurred())

					boshClient.DeleteConfigsReturns(errors.New("failed to delete configs"))

					b = createDefaultBroker()

					pollDetails := domain.PollDetails{
						OperationData: string(operationData),
					}
					actualLastOperationData, actualError := b.LastOperation(context.Background(), instanceID, pollDetails)
					Expect(actualError).NotTo(HaveOccurred())

					Expect(actualLastOperationData.State).To(Equal(domain.Failed))
					Expect(actualLastOperationData.Description).To(SatisfyAll(
						ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information:"),
						MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
						ContainSubstring("service: a-cool-redis-service"),
						ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
						ContainSubstring("operation: delete"),
						Not(ContainSubstring(fmt.Sprintf("task-id"))),
					))

					Expect(logBuffer.String()).To(ContainSubstring("failed to delete configs"))
				})

				It("cleans up secrets and returns success", func() {
					operationData, err := json.Marshal(broker.OperationData{
						OperationType: broker.OperationTypeDelete,
						BoshTaskID:    taskID,
					})
					Expect(err).NotTo(HaveOccurred())

					b = createDefaultBroker()

					pollDetails := domain.PollDetails{
						OperationData: string(operationData),
					}
					actualLastOperation, actualLastOperationError := b.LastOperation(context.Background(), instanceID, pollDetails)

					Expect(actualLastOperationError).NotTo(HaveOccurred())
					Expect(actualLastOperation.State).To(Equal(domain.Succeeded))
					Expect(actualLastOperation.Description).To(Equal("Instance deletion completed"))

					Expect(boshClient.GetTaskCallCount()).To(Equal(1))
					actualTaskID, _ := boshClient.GetTaskArgsForCall(0)
					Expect(actualTaskID).To(Equal(taskID))

					By("deleting secrets")
					Expect(fakeSecretManager.DeleteSecretsForInstanceCallCount()).To(Equal(1), "expected to call secret manager")

					actualInstanceID, _ := fakeSecretManager.DeleteSecretsForInstanceArgsForCall(0)
					Expect(instanceID).To(Equal(actualInstanceID))

					By("logging the deployment, type: delete, state: done")
					expectedLogMessage := fmt.Sprintf(
						"BOSH task ID %d status: %s %s deployment for instance %s: Description: %s",
						taskID,
						boshdirector.TaskDone,
						broker.OperationTypeDelete,
						instanceID,
						"it's a task"+"-"+instanceID,
					)
					Expect(logBuffer.String()).To(ContainSubstring(expectedLogMessage))
				})

				It("returns failed status and logs detail when deleting a secret fails", func() {
					operationData, err := json.Marshal(broker.OperationData{
						OperationType: broker.OperationTypeDelete,
						BoshTaskID:    taskID,
					})
					Expect(err).NotTo(HaveOccurred())

					fakeSecretManager.DeleteSecretsForInstanceReturns(errors.New("failed to delete secrets"))

					b = createDefaultBroker()

					pollDetails := domain.PollDetails{
						OperationData: string(operationData),
					}
					actualLastOperationData, actualError := b.LastOperation(context.Background(), instanceID, pollDetails)
					Expect(actualError).NotTo(HaveOccurred())

					Expect(actualLastOperationData.State).To(Equal(domain.Failed))
					Expect(actualLastOperationData.Description).To(SatisfyAll(
						ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information:"),
						MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
						ContainSubstring("service: a-cool-redis-service"),
						ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
						ContainSubstring("operation: delete"),
						Not(ContainSubstring(fmt.Sprintf("task-id"))),
					))

					Expect(logBuffer.String()).To(ContainSubstring("failed to delete secrets"))
				})

				Context("bosh configs are disabled", func() {
					It("doesn't call GetConfigs or DeleteConfig", func() {
						brokerConfig.DisableBoshConfigs = true
						b = createDefaultBroker()

						_, err := b.LastOperation(context.Background(), instanceID, domain.PollDetails{
							OperationData: string(operationData),
						})
						Expect(err).NotTo(HaveOccurred())

						Expect(boshClient.GetConfigsCallCount()).To(Equal(0), "GetConfigs was called")
						Expect(boshClient.DeleteConfigCallCount()).To(Equal(0), "DeleteConfig was called")
					})
				})
			})
		})

		Describe("while force-deleting", func() {
			Describe("last operation is Processing",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskProcessing, Description: "it's a task" + "-" + instanceID, ID: taskID},
					ActualOperationType: broker.OperationTypeForceDelete,

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance forced deletion in progress",
				}),
			)

			Describe("last operation is Queued",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskQueued, Description: "it's a task" + "-" + instanceID, ID: taskID},
					ActualOperationType: broker.OperationTypeForceDelete,

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance forced deletion in progress",
				}),
			)

			Describe("last operation is Error",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskError,
						Result:      "result from error",
						Description: "it's a task" + "-" + instanceID,
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeForceDelete,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance forced deletion failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: force-delete",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Cancelled",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskCancelled,
						Result:      "result from error",
						Description: "it's a task" + "-" + instanceID,
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeForceDelete,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance forced deletion failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: force-delete",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Cancelling",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskCancelling,
						Result:      "result from error",
						Description: "it's a task" + "-" + instanceID,
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeForceDelete,
					LogContains:         "result from error",

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance forced deletion in progress",
				}),
			)

			Describe("last operation is Timed out",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskTimeout,
						Result:      "result from error",
						Description: "it's a task" + "-" + instanceID,
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeForceDelete,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance forced deletion failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: force-delete",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is unrecognised",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       "not the state you were looking for",
						Result:      "who knows",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeForceDelete,
					LogContains:         "who knows",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance forced deletion failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: force-delete",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Successful", func() {
				BeforeEach(func() {
					boshClient.GetTaskReturns(boshdirector.BoshTask{
						State:       boshdirector.TaskDone,
						Description: "it's a task" + "-" + instanceID,
						ID:          taskID,
					}, nil)

					boshClient.GetConfigsReturns([]boshdirector.BoshConfig{
						{
							Type: "some-config-type",
							Name: "some-config-name",
						},
					}, nil)
				})

				It("cleans up configs and returns success", func() {
					b = createDefaultBroker()

					operationData, err := json.Marshal(broker.OperationData{OperationType: broker.OperationTypeForceDelete, BoshTaskID: taskID})
					Expect(err).NotTo(HaveOccurred())

					pollDetails := domain.PollDetails{
						OperationData: string(operationData),
					}
					actualLastOperation, actualLastOperationError := b.LastOperation(context.Background(), instanceID, pollDetails)

					Expect(actualLastOperationError).NotTo(HaveOccurred())
					Expect(actualLastOperation.State).To(Equal(domain.Succeeded))
					Expect(actualLastOperation.Description).To(Equal("Instance forced deletion completed"))

					Expect(boshClient.GetTaskCallCount()).To(Equal(1))
					actualTaskID, _ := boshClient.GetTaskArgsForCall(0)
					Expect(actualTaskID).To(Equal(taskID))

					By("deleting configs")
					Expect(boshClient.DeleteConfigsCallCount()).To(Equal(1), "expected to call delete config")

					By("logging the deployment, type: delete, state: done")
					expectedLogMessage := fmt.Sprintf(
						"BOSH task ID %d status: %s %s deployment for instance %s: Description: %s",
						taskID,
						boshdirector.TaskDone,
						broker.OperationTypeForceDelete,
						instanceID,
						"it's a task"+"-"+instanceID,
					)
					Expect(logBuffer.String()).To(ContainSubstring(expectedLogMessage))
				})

				It("returns failed status and logs detail when deleting configs fails", func() {
					operationData, err := json.Marshal(broker.OperationData{
						OperationType: broker.OperationTypeForceDelete,
						BoshTaskID:    taskID,
					})
					Expect(err).NotTo(HaveOccurred())

					boshClient.DeleteConfigsReturns(errors.New("failed to delete configs"))

					b = createDefaultBroker()

					pollDetails := domain.PollDetails{
						OperationData: string(operationData),
					}
					actualLastOperationData, actualError := b.LastOperation(context.Background(), instanceID, pollDetails)
					Expect(actualError).NotTo(HaveOccurred())

					Expect(actualLastOperationData.State).To(Equal(domain.Failed))
					Expect(actualLastOperationData.Description).To(SatisfyAll(
						ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information:"),
						MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
						ContainSubstring("service: a-cool-redis-service"),
						ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
						ContainSubstring("operation: force-delete"),
						Not(ContainSubstring(fmt.Sprintf("task-id"))),
					))

					Expect(logBuffer.String()).To(ContainSubstring("failed to delete configs"))
				})

				It("cleans up secrets and returns success", func() {
					operationData, err := json.Marshal(broker.OperationData{
						OperationType: broker.OperationTypeForceDelete,
						BoshTaskID:    taskID,
					})
					Expect(err).NotTo(HaveOccurred())

					b = createDefaultBroker()

					pollDetails := domain.PollDetails{
						OperationData: string(operationData),
					}
					actualLastOperation, actualLastOperationError := b.LastOperation(context.Background(), instanceID, pollDetails)

					Expect(actualLastOperationError).NotTo(HaveOccurred())
					Expect(actualLastOperation.State).To(Equal(domain.Succeeded))
					Expect(actualLastOperation.Description).To(Equal("Instance forced deletion completed"))

					Expect(boshClient.GetTaskCallCount()).To(Equal(1))
					actualTaskID, _ := boshClient.GetTaskArgsForCall(0)
					Expect(actualTaskID).To(Equal(taskID))

					By("deleting secrets")
					Expect(fakeSecretManager.DeleteSecretsForInstanceCallCount()).To(Equal(1), "expected to call secret manager")

					actualInstanceID, _ := fakeSecretManager.DeleteSecretsForInstanceArgsForCall(0)
					Expect(instanceID).To(Equal(actualInstanceID))

					By("logging the deployment, type: delete, state: done")
					expectedLogMessage := fmt.Sprintf(
						"BOSH task ID %d status: %s %s deployment for instance %s: Description: %s",
						taskID,
						boshdirector.TaskDone,
						broker.OperationTypeForceDelete,
						instanceID,
						"it's a task"+"-"+instanceID,
					)
					Expect(logBuffer.String()).To(ContainSubstring(expectedLogMessage))
				})

				It("returns failed status and logs detail when deleting a secret fails", func() {
					operationData, err := json.Marshal(broker.OperationData{
						OperationType: broker.OperationTypeForceDelete,
						BoshTaskID:    taskID,
					})
					Expect(err).NotTo(HaveOccurred())

					fakeSecretManager.DeleteSecretsForInstanceReturns(errors.New("failed to delete secrets"))

					b = createDefaultBroker()

					pollDetails := domain.PollDetails{
						OperationData: string(operationData),
					}
					actualLastOperationData, actualError := b.LastOperation(context.Background(), instanceID, pollDetails)
					Expect(actualError).NotTo(HaveOccurred())

					Expect(actualLastOperationData.State).To(Equal(domain.Failed))
					Expect(actualLastOperationData.Description).To(SatisfyAll(
						ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information:"),
						MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
						ContainSubstring("service: a-cool-redis-service"),
						ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
						ContainSubstring("operation: force-delete"),
						Not(ContainSubstring(fmt.Sprintf("task-id"))),
					))

					Expect(logBuffer.String()).To(ContainSubstring("failed to delete secrets"))
				})

				Context("bosh configs are disabled", func() {
					It("doesn't call GetConfigs or DeleteConfig", func() {
						brokerConfig.DisableBoshConfigs = true
						b = createDefaultBroker()

						operationData, err := json.Marshal(broker.OperationData{OperationType: broker.OperationTypeForceDelete, BoshTaskID: taskID})
						Expect(err).NotTo(HaveOccurred())

						_, err = b.LastOperation(context.Background(), instanceID, domain.PollDetails{
							OperationData: string(operationData),
						})
						Expect(err).NotTo(HaveOccurred())

						Expect(boshClient.GetConfigsCallCount()).To(Equal(0), "GetConfigs was called")
						Expect(boshClient.DeleteConfigCallCount()).To(Equal(0), "DeleteConfig was called")
					})
				})
			})
		})

		Describe("while recreating", func() {
			Describe("last operation is Processing",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskProcessing, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeRecreate,

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance recreate in progress",
				}),
			)

			Describe("last operation is Queued",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskQueued, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeRecreate,

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance recreate in progress",
				}),
			)

			Describe("last operation is Error",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskError,
						Result:      "result from error",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeRecreate,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance recreate failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: recreate",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Cancelled",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskCancelled,
						Result:      "result from error",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeRecreate,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance recreate failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: recreate",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Cancelling",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskCancelling,
						Result:      "result from error",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeRecreate,
					LogContains:         "result from error",

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance recreate in progress",
				}),
			)

			Describe("last operation is Timed out",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskTimeout,
						Result:      "result from error",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeRecreate,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance recreate failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: recreate",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is unrecognised",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       "not the state you were looking for",
						Result:      "who knows",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeRecreate,
					LogContains:         "who knows",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance recreate failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: recreate",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Successful",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskDone, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeRecreate,

					ExpectedLastOperationState:       domain.Succeeded,
					ExpectedLastOperationDescription: "Instance recreate completed",
				}),
			)
		})

		Describe("while updating", func() {
			Describe("last operation is Processing",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskProcessing, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpdate,

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance update in progress",
				}),
			)

			Describe("last operation is Queued",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskQueued, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpdate,

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance update in progress",
				}),
			)

			Describe("last operation is Error",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskError,
						Result:      "result from error",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeUpdate,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance update failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: update",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Cancelled",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskCancelled,
						Result:      "result from error",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeUpdate,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance update failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: update",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Cancelling",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskCancelling,
						Result:      "result from error",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeUpdate,
					LogContains:         "result from error",

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance update in progress",
				}),
			)

			Describe("last operation is Timed out",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       boshdirector.TaskTimeout,
						Result:      "result from error",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeUpdate,
					LogContains:         "result from error",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance update failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: update",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is unrecognised",
				testLastOperation(testCase{
					ActualBoshTask: boshdirector.BoshTask{
						State:       "not the state you were looking for",
						Result:      "who knows",
						Description: "it's a task",
						ID:          taskID,
					},
					ActualOperationType: broker.OperationTypeUpdate,
					LogContains:         "who knows",

					ExpectedLastOperationState: domain.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance update failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: update",
						fmt.Sprintf("task-id: %d", taskID),
					},
				}),
			)

			Describe("last operation is Successful",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskDone, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpdate,

					ExpectedLastOperationState:       domain.Succeeded,
					ExpectedLastOperationDescription: "Instance update completed",
				}),
			)
		})

		Describe("while upgrading", func() {
			Describe("last operation is Processing",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskProcessing, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance upgrade in progress",
				}),
			)

			Describe("last operation is Queued",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskQueued, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance upgrade in progress",
				}),
			)

			Describe("last operation is Error",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskError, Result: "result from error", Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,

					ExpectedLastOperationState:       domain.Failed,
					ExpectedLastOperationDescription: "Failed for bosh task: 199",
				}),
			)

			Describe("last operation is Cancelled",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskCancelled, Result: "result from error", Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,

					ExpectedLastOperationState:       domain.Failed,
					ExpectedLastOperationDescription: "Failed for bosh task: 199",
				}),
			)

			Describe("last operation is Cancelling",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskCancelling, Result: "result from error", Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,

					ExpectedLastOperationState:       domain.InProgress,
					ExpectedLastOperationDescription: "Instance upgrade in progress",
				}),
			)

			Describe("last operation is Timed out",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskTimeout, Result: "result from error", Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,

					ExpectedLastOperationState:       domain.Failed,
					ExpectedLastOperationDescription: "Failed for bosh task: 199",
				}),
			)

			Describe("last operation is unrecognised",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: "not the state you were looking for", Result: "who knows", Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,
					LogContains:         "who knows",

					ExpectedLastOperationState:       domain.Failed,
					ExpectedLastOperationDescription: "Failed for bosh task: 199",
				}),
			)

			Describe("last operation is Successful",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskDone, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,

					ExpectedLastOperationState:       domain.Succeeded,
					ExpectedLastOperationDescription: "Instance upgrade completed",
				}),
			)
		})
	})
})
