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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
)

var _ = Describe("LastOperation", func() {
	Context("failures", func() {
		var (
			instanceID    = "a-useful-instance"
			operationData string

			lastOpErr error
		)

		JustBeforeEach(func() {
			b = createDefaultBroker()
			_, lastOpErr = b.LastOperation(context.Background(), instanceID, operationData)
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
	})

	Context("when the task can be retrieved", func() {
		var (
			instanceID    = "not-relevent"
			operationData []byte
			taskID        = 199
		)

		type testCase struct {
			ActualBoshTask      boshdirector.BoshTask
			ActualOperationType broker.OperationType
			LogContains         string

			ExpectedLastOperation                 brokerapi.LastOperation
			ExpectedLastOperationState            brokerapi.LastOperationState
			ExpectedLastOperationDescription      string
			ExpectedLastOperationDescriptionParts []string
		}

		testLogMessage := func(testCase testCase) func() {
			return func() {
				expectedLogMessage := fmt.Sprintf(
					"BOSH task ID %d status: %s %s deployment for instance %s: Description: %s Result: %s\n",
					taskID,
					testCase.ActualBoshTask.State,
					testCase.ActualOperationType,
					instanceID,
					testCase.ActualBoshTask.Description,
					testCase.ActualBoshTask.Result,
				)
				Expect(logBuffer.String()).To(ContainSubstring(expectedLogMessage))
			}
		}

		testLastOperation := func(testCase testCase) func() {
			return func() {
				var (
					actualLastOperation      brokerapi.LastOperation
					actualLastOperationError error
				)

				JustBeforeEach(func() {
					var err error
					operationData, err = json.Marshal(broker.OperationData{OperationType: testCase.ActualOperationType, BoshTaskID: taskID})
					Expect(err).NotTo(HaveOccurred())

					boshClient.GetTaskReturns(testCase.ActualBoshTask, nil)
					b = createDefaultBroker()
					actualLastOperation, actualLastOperationError = b.LastOperation(context.Background(), instanceID, string(operationData))
				})

				It("retrieves the task by ID", func() {
					Expect(boshClient.GetTaskCallCount()).To(Equal(1))
					actualTaskID, _ := boshClient.GetTaskArgsForCall(0)
					Expect(actualTaskID).To(Equal(taskID))
				})

				It("does not error", func() {
					Expect(actualLastOperationError).NotTo(HaveOccurred())
				})

				It("returns the state", func() {
					Expect(actualLastOperation.State).To(Equal(
						testCase.ExpectedLastOperationState,
					))
				})

				It("returns a description", func() {
					if testCase.ExpectedLastOperationDescription != "" {
						Expect(actualLastOperation.Description).To(Equal(
							testCase.ExpectedLastOperationDescription,
						))
					}
				})

				It("returns a description containing all the expected parts", func() {
					for _, part := range testCase.ExpectedLastOperationDescriptionParts {
						Expect(actualLastOperation.Description).To(MatchRegexp(part))
					}
				})

				It("logs a message", func() {
					Expect(logBuffer.String()).To(ContainSubstring(testCase.LogContains))
				})

				It(fmt.Sprintf("logs the deployment, type: %s, state: %s", testCase.ActualOperationType, testCase.ActualBoshTask.State), testLogMessage(testCase))
			}
		}

		Describe("while creating", func() {
			Describe("last operation is Processing",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskProcessing, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeCreate,

					ExpectedLastOperationState:       brokerapi.InProgress,
					ExpectedLastOperationDescription: "Instance provisioning in progress",
				}),
			)

			Describe("last operation is Queued",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskQueued, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeCreate,

					ExpectedLastOperationState:       brokerapi.InProgress,
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

					ExpectedLastOperationState: brokerapi.Failed,
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

					ExpectedLastOperationState: brokerapi.Failed,
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

					ExpectedLastOperationState: brokerapi.Failed,
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

					ExpectedLastOperationState:       brokerapi.InProgress,
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

					ExpectedLastOperationState: brokerapi.Failed,
					ExpectedLastOperationDescriptionParts: []string{
						"Instance provisioning failed: There was a problem completing your request. Please contact your operations team providing the following information:",
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
						"service: a-cool-redis-service",
						fmt.Sprintf("service-instance-guid: %s", instanceID),
						"operation: create",
						fmt.Sprintf("task-id: %d", taskID),
					}}),
			)

			Describe("last operation is Successful",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskDone, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeCreate,

					ExpectedLastOperationState:       brokerapi.Succeeded,
					ExpectedLastOperationDescription: "Instance provisioning completed",
				}),
			)
		})

		Describe("while deleting", func() {
			BeforeEach(func() {
				boshClient.GetDeploymentReturns([]byte("mani"), true, nil)
			})

			Describe("last operation is Processing",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskProcessing, Description: "it's a task" + "-" + instanceID, ID: taskID},
					ActualOperationType: broker.OperationTypeDelete,

					ExpectedLastOperationState:       brokerapi.InProgress,
					ExpectedLastOperationDescription: "Instance deletion in progress",
				}),
			)

			Describe("last operation is Queued",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskQueued, Description: "it's a task" + "-" + instanceID, ID: taskID},
					ActualOperationType: broker.OperationTypeDelete,

					ExpectedLastOperationState:       brokerapi.InProgress,
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

					ExpectedLastOperationState: brokerapi.Failed,
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

					ExpectedLastOperationState: brokerapi.Failed,
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

					ExpectedLastOperationState:       brokerapi.InProgress,
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

					ExpectedLastOperationState: brokerapi.Failed,
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

					ExpectedLastOperationState: brokerapi.Failed,
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

			Describe("last operation is Successful",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskDone, Description: "it's a task" + "-" + instanceID, ID: taskID},
					ActualOperationType: broker.OperationTypeDelete,

					ExpectedLastOperationState:       brokerapi.Succeeded,
					ExpectedLastOperationDescription: "Instance deletion completed",
				}),
			)
		})

		Describe("while updating", func() {
			Describe("last operation is Processing",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskProcessing, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpdate,

					ExpectedLastOperationState:       brokerapi.InProgress,
					ExpectedLastOperationDescription: "Instance update in progress",
				}),
			)

			Describe("last operation is Queued",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskQueued, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpdate,

					ExpectedLastOperationState:       brokerapi.InProgress,
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

					ExpectedLastOperationState: brokerapi.Failed,
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

					ExpectedLastOperationState: brokerapi.Failed,
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

					ExpectedLastOperationState:       brokerapi.InProgress,
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

					ExpectedLastOperationState: brokerapi.Failed,
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

					ExpectedLastOperationState: brokerapi.Failed,
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

					ExpectedLastOperationState:       brokerapi.Succeeded,
					ExpectedLastOperationDescription: "Instance update completed",
				}),
			)
		})

		Describe("while upgrading", func() {
			Describe("last operation is Processing",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskProcessing, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,

					ExpectedLastOperationState:       brokerapi.InProgress,
					ExpectedLastOperationDescription: "Instance upgrade in progress",
				}),
			)

			Describe("last operation is Queued",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskQueued, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,

					ExpectedLastOperationState:       brokerapi.InProgress,
					ExpectedLastOperationDescription: "Instance upgrade in progress",
				}),
			)

			Describe("last operation is Error",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskError, Result: "result from error", Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,

					ExpectedLastOperationState:       brokerapi.Failed,
					ExpectedLastOperationDescription: "Failed for bosh task: 199",
				}),
			)

			Describe("last operation is Cancelled",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskCancelled, Result: "result from error", Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,

					ExpectedLastOperationState:       brokerapi.Failed,
					ExpectedLastOperationDescription: "Failed for bosh task: 199",
				}),
			)

			Describe("last operation is Cancelling",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskCancelling, Result: "result from error", Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,

					ExpectedLastOperationState:       brokerapi.InProgress,
					ExpectedLastOperationDescription: "Instance upgrade in progress",
				}),
			)

			Describe("last operation is Timed out",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskTimeout, Result: "result from error", Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,

					ExpectedLastOperationState:       brokerapi.Failed,
					ExpectedLastOperationDescription: "Failed for bosh task: 199",
				}),
			)

			Describe("last operation is unrecognised",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: "not the state you were looking for", Result: "who knows", Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,
					LogContains:         "who knows",

					ExpectedLastOperationState:       brokerapi.Failed,
					ExpectedLastOperationDescription: "Failed for bosh task: 199",
				}),
			)

			Describe("last operation is Successful",
				testLastOperation(testCase{
					ActualBoshTask:      boshdirector.BoshTask{State: boshdirector.TaskDone, Description: "it's a task", ID: taskID},
					ActualOperationType: broker.OperationTypeUpgrade,

					ExpectedLastOperationState:       brokerapi.Succeeded,
					ExpectedLastOperationDescription: "Instance upgrade completed",
				}),
			)
		})
	})
})
