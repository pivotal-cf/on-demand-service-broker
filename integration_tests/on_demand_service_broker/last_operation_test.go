// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_tests

import (
	"encoding/json"
	"fmt"
	"net/http"

	"io/ioutil"

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
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("last operation", func() {
	const (
		instanceID = "some-instance-id"
		boshTaskID = 10654
	)

	var (
		planID                string
		postDeployErrandName  string
		contextID             string
		boshUAA               *mockuaa.ClientCredentialsServer
		boshDirector          *mockbosh.MockBOSH
		cfAPI                 *mockhttp.Server
		cfUAA                 *mockuaa.ClientCredentialsServer
		runningBroker         *gexec.Session
		operationType         broker.OperationType
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
		planID = dedicatedPlanID
		postDeployErrandName = ""
		contextID = ""
		boshUAA = mockuaa.NewClientCredentialsServerTLS(boshClientID, boshClientSecret, pathToSSLCerts("cert.pem"), pathToSSLCerts("key.pem"), "bosh uaa token")
		boshDirector = mockbosh.NewWithUAA(boshUAA.URL)
		boshDirector.ExpectedAuthorizationHeader(boshUAA.ExpectedAuthorizationHeader())
		boshDirector.ExcludeAuthorizationCheck("/info")

		cfAPI = mockcfapi.New()
		cfUAA = mockuaa.NewClientCredentialsServer(cfUaaClientID, cfUaaClientSecret, "CF UAA token")
		adapter.GenerateManifest().ToReturnManifest(rawManifestWithDeploymentName(instanceID))
		operationType = ""
		brokerConfig = defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)
		actualResponse = map[string]interface{}{}
	})

	JustBeforeEach(func() {
		operationData := broker.OperationData{}
		if operationType != "" {
			operationData = broker.OperationData{
				BoshTaskID:    boshTaskID,
				OperationType: operationType,
				BoshContextID: contextID,
				PlanID:        planID,
				PostDeployErrand: broker.PostDeployErrand{
					Name: postDeployErrandName,
				},
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
				LifecycleErrands: &serviceadapter.LifecycleErrands{
					PostDeploy: serviceadapter.Errand{
						Name:      errandName,
						Instances: []string{"instance-group-name/0"},
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
				var instances struct {
					Instances []boshdirector.Instance `json:"instances"`
				}

				instance := &boshdirector.Instance{Group: "instance-group-name", ID: "0"}
				instances.Instances = append(instances.Instances, *instance)
				instancesRaw, err := json.Marshal(instances)
				Expect(err).NotTo(HaveOccurred())

				boshDirector.VerifyAndMock(
					mockbosh.TasksByContext(deploymentName(instanceID), contextID).
						RespondsOKWithJSON(boshdirector.BoshTasks{taskDone}),
					mockbosh.TaskOutput(taskDone.ID).RespondsOKWith(""),
					mockbosh.Errand(deploymentName(instanceID), errandName, string(instancesRaw)).
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
			planID = "pre-delete-plan-id"
			operationType = broker.OperationTypeDelete
			contextID = "some-context-id"

			planWithPostDeploy := config.Plan{
				ID:   planID,
				Name: "pre-delete-plan",
				LifecycleErrands: &serviceadapter.LifecycleErrands{
					PreDelete: serviceadapter.Errand{Name: errandName},
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
						WithoutContextID().RespondsOKWith(fmt.Sprintf(`{"ID":%d,"State":"processing"}`, taskProcessing.ID)),
					mockbosh.Task(taskProcessing.ID).RespondsOKWith(fmt.Sprintf(`{"ID":%d,"State":"done"}`, taskProcessing.ID)),
					mockbosh.TaskOutputEvent(taskProcessing.ID).RespondsOKWith(fmt.Sprintf(`{"ID":%d,"State":"processing"}`, taskProcessing.ID)),
					mockbosh.TaskOutput(taskProcessing.ID).RespondsOKWith(fmt.Sprintf(`{"ID":%d,"State":"processing"}`, taskProcessing.ID)),
					mockbosh.TasksByContextID(contextID).RespondsOKWith(fmt.Sprintf(`[{"ID":%d}]`, taskProcessing.ID)),
					mockbosh.Task(taskProcessing.ID).RespondsOKWith(fmt.Sprintf(`{"ID":%d,"State":"processing", "Description": "processing thing"}`, taskProcessing.ID)),
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
				LifecycleErrands: &serviceadapter.LifecycleErrands{
					PostDeploy: serviceadapter.Errand{
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

})

func toJSONString(obj map[string]interface{}) string {
	data, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	return string(data)
}
