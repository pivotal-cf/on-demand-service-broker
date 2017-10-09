// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_tests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
)

var _ = Describe("last operation", func() {
	const (
		instanceID = "some-instance-id"
		boshTaskID = 10654
	)

	var (
		planID                string
		postDeployErrandName  string
		boshDirector          *mockbosh.MockBOSH
		cfAPI                 *mockhttp.Server
		cfUAA                 *mockuaa.ClientCredentialsServer
		boshUAA               *mockuaa.ClientCredentialsServer
		runningBroker         *gexec.Session
		operationType         broker.OperationType
		contextID             string
		lastOperationResponse *http.Response
		brokerConfig          config.Config
		actualResponse        map[string]interface{}
		rawResponse           []byte

		taskDone        = boshdirector.BoshTask{ID: 1, State: boshdirector.TaskDone, ContextID: contextID, Description: "done thing"}
		anotherTaskDone = boshdirector.BoshTask{ID: 4, State: boshdirector.TaskDone, ContextID: contextID, Description: "another done thing"}
		taskProcessing  = boshdirector.BoshTask{ID: 2, State: boshdirector.TaskProcessing, ContextID: contextID, Description: "processing thing"}
		taskFailed      = boshdirector.BoshTask{ID: 3, State: boshdirector.TaskError, ContextID: contextID, Description: "failed thing"}

		describeFailure = SatisfyAll(
			ContainSubstring("Instance provisioning failed: There was a problem completing your request. Please contact your operations team providing the following information:"),
			MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
			ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
			ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
			ContainSubstring("operation: create"),
			ContainSubstring(fmt.Sprintf("task-id: %d", boshTaskID)),
		)
		reportedStateRegex = func(status string) string {
			return logRegexpStringWithRequestIDCapture(fmt.Sprintf(`BOSH task ID %d status: %s create deployment for instance %s`, boshTaskID, status, instanceID))
		}
	)

	BeforeEach(func() {
		contextID = ""
		planID = dedicatedPlanID
		boshUAA = mockuaa.NewClientCredentialsServer(boshClientID, boshClientSecret, "bosh uaa token")
		boshDirector = mockbosh.NewWithUAA(boshUAA.URL)
		boshDirector.ExpectedAuthorizationHeader(boshUAA.ExpectedAuthorizationHeader())
		boshDirector.ExcludeAuthorizationCheck("/info")

		cfAPI = mockcfapi.New()
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")
		adapter.GenerateManifest().ToReturnManifest(rawManifestWithDeploymentName(instanceID))
		brokerConfig = defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)
	})

	JustBeforeEach(func() {
		operationData := broker.OperationData{}
		if operationType != "" {
			operationData = broker.OperationData{
				BoshTaskID:           boshTaskID,
				OperationType:        operationType,
				BoshContextID:        contextID,
				PlanID:               planID,
				PostDeployErrandName: postDeployErrandName,
			}
		}
		lastOperationResponse = lastOperationForInstance(instanceID, operationData)

		var err error
		defer lastOperationResponse.Body.Close()
		rawResponse, err = ioutil.ReadAll(lastOperationResponse.Body)
		Expect(err).NotTo(HaveOccurred())
		json.Unmarshal(rawResponse, &actualResponse)
	})

	AfterEach(func() {
		killBrokerAndCheckForOpenConnections(runningBroker, boshDirector.URL)
		boshDirector.VerifyMocks()
		boshDirector.Close()
		boshUAA.Close()

		cfAPI.VerifyMocks()
		cfAPI.Close()
		cfUAA.Close()
	})

	Context("when lifecycle errands are NOT configured and operation data does NOT contain a context ID", func() {
		BeforeEach(func() {
			operationType = broker.OperationTypeCreate
			runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)
		})

		Context("when the task is processing", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Task(boshTaskID).RespondsWithTaskContainingState(boshdirector.TaskProcessing),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))

				Expect(rawResponse).To(MatchJSON(fmt.Sprintf(`{"state" : "%s", "description": "Instance provisioning in progress"}`, brokerapi.InProgress)))
				Eventually(runningBroker).Should(gbytes.Say(reportedStateRegex("processing")))
			})
		})

		Context("when the task is done", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Task(boshTaskID).RespondsWithTaskContainingState(boshdirector.TaskDone),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
				Expect(rawResponse).To(MatchJSON(fmt.Sprintf(`{"state" : "%s", "description": "Instance provisioning completed"}`, brokerapi.Succeeded)))
				Eventually(runningBroker).Should(gbytes.Say(reportedStateRegex("done")))
			})
		})

		Context("when the task has errored", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Task(boshTaskID).RespondsWithTaskContainingState(boshdirector.TaskError),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
				Expect(actualResponse["state"]).To(Equal(string(brokerapi.Failed)))
				Expect(actualResponse["description"].(string)).To(describeFailure)

				Eventually(runningBroker).Should(gbytes.Say(reportedStateRegex("error")))
			})
		})

		Context("when the task is cancelling", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Task(boshTaskID).RespondsWithTaskContainingState(boshdirector.TaskCancelling),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("has a description", func() {
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{
						"state":       brokerapi.InProgress,
						"description": "Instance provisioning in progress",
					},
				)))
			})

			It("logs with a request ID", func() {
				Eventually(runningBroker).Should(gbytes.Say(reportedStateRegex("cancelling")))
			})
		})

		Context("when the task is cancelled", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Task(boshTaskID).RespondsWithTaskContainingState(boshdirector.TaskCancelled),
				)
			})

			It("responds with 200 with state failed", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
				Expect(actualResponse["state"]).To(Equal(string(brokerapi.Failed)))
				Expect(actualResponse["description"].(string)).To(describeFailure)

				Eventually(runningBroker).Should(gbytes.Say(reportedStateRegex("cancelled")))
			})
		})

		Context("when the task has timed out", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Task(boshTaskID).RespondsWithTaskContainingState(boshdirector.TaskTimeout),
				)
			})

			It("responds with 200 with state failed", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
				Expect(actualResponse["state"]).To(Equal(string(brokerapi.Failed)))
				Expect(actualResponse["description"]).To(describeFailure)

				Eventually(runningBroker).Should(gbytes.Say(reportedStateRegex("timeout")))
			})
		})

		Context("when the task has an unrecognised state", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Task(boshTaskID).RespondsWithTaskContainingState("unrecognised-state"),
				)
			})

			It("responds with 200 with state failed", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
				Expect(actualResponse["state"]).To(Equal(string(brokerapi.Failed)))
				Expect(actualResponse["description"]).To(describeFailure)

				regexpString := logRegexpStringWithRequestIDCapture(`Unrecognised BOSH task state: unrecognised-state`)
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
				Eventually(runningBroker).Should(gbytes.Say(reportedStateRegex("unrecognised-state")))
			})
		})
	})

	Context("when post-deploy errand is configured and operation data contains a context id", func() {
		var (
			errandName = "health-check"
		)

		BeforeEach(func() {
			planID = "post-deploy-plan-id"
			operationType = broker.OperationTypeCreate
			contextID = "some-context-id"

			planWithPostDeploy := config.Plan{
				ID:   planID,
				Name: "post-deploy-plan",
				LifecycleErrands: &config.LifecycleErrands{
					PostDeploy: config.Errand{
						Name: errandName,
					},
				},
			}

			brokerConfig.ServiceCatalog.Plans = []config.Plan{planWithPostDeploy}

			runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)
		})

		Context("and when there is a single incomplete task", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsOKWithJSON(boshdirector.BoshTasks{taskProcessing}),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("has a description", func() {
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{
						"state":       brokerapi.InProgress,
						"description": "Instance provisioning in progress",
					},
				)),
				)
			})

			It("logs with a request ID", func() {
				regexpString := logRegexpStringWithRequestIDCapture(
					fmt.Sprintf(`BOSH task ID %d status: processing create deployment for instance %s: Description: %s`, taskProcessing.ID, instanceID, taskProcessing.Description),
				)
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
			})
		})

		Context("and when there is a single complete task", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsOKWithJSON(boshdirector.BoshTasks{taskDone}),
					mockbosh.TaskOutput(taskDone.ID).RespondsOKWith(""),
					mockbosh.Errand(deploymentName(instanceID), errandName, `{}`).
						WithContextID(contextID).RedirectsToTask(taskProcessing.ID),
					mockbosh.Task(taskProcessing.ID).RespondsOKWithJSON(taskProcessing),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("has a description", func() {
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{
						"state":       brokerapi.InProgress,
						"description": "Instance provisioning in progress",
					},
				)),
				)
			})

			It("logs with a request ID", func() {
				regexpString := logRegexpStringWithRequestIDCapture(
					fmt.Sprintf(`BOSH task ID %d status: processing create deployment for instance %s: Description: %s`, taskProcessing.ID, instanceID, taskProcessing.Description),
				)
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
			})
		})

		Context("and when there are two tasks and the second task is processing", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsOKWithJSON(boshdirector.BoshTasks{taskProcessing, taskDone}),
					mockbosh.TaskOutput(taskDone.ID).RespondsOKWith(""),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("has a description", func() {
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{
						"state":       brokerapi.InProgress,
						"description": "Instance provisioning in progress",
					},
				)))
			})

			It("logs with a request ID", func() {
				regexpString := logRegexpStringWithRequestIDCapture(fmt.Sprintf(`BOSH task ID %d status: processing create deployment for instance %s: Description: %s`, taskProcessing.ID, instanceID, taskProcessing.Description))
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
			})
		})

		Context("and when there are two tasks and the second task has failed", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsOKWithJSON(boshdirector.BoshTasks{taskFailed, taskDone}),
					mockbosh.TaskOutput(taskDone.ID).RespondsOKWith(""),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			Describe("response body", func() {
				It("has state failed", func() {
					Expect(actualResponse["state"]).To(Equal(string(brokerapi.Failed)))
				})

				It("has generic error description", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						"Instance provisioning failed: There was a problem completing your request. Please contact your operations team providing the following information:",
					))
				})

				It("has description with broker request id", func() {
					Expect(actualResponse["description"]).To(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					))
				})

				It("has description with service name", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						fmt.Sprintf("service: %s", serviceName),
					))
				})

				It("has description with service instance guid", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					))
				})

				It("includes the operation type", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						"operation: create",
					))
				})

				It("includes the failed bosh task ID", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						fmt.Sprintf("task-id: %d", taskFailed.ID),
					))
				})
			})

			It("logs with a request ID", func() {
				regexpString := logRegexpStringWithRequestIDCapture(
					fmt.Sprintf(`BOSH task ID %d status: error create deployment for instance %s: Description: %s`, taskFailed.ID, instanceID, taskFailed.Description),
				)
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
			})
		})

		Context("and when there are two done tasks and both were successful", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsOKWithJSON(boshdirector.BoshTasks{anotherTaskDone, taskDone}),
					mockbosh.TaskOutput(anotherTaskDone.ID).RespondsOKWith(""),
					mockbosh.TaskOutput(taskDone.ID).RespondsOKWith(""),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("has a description", func() {
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{
						"state":       brokerapi.Succeeded,
						"description": "Instance provisioning completed",
					},
				)))
			})

			It("logs with a request ID", func() {
				regexpString := logRegexpStringWithRequestIDCapture(
					fmt.Sprintf(`BOSH task ID %d status: done create deployment for instance %s: Description: %s`, anotherTaskDone.ID, instanceID, anotherTaskDone.Description),
				)
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
			})
		})

		Context("and when there are two done tasks and the second is an errand that exited 1", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsOKWithJSON(boshdirector.BoshTasks{anotherTaskDone, taskDone}),
					mockbosh.TaskOutput(anotherTaskDone.ID).
						RespondsOKWithJSON(boshdirector.BoshTaskOutput{
							ExitCode: 1,
						}),
					mockbosh.TaskOutput(taskDone.ID).RespondsOKWith(""),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			Describe("response body", func() {
				It("has state failed", func() {
					Expect(actualResponse["state"]).To(Equal(string(brokerapi.Failed)))
				})

				It("has generic error description", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						"Instance provisioning failed: There was a problem completing your request. Please contact your operations team providing the following information:",
					))
				})

				It("has description with broker request id", func() {
					Expect(actualResponse["description"]).To(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					))
				})

				It("has description with service name", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						fmt.Sprintf("service: %s", serviceName),
					))
				})

				It("has description with service instance guid", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					))
				})

				It("includes the operation type", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						"operation: create",
					))
				})

				It("includes the failed bosh task ID", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						fmt.Sprintf("task-id: %d", anotherTaskDone.ID),
					))
				})
			})
		})
	})

	Context("when pre-delete errand is configured and operation data contains a context id", func() {
		var (
			errandName = "cleanup-data"
		)

		BeforeEach(func() {
			planID = "post-deploy-plan-id"
			operationType = broker.OperationTypeDelete
			contextID = "some-context-id"

			planWithPostDeploy := config.Plan{
				ID:   planID,
				Name: "pre-delete-plan",
				LifecycleErrands: &config.LifecycleErrands{
					PreDelete: errandName,
				},
			}

			brokerConfig.ServiceCatalog.Plans = []config.Plan{planWithPostDeploy}

			runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)
		})

		Context("and when there is a single incomplete task", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsOKWithJSON(boshdirector.BoshTasks{taskProcessing}),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("has a description", func() {
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{
						"state":       brokerapi.InProgress,
						"description": "Instance deletion in progress",
					},
				)))
			})

			It("logs with a request ID", func() {
				regexpString := logRegexpStringWithRequestIDCapture(
					fmt.Sprintf(
						`BOSH task ID %d status: processing delete deployment for instance %s`,
						taskProcessing.ID,
						instanceID,
					),
				)
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
			})
		})

		Context("and when there is a single complete task", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsOKWithJSON(boshdirector.BoshTasks{taskDone}),
					mockbosh.TaskOutput(taskDone.ID).RespondsOKWith(""),
					mockbosh.DeleteDeployment(deploymentName(instanceID)).
						WithContextID(contextID).RedirectsToTask(taskProcessing.ID),
					mockbosh.Task(taskProcessing.ID).RespondsOKWithJSON(taskProcessing),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("has a description", func() {
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{
						"state":       brokerapi.InProgress,
						"description": "Instance deletion in progress",
					},
				)),
				)
			})

			It("logs with a request ID", func() {
				regexpString := logRegexpStringWithRequestIDCapture(
					fmt.Sprintf(`BOSH task ID %d status: processing delete deployment for instance %s: Description: %s`, taskProcessing.ID, instanceID, taskProcessing.Description),
				)
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
			})
		})

		Context("and when there is a single task which is an errand that exited 1", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsOKWithJSON(boshdirector.BoshTasks{taskDone}),
					mockbosh.TaskOutput(taskDone.ID).
						RespondsOKWithJSON(boshdirector.BoshTaskOutput{
							ExitCode: 1,
						}),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			Describe("response body", func() {
				It("has state failed", func() {
					Expect(actualResponse["state"]).To(Equal(string(brokerapi.Failed)))
				})

				It("has generic error description", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						"Instance deletion failed: There was a problem completing your request. Please contact your operations team providing the following information:",
					))
				})

				It("has description with broker request id", func() {
					Expect(actualResponse["description"]).To(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					))
				})

				It("has description with service name", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						fmt.Sprintf("service: %s", serviceName),
					))
				})

				It("has description with service instance guid", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					))
				})

				It("includes the operation type", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						"operation: delete",
					))
				})

				It("includes the failed bosh task ID", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						fmt.Sprintf("task-id: %d", taskDone.ID),
					))
				})
			})
		})

		Context("and when there are two tasks and the second task is processing", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsOKWithJSON(boshdirector.BoshTasks{taskProcessing, taskDone}),
					mockbosh.TaskOutput(taskDone.ID).RespondsOKWith(""),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("has a description", func() {
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{
						"state":       brokerapi.InProgress,
						"description": "Instance deletion in progress",
					},
				)))
			})

			It("logs with a request ID", func() {
				regexpString := logRegexpStringWithRequestIDCapture(fmt.Sprintf(`BOSH task ID %d status: processing delete deployment for instance %s: Description: %s`, taskProcessing.ID, instanceID, taskProcessing.Description))
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
			})
		})

		Context("and when there are two tasks and the second task has failed", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsOKWithJSON(boshdirector.BoshTasks{taskFailed, taskDone}),
					mockbosh.TaskOutput(taskDone.ID).RespondsOKWith(""),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			Describe("response body", func() {
				It("has state failed", func() {
					Expect(actualResponse["state"]).To(Equal(string(brokerapi.Failed)))
				})

				It("has generic error description", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						"Instance deletion failed: There was a problem completing your request. Please contact your operations team providing the following information:",
					))
				})

				It("has description with broker request id", func() {
					Expect(actualResponse["description"]).To(MatchRegexp(
						`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
					))
				})

				It("has description with service name", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						fmt.Sprintf("service: %s", serviceName),
					))
				})

				It("has description with service instance guid", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						fmt.Sprintf("service-instance-guid: %s", instanceID),
					))
				})

				It("includes the operation type", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						"operation: delete",
					))
				})

				It("includes the failed bosh task ID", func() {
					Expect(actualResponse["description"]).To(ContainSubstring(
						fmt.Sprintf("task-id: %d", taskFailed.ID),
					))
				})
			})

			It("logs with a request ID", func() {
				regexpString := logRegexpStringWithRequestIDCapture(
					fmt.Sprintf(`BOSH task ID %d status: error delete deployment for instance %s: Description: %s`, taskFailed.ID, instanceID, taskFailed.Description),
				)
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
			})
		})

		Context("and when there are two done tasks and both were successful", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsOKWithJSON(boshdirector.BoshTasks{anotherTaskDone, taskDone}),
					mockbosh.TaskOutput(anotherTaskDone.ID).RespondsOKWith(""),
					mockbosh.TaskOutput(taskDone.ID).RespondsOKWith(""),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("has a description", func() {
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{
						"state":       brokerapi.Succeeded,
						"description": "Instance deletion completed",
					},
				)))
			})

			It("logs with a request ID", func() {
				regexpString := logRegexpStringWithRequestIDCapture(
					fmt.Sprintf(`BOSH task ID %d status: done delete deployment for instance %s: Description: %s`, anotherTaskDone.ID, instanceID, anotherTaskDone.Description),
				)
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
			})
		})
	})

	Context("when a lifecycle errand is added to plan config during the service instance deployment", func() {
		BeforeEach(func() {
			planID = "post-deploy-plan-id"
			operationType = broker.OperationTypeCreate

			planWithPostDeploy := config.Plan{
				ID:   planID,
				Name: "post-deploy-plan",
				LifecycleErrands: &config.LifecycleErrands{
					PostDeploy: config.Errand{
						Name: "health-check",
					},
				},
			}

			brokerConfig.ServiceCatalog.Plans = []config.Plan{planWithPostDeploy}

			runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)
		})

		Context("and the deployment task is complete", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Task(boshTaskID).RespondsWithTaskContainingState(boshdirector.TaskDone),
				)
			})

			It("responds with operation succeeded", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{"state": brokerapi.Succeeded, "description": "Instance provisioning completed"})),
				)
			})
		})

		Context("and the deployment task is incomplete", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Task(boshTaskID).RespondsWithTaskContainingState(boshdirector.TaskProcessing),
				)
			})

			It("responds with operation in progress", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{"state": brokerapi.InProgress, "description": "Instance provisioning in progress"})),
				)
			})
		})
	})

	Context("when a lifecycle errand is removed from plan config during the deployment", func() {
		var (
			taskDone       boshdirector.BoshTask
			taskProcessing boshdirector.BoshTask
		)

		BeforeEach(func() {
			planID = dedicatedPlanID
			operationType = broker.OperationTypeCreate
			postDeployErrandName = "health-check"

			contextID = "some-context-id"

			taskDone = boshdirector.BoshTask{ID: 1, State: boshdirector.TaskDone, ContextID: contextID}
			taskProcessing = boshdirector.BoshTask{ID: 2, State: boshdirector.TaskProcessing, ContextID: contextID}

			runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)
		})

		Context("and the deployment task is complete", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsWithATask(taskDone),
					mockbosh.TaskOutput(taskDone.ID).RespondsOKWith(""),
					mockbosh.Errand(deploymentName(instanceID), postDeployErrandName, `{}`).
						WithContextID(contextID).RedirectsToTask(taskProcessing.ID),
					mockbosh.Task(taskProcessing.ID).RespondsOKWithJSON(taskProcessing),
				)
			})

			It("responds with operation in progress", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{"state": brokerapi.InProgress, "description": "Instance provisioning in progress"})),
				)
			})
		})

		Context("and the deployment task is incomplete", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsWithATask(taskProcessing),
				)
			})

			It("responds with operation in progress", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{"state": brokerapi.InProgress, "description": "Instance provisioning in progress"})),
				)
			})
		})
	})

	Context("when bosh fails to get the task", func() {
		BeforeEach(func() {
			runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)
			operationType = broker.OperationTypeCreate
			boshDirector.VerifyAndMock(
				mockbosh.Task(boshTaskID).RespondsInternalServerErrorWith("bosh task failed"),
			)
		})

		It("responds with 500", func() {
			Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusInternalServerError))
		})

		Describe("response body", func() {
			It("has generic error description", func() {
				Expect(actualResponse["description"]).To(ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information:",
				))
			})

			It("has description with broker request id", func() {
				Expect(actualResponse["description"]).To(MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				))
			})

			It("has description with service name", func() {
				Expect(actualResponse["description"]).To(ContainSubstring(
					fmt.Sprintf("service: %s", serviceName),
				))
			})

			It("has description with service instance guid", func() {
				Expect(actualResponse["description"]).To(ContainSubstring(
					fmt.Sprintf("service-instance-guid: %s", instanceID),
				))
			})

			It("includes the operation type", func() {
				Expect(actualResponse["description"]).To(ContainSubstring(
					"operation: create",
				))
			})

			It("includes the bosh task ID", func() {
				Expect(actualResponse["description"]).To(ContainSubstring(
					fmt.Sprintf("task-id: %d", boshTaskID),
				))
			})
		})

		It("logs with a request ID", func() {
			regexpString := logRegexpStringWithRequestIDCapture(fmt.Sprintf(`error: error retrieving tasks from bosh, for deployment '%s'`, deploymentName(instanceID)))
			Eventually(runningBroker).Should(gbytes.Say(regexpString))
		})
	})

	Context("when the Cloud Controller does not send operation data", func() {
		BeforeEach(func() {
			runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)
			operationType = ""
		})

		It("responds with HTTP 500", func() {
			Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusInternalServerError))
		})

		Describe("response body", func() {
			It("has generic error description", func() {
				Expect(actualResponse["description"]).To(ContainSubstring(
					"There was a problem completing your request. Please contact your operations team providing the following information:",
				))
			})

			It("has description with broker request id", func() {
				Expect(actualResponse["description"]).To(MatchRegexp(
					`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
				))
			})

			It("has description with service name", func() {
				Expect(actualResponse["description"]).To(ContainSubstring(
					fmt.Sprintf("service: %s", serviceName),
				))
			})

			It("has description with service instance guid", func() {
				Expect(actualResponse["description"]).To(ContainSubstring(
					fmt.Sprintf("service-instance-guid: %s", instanceID),
				))
			})

			It("does not include the operation type", func() {
				Expect(actualResponse["description"]).NotTo(ContainSubstring(
					"operation:",
				))
			})

			It("does not include a bosh task id", func() {
				Expect(actualResponse["description"]).NotTo(ContainSubstring(
					"task-id:",
				))
			})
		})

		It("logs that the Cloud Controller version may be too old", func() {
			Eventually(runningBroker).Should(gbytes.Say("Request missing operation data, please check your Cloud Foundry version is v238+"))
		})
	})

	Context("when creating", func() {
		BeforeEach(func() {
			runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)
			operationType = broker.OperationTypeCreate
		})

		Context("while the task is in progress", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Task(boshTaskID).RespondsWithTaskContainingState(boshdirector.TaskProcessing),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("has a description", func() {
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{
						"state":       brokerapi.InProgress,
						"description": "Instance provisioning in progress",
					},
				)))
			})

			It("logs with a request ID", func() {
				regexpString := logRegexpStringWithRequestIDCapture(fmt.Sprintf(`BOSH task ID %d status: processing create deployment for instance %s`, boshTaskID, instanceID))
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
			})
		})
	})

	Context("when deleting", func() {
		BeforeEach(func() {
			runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)
			operationType = broker.OperationTypeDelete
		})

		Context("while the task is in progress", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Task(boshTaskID).RespondsWithTaskContainingState(boshdirector.TaskProcessing),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("has a description", func() {
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{
						"state":       brokerapi.InProgress,
						"description": "Instance deletion in progress",
					},
				)))
			})

			It("logs with a request ID", func() {
				regexpString := logRegexpStringWithRequestIDCapture(fmt.Sprintf(`BOSH task ID %d status: processing delete deployment for instance %s`, boshTaskID, instanceID))
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
			})
		})
	})

	Context("when updating", func() {
		BeforeEach(func() {
			runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)
			operationType = broker.OperationTypeUpdate
		})

		Context("while the task is in progress", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Task(boshTaskID).RespondsWithTaskContainingState(boshdirector.TaskProcessing),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("has a description", func() {
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{
						"state":       brokerapi.InProgress,
						"description": "Instance update in progress",
					},
				)))
			})

			It("logs with a request ID", func() {
				regexpString := logRegexpStringWithRequestIDCapture(fmt.Sprintf(`BOSH task ID %d status: processing update deployment for instance %s`, boshTaskID, instanceID))
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
			})
		})
	})

	Context("when upgrading", func() {
		BeforeEach(func() {
			runningBroker = startBrokerWithPassingStartupChecks(brokerConfig, cfAPI, boshDirector)
			operationType = broker.OperationTypeUpgrade
		})

		Context("while the task is in progress", func() {
			BeforeEach(func() {
				boshDirector.VerifyAndMock(
					mockbosh.Task(boshTaskID).RespondsWithTaskContainingState(boshdirector.TaskProcessing),
				)
			})

			It("responds with 200", func() {
				Expect(lastOperationResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("has a description", func() {
				Expect(rawResponse).To(MatchJSON(toJSONString(
					map[string]interface{}{
						"state":       brokerapi.InProgress,
						"description": "Instance upgrade in progress",
					},
				)))
			})

			It("logs with a request ID", func() {
				regexpString := logRegexpStringWithRequestIDCapture(fmt.Sprintf(`BOSH task ID %d status: processing upgrade deployment for instance %s`, boshTaskID, instanceID))
				Eventually(runningBroker).Should(gbytes.Say(regexpString))
			})
		})
	})
})

func toJSONString(obj map[string]interface{}) string {
	data, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	return string(data)
}
