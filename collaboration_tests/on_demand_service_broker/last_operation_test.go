package on_demand_service_broker_test

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	brokerConfig "github.com/pivotal-cf/on-demand-service-broker/config"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"github.com/pkg/errors"
)

var _ = Describe("Last Operation", func() {

	const (
		instanceID           = "some-instance-id"
		postDeployErrandName = "post-deploy-errand"
		boshTaskID           = 6782
	)

	var (
		operationData        broker.OperationData
		processingTask       = boshdirector.BoshTask{ID: boshTaskID, State: boshdirector.TaskProcessing}
		doneTask             = boshdirector.BoshTask{ID: boshTaskID, State: boshdirector.TaskDone, Description: "succeeded"}
		failedTask           = boshdirector.BoshTask{ID: boshTaskID, State: boshdirector.TaskError, Description: "failed"}
		failedErrandTask     = boshdirector.BoshTask{ID: boshTaskID + 1, State: boshdirector.TaskError, Description: "failed"}
		doneErrandTask       = boshdirector.BoshTask{ID: boshTaskID + 1, State: boshdirector.TaskDone, Description: "errand completed"}
		processingErrandTask = boshdirector.BoshTask{ID: boshTaskID + 1, State: boshdirector.TaskProcessing, Description: "errand running"}
	)

	Context("when no lifecycle errands are configured", func() {
		BeforeEach(func() {
			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name: serviceName,
				},
			}

			StartServer(conf)
		})

		DescribeTable("depending on the operation type, responds with 200 when", func(operationType broker.OperationType, action, responseDescription string) {
			fakeBoshClient.GetTaskReturns(boshdirector.BoshTask{ID: boshTaskID, State: boshdirector.TaskProcessing}, nil)

			operationData := broker.OperationData{
				BoshTaskID:    boshTaskID,
				OperationType: operationType,
				BoshContextID: "",
				PlanID:        dedicatedPlanID,
				PostDeployErrand: broker.PostDeployErrand{
					Name: postDeployErrandName,
				},
			}

			response, bodyContent := doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct error description")
			Expect(bodyContent).To(MatchJSON(fmt.Sprintf(`
				{
					"state":       "in progress",
					"description": "%s"
				}`,
				responseDescription,
			)))

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf(`BOSH task ID %d status: processing %s deployment for instance %s`,
					boshTaskID,
					action,
					instanceID,
				),
			))
		},
			Entry("create is in progress", broker.OperationTypeCreate, "create", "Instance provisioning in progress"),
			Entry("delete is in progress", broker.OperationTypeDelete, "delete", "Instance deletion in progress"),
			Entry("update is in progress", broker.OperationTypeUpdate, "update", "Instance update in progress"),
			Entry("upgrade is in progress", broker.OperationTypeUpgrade, "upgrade", "Instance upgrade in progress"),
		)

		DescribeTable("depending on the task state, responds with 200 when", func(taskState, responseState, description string) {
			fakeBoshClient.GetTaskReturns(boshdirector.BoshTask{ID: boshTaskID, State: taskState}, nil)

			operationData := broker.OperationData{
				BoshTaskID:    boshTaskID,
				OperationType: broker.OperationTypeCreate,
				BoshContextID: "",
				PlanID:        dedicatedPlanID,
				PostDeployErrand: broker.PostDeployErrand{
					Name: postDeployErrandName,
				},
			}

			response, bodyContent := doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct error description")
			var unmarshalled map[string]interface{}
			Expect(json.Unmarshal(bodyContent, &unmarshalled)).To(Succeed())

			Expect(unmarshalled["state"]).To(Equal(responseState))

			if responseState != string(brokerapi.Failed) {
				Expect(unmarshalled["description"]).To(Equal(description))
			} else {
				Expect(unmarshalled["description"]).To(SatisfyAll(
					ContainSubstring("Instance provisioning failed: There was a problem completing your request. Please contact your operations team providing the following information:"),
					MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
					ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
					ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
					ContainSubstring("operation: create"),
					ContainSubstring(fmt.Sprintf("task-id: %d", boshTaskID)),
				))
			}

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf(`BOSH task ID %d status: %s create deployment for instance %s`,
					boshTaskID,
					taskState,
					instanceID,
				),
			))
		},
			Entry("a task is processing", boshdirector.TaskProcessing, string(brokerapi.InProgress), "Instance provisioning in progress"),
			Entry("a task is done", boshdirector.TaskDone, string(brokerapi.Succeeded), "Instance provisioning completed"),
			Entry("a task is cancelling", boshdirector.TaskCancelling, string(brokerapi.InProgress), "Instance provisioning in progress"),
			Entry("a task has timed out", boshdirector.TaskTimeout, string(brokerapi.Failed), ""),
			Entry("a task is cancelled", boshdirector.TaskCancelled, string(brokerapi.Failed), ""),
			Entry("a task has errored", boshdirector.TaskError, string(brokerapi.Failed), ""),
			Entry("a task has an unrecognised state", "other-state", string(brokerapi.Failed), ""),
		)

		It("responds with 500 if BOSH fails to get the task", func() {
			fakeBoshClient.GetTaskReturns(boshdirector.BoshTask{}, errors.New("oops"))
			operationData := broker.OperationData{
				BoshTaskID:    boshTaskID,
				OperationType: broker.OperationTypeCreate,
				BoshContextID: "",
				PlanID:        dedicatedPlanID,
				PostDeployErrand: broker.PostDeployErrand{
					Name: postDeployErrandName,
				},
			}

			response, bodyContent := doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error description")
			var unmarshalled map[string]interface{}
			Expect(json.Unmarshal(bodyContent, &unmarshalled)).To(Succeed())

			Expect(unmarshalled["description"]).To(SatisfyAll(
				ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information:"),
				MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				ContainSubstring("operation: create"),
				ContainSubstring(fmt.Sprintf("task-id: %d", boshTaskID)),
			))

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(fmt.Sprintf(`error: error retrieving tasks from bosh, for deployment 'service-instance_%s'`, instanceID)))
		})

		It("responds with 500 if Cloud Controller does not send operation data", func() {
			fakeBoshClient.GetTaskReturns(boshdirector.BoshTask{ID: boshTaskID, State: boshdirector.TaskProcessing}, nil)
			response, bodyContent := doEmptyLastOperationRequest(instanceID)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))

			By("returning the correct error description")
			var unmarshalled map[string]interface{}
			Expect(json.Unmarshal(bodyContent, &unmarshalled)).To(Succeed())

			Expect(unmarshalled["description"]).To(SatisfyAll(
				ContainSubstring("There was a problem completing your request. Please contact your operations team providing the following information:"),
				MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				Not(ContainSubstring("operation:")),
				Not(ContainSubstring("task-id")),
			))

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say("Request missing operation data, please check your Cloud Foundry version is v238+"))
		})
	})

	Context("when post-deploy errand is configured", func() {
		const (
			planID        = "post-deploy-plan-id"
			operationType = broker.OperationTypeCreate
			contextID     = "some-context-id"
			errandName    = "post-deploy-errand"
		)

		BeforeEach(func() {
			planWithPostDeploy := brokerConfig.Plan{
				ID:   planID,
				Name: "post-deploy-plan",
				LifecycleErrands: &sdk.LifecycleErrands{
					PostDeploy: []sdk.Errand{{
						Name:      errandName,
						Instances: []string{"instance-group-name/0"},
					}},
				},
			}
			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name:  serviceName,
					Plans: []brokerConfig.Plan{planWithPostDeploy},
				},
			}

			operationData = broker.OperationData{
				BoshTaskID:    doneTask.ID,
				OperationType: operationType,
				BoshContextID: contextID,
				PlanID:        planID,
				PostDeployErrand: broker.PostDeployErrand{
					Name: postDeployErrandName,
				},
			}
			StartServer(conf)
		})

		It("returns 200 when there is a single incomplete task", func() {
			fakeBoshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{processingTask}, nil)

			operationData.BoshTaskID = processingTask.ID

			response, bodyContent := doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct response description")
			Expect(bodyContent).To(MatchJSON(`
				{
					"state":       "in progress",
					"description": "Instance provisioning in progress"
				}`,
			))

			By("not running the post deploy errand")
			Expect(fakeBoshClient.RunErrandCallCount()).To(Equal(0))

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf(`BOSH task ID %d status: processing create deployment for instance %s`,
					boshTaskID,
					instanceID,
				),
			))
		})

		It("returns 200 when there is a single complete task", func() {
			fakeBoshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{doneTask}, nil)
			fakeBoshClient.RunErrandReturns(processingTask.ID, nil)
			fakeBoshClient.GetTaskReturns(processingTask, nil)

			operationData.BoshTaskID = doneTask.ID

			response, bodyContent := doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct response description")
			Expect(bodyContent).To(MatchJSON(`
				{
					"state":       "in progress",
					"description": "Instance provisioning in progress"
				}`,
			))

			By("running the post deploy errand")
			Expect(fakeBoshClient.RunErrandCallCount()).To(Equal(1))

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf(`BOSH task ID %d status: processing create deployment for instance %s: Description: %s`,
					boshTaskID,
					instanceID,
					processingTask.Description,
				),
			))
		})

		It("returns 200 when there are two tasks and the errand task is still running", func() {
			fakeBoshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{processingErrandTask, doneTask}, nil)

			operationData.BoshTaskID = doneTask.ID

			response, bodyContent := doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct response description")
			Expect(bodyContent).To(MatchJSON(`
				{
					"state":       "in progress",
					"description": "Instance provisioning in progress"
				}`,
			))

			By("not running the post deploy errand")
			Expect(fakeBoshClient.RunErrandCallCount()).To(Equal(0))

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf(`BOSH task ID %d status: processing create deployment for instance %s: Description: %s`,
					processingErrandTask.ID,
					instanceID,
					processingErrandTask.Description,
				),
			))
		})

		It("returns 200 when there are two tasks and the errand task has failed", func() {
			fakeBoshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{failedErrandTask, doneTask}, nil)

			operationData.BoshTaskID = doneTask.ID

			response, bodyContent := doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct response description")
			var parsedResponse map[string]interface{}
			Expect(json.Unmarshal(bodyContent, &parsedResponse)).To(Succeed())
			Expect(parsedResponse["state"]).To(Equal(string(brokerapi.Failed)))

			Expect(parsedResponse["description"]).To(SatisfyAll(
				ContainSubstring("Instance provisioning failed: There was a problem completing your request. Please contact your operations team providing the following information:"),
				MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				ContainSubstring("operation: create"),
				ContainSubstring(fmt.Sprintf("task-id: %d", failedErrandTask.ID)),
			))

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf(`BOSH task ID %d status: error create deployment for instance %s: Description: %s`,
					failedErrandTask.ID,
					instanceID,
					failedErrandTask.Description),
			))
		})

		It("returns 200 when there are two tasks and the errand task has succeeded", func() {
			fakeBoshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{doneErrandTask, doneTask}, nil)

			operationData.BoshTaskID = doneTask.ID

			response, bodyContent := doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct response description")
			Expect(bodyContent).To(MatchJSON(`
				{
					"state":       "succeeded",
					"description": "Instance provisioning completed"
				}`,
			))

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf(`BOSH task ID %d status: done create deployment for instance %s: Description: %s`,
					doneErrandTask.ID,
					instanceID,
					doneErrandTask.Description),
			))
		})
	})

	Context("when pre-delete errand is configured", func() {
		const (
			operationType = broker.OperationTypeDelete
			contextID     = "some-context-id"
			errandName    = "pre-delete-errand"
		)

		BeforeEach(func() {

			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name: serviceName,
				},
			}

			operationData = broker.OperationData{
				BoshTaskID:    doneTask.ID,
				OperationType: operationType,
				BoshContextID: contextID,
				Errands:       []brokerConfig.Errand{{Name: "foo"}},
			}
			StartServer(conf)
		})

		It("returns 200 when there is a single incomplete task", func() {
			fakeBoshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{processingErrandTask}, nil)

			operationData.BoshTaskID = processingErrandTask.ID

			response, bodyContent := doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct response description")
			Expect(bodyContent).To(MatchJSON(`
				{
					"state":       "in progress",
					"description": "Instance deletion in progress"
				}`,
			))

			By("not running the delete deployment")
			Expect(fakeBoshClient.DeleteDeploymentCallCount()).To(Equal(0))

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf(`BOSH task ID %d status: processing delete deployment for instance %s`,
					processingErrandTask.ID,
					instanceID,
				),
			))
		})

		It("returns 200 when there are two tasks and the delete task is still running", func() {
			fakeBoshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{processingTask, doneErrandTask}, nil)

			operationData.BoshTaskID = doneErrandTask.ID

			response, bodyContent := doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct response description")
			Expect(bodyContent).To(MatchJSON(`
				{
					"state":       "in progress",
					"description": "Instance deletion in progress"
				}`,
			))

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf(`BOSH task ID %d status: processing delete deployment for instance %s: Description: %s`,
					processingTask.ID,
					instanceID,
					processingTask.Description,
				),
			))
		})

		It("returns 200 when there are two tasks and the delete task has failed", func() {
			fakeBoshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{failedTask, doneErrandTask}, nil)

			operationData.BoshTaskID = doneErrandTask.ID

			response, bodyContent := doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct response description")
			var parsedResponse map[string]interface{}
			Expect(json.Unmarshal(bodyContent, &parsedResponse)).To(Succeed())
			Expect(parsedResponse["state"]).To(Equal(string(brokerapi.Failed)))

			Expect(parsedResponse["description"]).To(SatisfyAll(
				ContainSubstring("Instance deletion failed: There was a problem completing your request. Please contact your operations team providing the following information:"),
				MatchRegexp(`broker-request-id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
				ContainSubstring(fmt.Sprintf("service: %s", serviceName)),
				ContainSubstring(fmt.Sprintf("service-instance-guid: %s", instanceID)),
				ContainSubstring("operation: delete"),
				ContainSubstring(fmt.Sprintf("task-id: %d", failedTask.ID)),
			))

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf(`BOSH task ID %d status: error delete deployment for instance %s: Description: %s`,
					failedTask.ID,
					instanceID,
					failedTask.Description),
			))
		})

		It("runs all errands and delete the deployment", func() {
			operationData.Errands = []brokerConfig.Errand{{Name: "foo"}, {Name: "bar"}}
			By("running the first errand")
			inProgressJSON := `
				{
					"state":       "in progress",
					"description": "Instance deletion in progress"
				}`

			firstErrand := boshdirector.BoshTask{ID: 1, State: boshdirector.TaskProcessing, Description: "errand 1", Result: "result-1", ContextID: contextID}
			secondErrand := boshdirector.BoshTask{ID: 2, State: boshdirector.TaskProcessing, Description: "errand 2", Result: "result-1", ContextID: contextID}

			fakeBoshClient.GetNormalisedTasksByContextReturnsOnCall(0, boshdirector.BoshTasks{firstErrand}, nil)
			operationData.BoshTaskID = firstErrand.ID
			response, bodyContent := doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct response description")
			Expect(bodyContent).To(MatchJSON(inProgressJSON))

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf(`BOSH task ID %d status: processing delete deployment for instance %s: Description: %s`,
					firstErrand.ID,
					instanceID,
					firstErrand.Description,
				),
			))

			By("running the second errand")
			firstErrand.State = boshdirector.TaskDone
			fakeBoshClient.GetNormalisedTasksByContextReturnsOnCall(1, boshdirector.BoshTasks{firstErrand}, nil)
			fakeBoshClient.RunErrandStub = func(deploymentName string, errand string, instances []string, contextID string, log *log.Logger, reporter *boshdirector.AsyncTaskReporter) (int, error) {
				defer GinkgoRecover()
				Expect(errand).To(Equal("bar"))
				return secondErrand.ID, nil
			}
			fakeBoshClient.GetTaskReturns(secondErrand, nil)
			response, bodyContent = doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct response description")
			Expect(bodyContent).To(MatchJSON(inProgressJSON))

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf(`BOSH task ID %d status: processing delete deployment for instance %s: Description: %s`,
					secondErrand.ID,
					instanceID,
					secondErrand.Description,
				),
			))

			By("deleting the deployment")
			secondErrand.State = boshdirector.TaskDone
			fakeBoshClient.GetNormalisedTasksByContextReturnsOnCall(2, boshdirector.BoshTasks{secondErrand, firstErrand}, nil)
			fakeBoshClient.DeleteDeploymentReturns(processingTask.ID, nil)
			fakeBoshClient.GetTaskStub = func(taskID int, logger *log.Logger) (boshdirector.BoshTask, error) {
				defer GinkgoRecover()
				Expect(taskID).To(Equal(processingTask.ID))
				return processingTask, nil
			}

			response, bodyContent = doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct response description")
			Expect(bodyContent).To(MatchJSON(inProgressJSON))

			By("logging the appropriate message")
			Eventually(loggerBuffer).Should(gbytes.Say(
				fmt.Sprintf(`BOSH task ID %d status: processing delete deployment for instance %s: Description: %s`,
					processingTask.ID,
					instanceID,
					processingTask.Description,
				),
			))

			By("checking the deletion is complete")
			fakeBoshClient.GetNormalisedTasksByContextReturnsOnCall(3, boshdirector.BoshTasks{doneTask, secondErrand, firstErrand}, nil)

			response, bodyContent = doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("returning the correct response description")
			Expect(bodyContent).To(MatchJSON(`
				{
					"state":       "succeeded",
					"description": "Instance deletion completed"
				}`,
			))
		})
	})

	Context("depending on the setting of the context id on the request body", func() {
		const (
			planID        = "post-deploy-plan-id"
			operationType = broker.OperationTypeCreate
			errandName    = "post-deploy-errand"
		)
		BeforeEach(func() {
			planWithPostDeploy := brokerConfig.Plan{
				ID:   planID,
				Name: "post-deploy-plan",
				LifecycleErrands: &sdk.LifecycleErrands{
					PostDeploy: []sdk.Errand{{
						Name:      errandName,
						Instances: []string{"instance-group-name/0"},
					}},
				},
			}

			conf := brokerConfig.Config{
				Broker: brokerConfig.Broker{
					Port: serverPort, Username: brokerUsername, Password: brokerPassword,
				},
				ServiceCatalog: brokerConfig.ServiceOffering{
					Name:  serviceName,
					Plans: []brokerConfig.Plan{planWithPostDeploy},
				},
			}
			StartServer(conf)

		})

		It("doesn't run the lifecycle errands when it's empty", func() {
			operationData = broker.OperationData{
				BoshTaskID:    boshTaskID,
				OperationType: operationType,
				PlanID:        planID,
			}
			fakeBoshClient.GetTaskReturns(doneTask, nil)

			operationData.BoshTaskID = doneTask.ID

			response, bodyContent := doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("not running the post deploy errand")
			Expect(fakeBoshClient.RunErrandCallCount()).To(Equal(0))

			By("returning the correct response description")
			Expect(bodyContent).To(MatchJSON(`
				{
					"state":       "succeeded",
					"description": "Instance provisioning completed"
				}`,
			))
		})

		It("runs the lifecycle errands when it's set", func() {
			operationData = broker.OperationData{
				BoshTaskID:    boshTaskID,
				OperationType: operationType,
				BoshContextID: "some-context-id",
				PlanID:        planID,
				Errands:       []brokerConfig.Errand{{Name: "foo"}},
			}

			fakeBoshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{doneTask}, nil)
			fakeBoshClient.RunErrandReturns(processingTask.ID, nil)
			fakeBoshClient.GetTaskReturns(processingTask, nil)

			operationData.BoshTaskID = doneTask.ID

			response, bodyContent := doLastOperationRequest(instanceID, operationData)

			By("returning the correct HTTP status code")
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			By("running the post deploy errand")
			Expect(fakeBoshClient.RunErrandCallCount()).To(Equal(1))

			By("returning the correct response description")
			Expect(bodyContent).To(MatchJSON(`
				{
					"state":       "in progress",
					"description": "Instance provisioning in progress"
				}`,
			))
		})
	})

})

func doLastOperationRequest(instanceID string, operationData broker.OperationData) (*http.Response, []byte) {
	lastOperationURL := fmt.Sprintf("http://%s/v2/service_instances/%s/last_operation", serverURL, instanceID)

	operationDataBytes, err := json.Marshal(operationData)
	Expect(err).NotTo(HaveOccurred())
	lastOperationURL = fmt.Sprintf("%s?operation=%s", lastOperationURL, url.QueryEscape(string(operationDataBytes)))

	return doRequest(http.MethodGet, lastOperationURL, nil)
}

func doEmptyLastOperationRequest(instanceID string) (*http.Response, []byte) {
	lastOperationURL := fmt.Sprintf("http://%s/v2/service_instances/%s/last_operation", serverURL, instanceID)
	return doRequest(http.MethodGet, lastOperationURL, nil)
}
