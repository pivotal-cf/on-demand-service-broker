// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker_test

import (
	"errors"
	"fmt"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("Lifecycle runner", func() {
	const (
		deploymentName       = "some-deployment"
		contextID            = "some-uuid"
		planID               = "some-plan-id"
		errand1              = "some-errand"
		errand2              = "another-errand"
		anotherPlanID        = "another-plan-id"
		planIDWithoutErrands = "without-errands-plan-id"
	)

	var errandInstances = []string{"some-instance/0"}

	plans := config.Plans{
		config.Plan{
			ID: planID,
			LifecycleErrands: &sdk.LifecycleErrands{
				PostDeploy: []sdk.Errand{{
					Name:      errand1,
					Instances: errandInstances,
				}},
			},
		},
		config.Plan{
			ID: anotherPlanID,
			LifecycleErrands: &sdk.LifecycleErrands{
				PostDeploy: []sdk.Errand{{
					Name: errand2,
				}},
			},
		},
		config.Plan{
			ID: planIDWithoutErrands,
		},
	}

	taskProcessing := boshdirector.BoshTask{ID: 1, State: boshdirector.TaskProcessing, Description: "snapshot deployment", Result: "result-1", ContextID: contextID}
	taskErrored := boshdirector.BoshTask{ID: 2, State: boshdirector.TaskError, Description: "snapshot deployment", Result: "result-1", ContextID: contextID}
	taskComplete := boshdirector.BoshTask{ID: 3, State: boshdirector.TaskDone, Description: "snapshot deployment", Result: "result-1", ContextID: contextID}

	var deployRunner broker.LifeCycleRunner
	var logger *log.Logger
	var operationData broker.OperationData

	BeforeEach(func() {
		deployRunner = broker.NewLifeCycleRunner(
			boshClient,
			plans,
		)

		logger = loggerFactory.NewWithRequestID()
	})

	Describe("post-deploy errand", func() {
		BeforeEach(func() {
			operationData = broker.OperationData{
				BoshContextID: contextID,
				OperationType: broker.OperationTypeCreate,
				PlanID:        planID,
			}
		})

		It("runs all errands in order when the deployment is complete", func() {
			operationData = broker.OperationData{
				BoshContextID: contextID,
				OperationType: broker.OperationTypeCreate,
				Errands:       []config.Errand{{Name: "some-errand"}, {Name: "some-other-errand"}},
			}
			firstErrand := boshdirector.BoshTask{ID: 1, State: boshdirector.TaskProcessing, Description: "errand 1", Result: "result-1", ContextID: contextID}
			secondErrand := boshdirector.BoshTask{ID: 2, State: boshdirector.TaskProcessing, Description: "errand 2", Result: "result-1", ContextID: contextID}
			boshClient.GetTaskStub = func(id int, l *log.Logger) (boshdirector.BoshTask, error) {
				switch id {
				case secondErrand.ID:
					return secondErrand, nil
				case firstErrand.ID:
					return firstErrand, nil
				default:
					return boshdirector.BoshTask{}, fmt.Errorf("unexpected task id %d", id)
				}
			}

			By("waiting the deployment to complete")
			boshClient.GetNormalisedTasksByContextReturnsOnCall(0, boshdirector.BoshTasks{taskProcessing}, nil)
			task, err := deployRunner.GetTask(deploymentName, operationData, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(task).To(Equal(taskProcessing))

			By("running the first errand")
			boshClient.GetNormalisedTasksByContextReturnsOnCall(1, boshdirector.BoshTasks{taskComplete}, nil)
			boshClient.RunErrandReturns(firstErrand.ID, nil)

			task, err = deployRunner.GetTask(deploymentName, operationData, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(task).To(Equal(firstErrand))

			depName, errandName, instances, ctxID, _, _ := boshClient.RunErrandArgsForCall(0)
			Expect(depName).To(Equal(deploymentName))
			Expect(errandName).To(Equal(operationData.Errands[0].Name))
			Expect(instances).To(Equal(operationData.Errands[0].Instances))
			Expect(ctxID).To(Equal(operationData.BoshContextID))

			firstErrand.State = boshdirector.TaskDone

			By("running the second errand")
			boshClient.GetNormalisedTasksByContextReturnsOnCall(2, boshdirector.BoshTasks{firstErrand, taskComplete}, nil)
			boshClient.RunErrandReturns(secondErrand.ID, nil)

			task, err = deployRunner.GetTask(deploymentName, operationData, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(task).To(Equal(secondErrand))

			depName, errandName, instances, ctxID, _, _ = boshClient.RunErrandArgsForCall(1)
			Expect(depName).To(Equal(deploymentName))
			Expect(errandName).To(Equal(operationData.Errands[1].Name))
			Expect(instances).To(Equal(operationData.Errands[1].Instances))
			Expect(ctxID).To(Equal(operationData.BoshContextID))

			secondErrand.State = boshdirector.TaskDone

			By("returning the last task")
			boshClient.GetNormalisedTasksByContextReturnsOnCall(3, boshdirector.BoshTasks{secondErrand, firstErrand, taskComplete}, nil)
			task, err = deployRunner.GetTask(deploymentName, operationData, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(task).To(Equal(secondErrand))

			Expect(boshClient.RunErrandCallCount()).To(Equal(2))
			Expect(logBuffer.String()).To(BeEmpty())
		})

		It("returns the errored task when the deployment errors", func() {
			boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskErrored}, nil)
			task, _ := deployRunner.GetTask(deploymentName, operationData, logger)
			Expect(task.State).To(Equal(boshdirector.TaskError))
		})

		It("returns an error when there are no tasks", func() {
			boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{}, nil)
			_, err := deployRunner.GetTask(deploymentName, operationData, logger)
			Expect(err).To(MatchError("no tasks found for context id: " + contextID))
		})

		Context("when the deployment task is done", func() {
			var task boshdirector.BoshTask
			BeforeEach(func() {
				boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskComplete}, nil)
			})

			It("logs and does not run errands when the post-deploy errand and plan id are absent in operation data", func() {
				operationData.PlanID = ""
				operationData.Errands = []config.Errand{}
				operationData.PostDeployErrand.Name = ""
				task, _ = deployRunner.GetTask(deploymentName, operationData, logger)

				Expect(logBuffer.String()).To(ContainSubstring("can't determine lifecycle errands, neither PlanID nor PostDeployErrand.Name is present"))
				Expect(boshClient.RunErrandCallCount()).To(Equal(0))
				Expect(task).To(Equal(taskComplete))
			})

			Context("and the post deploy errand fails", func() {
				BeforeEach(func() {
					operationData = broker.OperationData{
						BoshContextID: "some-uuid",
						OperationType: broker.OperationTypeCreate,
						Errands:       []config.Errand{{Name: "foo"}},
					}
				})

				It("returns the failed task when the errands fails", func() {
					boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskErrored, taskComplete}, nil)
					task, _ = deployRunner.GetTask(deploymentName, operationData, logger)
					Expect(task.State).To(Equal(boshdirector.TaskError))
				})

				It("returns an error when running the errand errors", func() {
					boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskComplete}, nil)
					boshClient.RunErrandReturns(0, errors.New("some errand err"))

					_, err := deployRunner.GetTask(deploymentName, operationData, logger)
					Expect(err).To(MatchError("some errand err"))
				})

				It("returns an error when bosh cannot retrieve the task", func() {
					boshClient.GetTaskReturns(boshdirector.BoshTask{}, errors.New("some err"))
					_, err := deployRunner.GetTask(deploymentName, operationData, logger)
					Expect(err).To(MatchError("some err"))
				})
			})
		})

		Context("when getting tasks errors", func() {
			BeforeEach(func() {
				boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{}, errors.New("some err"))
			})

			It("returns an error", func() {
				_, err := deployRunner.GetTask(deploymentName, operationData, logger)
				Expect(err).To(MatchError("some err"))
			})
		})

		DescribeTable("for different operation types",
			func(operationType broker.OperationType, errandRuns bool) {
				operationData := broker.OperationData{
					OperationType: operationType,
					BoshContextID: contextID,
					PlanID:        planID,
					Errands:       []config.Errand{{Name: "foo"}},
				}
				boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskComplete}, nil)
				deployRunner.GetTask(deploymentName, operationData, logger)

				if errandRuns {
					Expect(boshClient.RunErrandCallCount()).To(Equal(1))
				} else {
					Expect(boshClient.RunErrandCallCount()).To(Equal(0))
				}
			},
			Entry("create runs errand", broker.OperationTypeCreate, true),
			Entry("update runs errand", broker.OperationTypeUpdate, true),
			Entry("upgrade runs errand", broker.OperationTypeUpgrade, true),
			Entry("recreate runs errand", broker.OperationTypeRecreate, true),
			Entry("delete does not run errand", broker.OperationTypeDelete, false),
		)

		Context("when the broker receives old-style operation data (without Errands field)", func() {
			It("runs the post-deploy errand", func() {
				operationData = broker.OperationData{
					BoshContextID: contextID,
					OperationType: broker.OperationTypeCreate,
					PostDeployErrand: broker.PostDeployErrand{
						Name: "some-errand",
					},
					Errands: []config.Errand{},
				}
				firstErrand := boshdirector.BoshTask{ID: 1, State: boshdirector.TaskProcessing, Description: "errand 1", Result: "result-1", ContextID: contextID}
				boshClient.GetTaskStub = func(id int, l *log.Logger) (boshdirector.BoshTask, error) {
					if id == firstErrand.ID {
						return firstErrand, nil
					}

					return boshdirector.BoshTask{}, fmt.Errorf("unexpected task id %d", id)
				}

				boshClient.RunErrandReturns(firstErrand.ID, nil)
				boshClient.GetNormalisedTasksByContextReturnsOnCall(0, boshdirector.BoshTasks{taskComplete}, nil)
				task, err := deployRunner.GetTask(deploymentName, operationData, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(task).To(Equal(firstErrand))

				Expect(logBuffer.String()).To(BeEmpty())
			})
		})

	})

	Describe("pre-delete errand", func() {
		BeforeEach(func() {
			operationData = broker.OperationData{
				BoshContextID: contextID,
				OperationType: broker.OperationTypeDelete,
				Errands:       []config.Errand{{Name: "a-cool-errand"}},
			}
		})

		Context("when a first task is incomplete", func() {
			BeforeEach(func() {
				boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskProcessing}, nil)
			})

			It("returns the processing task", func() {
				task, _ := deployRunner.GetTask(deploymentName, operationData, logger)
				Expect(task.State).To(Equal(boshdirector.TaskProcessing))
			})
		})

		Context("when the first task has errored", func() {
			BeforeEach(func() {
				boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskErrored}, nil)
			})

			It("returns the errored task", func() {
				task, _ := deployRunner.GetTask(deploymentName, operationData, logger)
				Expect(task.State).To(Equal(boshdirector.TaskError))
			})
		})

		Context("when a first task cannot be found", func() {
			BeforeEach(func() {
				boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{}, nil)
			})

			It("returns an error", func() {
				_, err := deployRunner.GetTask(deploymentName, operationData, logger)
				Expect(err).To(MatchError("no tasks found for context id: " + contextID))
			})
		})

		Context("when a first task is complete", func() {
			var task boshdirector.BoshTask

			BeforeEach(func() {
				boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskComplete}, nil)
				boshClient.DeleteDeploymentReturns(taskProcessing.ID, nil)
				boshClient.GetTaskReturns(taskProcessing, nil)
				task, _ = deployRunner.GetTask(deploymentName, operationData, logger)
			})

			It("runs bosh delete deployment ", func() {
				Expect(boshClient.DeleteDeploymentCallCount()).To(Equal(1))
			})

			It("deletes the correct deployment", func() {
				deletedDeploymentName, _, _, _ := boshClient.DeleteDeploymentArgsForCall(0)
				Expect(deletedDeploymentName).To(Equal(deploymentName))
			})

			It("runs the delete deployment with the correct contextID", func() {
				_, ctxID, _, _ := boshClient.DeleteDeploymentArgsForCall(0)
				Expect(ctxID).To(Equal(contextID))
			})

			It("returns the post deploy errand processing task", func() {
				Expect(task.ID).To(Equal(taskProcessing.ID))
				Expect(task.State).To(Equal(boshdirector.TaskProcessing))
			})

			Context("and running bosh delete deployment fails", func() {
				BeforeEach(func() {
					boshClient.DeleteDeploymentReturns(0, errors.New("some err"))
				})

				It("returns an error", func() {
					_, err := deployRunner.GetTask(deploymentName, operationData, logger)
					Expect(err).To(MatchError("some err"))
				})
			})
		})

		Context("when there are two tasks for the context id", func() {
			BeforeEach(func() {
				boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskProcessing, taskComplete}, nil)
			})

			It("returns the latest task", func() {
				task, _ := deployRunner.GetTask(deploymentName, operationData, logger)
				Expect(task).To(Equal(taskProcessing))
			})
		})

		It("runs all errands in order and trigger the delete deployment", func() {
			operationData = broker.OperationData{
				BoshContextID: contextID,
				OperationType: broker.OperationTypeDelete,
				Errands:       []config.Errand{{Name: "some-errand"}, {Name: "some-other-errand"}},
			}
			firstErrand := boshdirector.BoshTask{ID: 1, State: boshdirector.TaskProcessing, Description: "errand 1", Result: "result-1", ContextID: contextID}
			secondErrand := boshdirector.BoshTask{ID: 2, State: boshdirector.TaskProcessing, Description: "errand 2", Result: "result-1", ContextID: contextID}
			boshClient.GetTaskStub = func(id int, l *log.Logger) (boshdirector.BoshTask, error) {
				switch id {
				case secondErrand.ID:
					return secondErrand, nil
				case taskProcessing.ID:
					return taskProcessing, nil
				default:
					return boshdirector.BoshTask{}, fmt.Errorf("unexpected task id %d", id)
				}
			}

			boshClient.GetNormalisedTasksByContextReturnsOnCall(0, boshdirector.BoshTasks{firstErrand}, nil)
			task, _ := deployRunner.GetTask(deploymentName, operationData, logger)
			Expect(task).To(Equal(firstErrand))

			firstErrand.State = boshdirector.TaskDone
			boshClient.GetNormalisedTasksByContextReturnsOnCall(1, boshdirector.BoshTasks{firstErrand}, nil)
			boshClient.RunErrandReturns(secondErrand.ID, nil)

			task, err := deployRunner.GetTask(deploymentName, operationData, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(task).To(Equal(secondErrand))

			secondErrand.State = boshdirector.TaskDone

			boshClient.GetNormalisedTasksByContextReturnsOnCall(2, boshdirector.BoshTasks{secondErrand, firstErrand}, nil)
			boshClient.DeleteDeploymentReturns(taskProcessing.ID, nil)
			task, err = deployRunner.GetTask(deploymentName, operationData, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(boshClient.DeleteDeploymentCallCount()).To(Equal(1))
			Expect(task).To(Equal(taskProcessing))

			boshClient.GetNormalisedTasksByContextReturnsOnCall(3, boshdirector.BoshTasks{taskComplete, secondErrand, firstErrand}, nil)
			task, err = deployRunner.GetTask(deploymentName, operationData, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(task).To(Equal(taskComplete))
		})

		Context("when getting tasks errors", func() {
			BeforeEach(func() {
				boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{}, errors.New("some err"))
			})

			It("returns an error", func() {
				_, err := deployRunner.GetTask(deploymentName, operationData, logger)
				Expect(err).To(MatchError("some err"))
			})
		})

		Context("when the broker receives old-style operation data (without Errands field)", func() {
			It("runs the pre-delete errand and deletes the deployment", func() {
				operationData = broker.OperationData{
					BoshContextID: contextID,
					OperationType: broker.OperationTypeDelete,
					PreDeleteErrand: broker.PreDeleteErrand{
						Name: "some-errand",
					},
					Errands: []config.Errand{},
				}
				firstErrand := boshdirector.BoshTask{ID: 1, State: boshdirector.TaskProcessing, Description: "errand 1", Result: "result-1", ContextID: contextID}
				boshClient.GetTaskStub = func(id int, l *log.Logger) (boshdirector.BoshTask, error) {
					if id == taskProcessing.ID {
						return taskProcessing, nil
					}

					return boshdirector.BoshTask{}, fmt.Errorf("unexpected task id %d", id)
				}

				boshClient.GetNormalisedTasksByContextReturnsOnCall(0, boshdirector.BoshTasks{firstErrand}, nil)
				task, _ := deployRunner.GetTask(deploymentName, operationData, logger)
				Expect(task).To(Equal(firstErrand))

				firstErrand.State = boshdirector.TaskDone

				boshClient.GetNormalisedTasksByContextReturnsOnCall(1, boshdirector.BoshTasks{firstErrand}, nil)
				boshClient.DeleteDeploymentReturns(taskProcessing.ID, nil)
				task, err := deployRunner.GetTask(deploymentName, operationData, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(boshClient.DeleteDeploymentCallCount()).To(Equal(1))
				Expect(task).To(Equal(taskProcessing))

				boshClient.GetNormalisedTasksByContextReturnsOnCall(2, boshdirector.BoshTasks{taskComplete, firstErrand}, nil)
				task, err = deployRunner.GetTask(deploymentName, operationData, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(task).To(Equal(taskComplete))
			})
		})
	})

	When("BoshContextID is not set", func() {
		It("returns the task using the task id", func() {
			operationData = broker.OperationData{
				BoshTaskID: taskComplete.ID,
			}
			boshClient.GetTaskReturns(taskComplete, nil)

			task, err := deployRunner.GetTask(deploymentName, operationData, logger)

			Expect(err).NotTo(HaveOccurred())
			Expect(task).To(Equal(taskComplete))
		})
	})
})
