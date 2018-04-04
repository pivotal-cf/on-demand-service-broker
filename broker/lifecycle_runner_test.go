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
				PostDeploy: sdk.Errand{
					Name:      errand1,
					Instances: errandInstances,
				},
			},
		},
		config.Plan{
			ID: anotherPlanID,
			LifecycleErrands: &sdk.LifecycleErrands{
				PostDeploy: sdk.Errand{
					Name: errand2,
				},
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
		Context("when operation data has a context id", func() {
			BeforeEach(func() {
				operationData = broker.OperationData{
					BoshContextID: contextID,
					OperationType: broker.OperationTypeCreate,
					PlanID:        planID,
				}
			})

			Context("and the deployment task is processing", func() {
				BeforeEach(func() {
					boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskProcessing}, nil)
				})

				It("returns the processing task", func() {
					task, _ := deployRunner.GetTask(deploymentName, operationData, logger)
					Expect(task.State).To(Equal(boshdirector.TaskProcessing))
				})
			})

			Context("and the deployment task has errored", func() {
				BeforeEach(func() {
					boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskErrored}, nil)
				})

				It("returns the errored task", func() {
					task, _ := deployRunner.GetTask(deploymentName, operationData, logger)
					Expect(task.State).To(Equal(boshdirector.TaskError))
				})
			})

			Context("when the deployment task cannot be found", func() {
				BeforeEach(func() {
					boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{}, nil)
				})

				It("returns an error", func() {
					_, err := deployRunner.GetTask(deploymentName, operationData, logger)
					Expect(err).To(MatchError("no tasks found for context id: " + contextID))
				})
			})

			Context("when the deployment task is done", func() {
				var task boshdirector.BoshTask
				BeforeEach(func() {
					boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskComplete}, nil)
				})

				Context("and the post-deploy errand and plan id are absent in operation data", func() {
					BeforeEach(func() {
						operationData.PlanID = ""
						operationData.PostDeployErrand.Name = ""
						task, _ = deployRunner.GetTask(deploymentName, operationData, logger)
					})
					It("logs that the plan id and errand are absent", func() {
						Expect(logBuffer.String()).To(ContainSubstring("can't determine lifecycle errands, neither PlanID nor PostDeployErrand.Name is present"))
					})

					It("does not run a post deploy errand", func() {
						Expect(boshClient.RunErrandCallCount()).To(Equal(0))
					})

					It("returns the completed task", func() {
						Expect(task).To(Equal(taskComplete))
					})
				})

				Context("and the post-deploy errand is present in the operation data", func() {
					BeforeEach(func() {
						var err error
						deployRunner = broker.NewLifeCycleRunner(boshClient, plans)
						operationData = broker.OperationData{
							BoshContextID: contextID,
							OperationType: broker.OperationTypeCreate,
							PostDeployErrand: broker.PostDeployErrand{
								Name:      errand1,
								Instances: errandInstances,
							},
						}

						task, err = deployRunner.GetTask(deploymentName, operationData, logger)
						Expect(err).NotTo(HaveOccurred())
					})

					It("runs the post-deploy errand set in the operation data", func() {
						Expect(boshClient.RunErrandCallCount()).To(Equal(1))
						name, expectedErrand, expectedErrandInstances, context, _, _ := boshClient.RunErrandArgsForCall(0)
						Expect(name).To(Equal(deploymentName))
						Expect(expectedErrand).To(Equal(errand1))
						Expect(expectedErrandInstances).To(Equal(errandInstances))
						Expect(context).To(Equal(contextID))
					})
				})

				Context("and the plan id is present", func() {
					Context("and the plan is configured with post deploy errand", func() {
						BeforeEach(func() {
							var err error
							deployRunner = broker.NewLifeCycleRunner(boshClient, plans)
							operationData = broker.OperationData{
								BoshContextID: contextID,
								OperationType: broker.OperationTypeCreate,
								PlanID:        planID,
							}

							task, err = deployRunner.GetTask(deploymentName, operationData, logger)
							Expect(err).NotTo(HaveOccurred())
						})
						It("uses the config to determine which errand to run", func() {
							Expect(boshClient.RunErrandCallCount()).To(Equal(1))
							name, expectedErrand, expectedErrandInstances, context, _, _ := boshClient.RunErrandArgsForCall(0)
							Expect(name).To(Equal(deploymentName))
							Expect(expectedErrand).To(Equal(errand1))
							Expect(expectedErrandInstances).To(Equal(errandInstances))
							Expect(context).To(Equal(contextID))
						})
					})
				})

				Context("and a post deploy is configured", func() {
					BeforeEach(func() {
						boshClient.RunErrandReturns(taskProcessing.ID, nil)
						boshClient.GetTaskReturns(taskProcessing, nil)
						task, _ = deployRunner.GetTask(deploymentName, operationData, logger)
					})

					It("runs the post deploy errand", func() {
						Expect(boshClient.RunErrandCallCount()).To(Equal(1))
					})

					It("runs the correct errand", func() {
						_, errandName, _, _, _, _ := boshClient.RunErrandArgsForCall(0)
						Expect(errandName).To(Equal(errand1))
					})

					It("runs the errand with the correct contextID", func() {
						_, _, _, ctxID, _, _ := boshClient.RunErrandArgsForCall(0)
						Expect(ctxID).To(Equal(contextID))
					})

					It("runs the errand with the correct errandInstances", func() {
						_, _, expectedErrandInstances, _, _, _ := boshClient.RunErrandArgsForCall(0)
						Expect(expectedErrandInstances).To(Equal(errandInstances))
					})

					It("returns the post deploy errand processing task", func() {
						Expect(task.ID).To(Equal(taskProcessing.ID))
						Expect(task.State).To(Equal(boshdirector.TaskProcessing))
					})

					Context("and a post deploy errand is incomplete", func() {
						BeforeEach(func() {
							boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskProcessing, taskComplete}, nil)
							task, _ = deployRunner.GetTask(deploymentName, operationData, logger)
						})

						It("returns the processing task", func() {
							Expect(task.State).To(Equal(boshdirector.TaskProcessing))
						})

						It("does not run a post deploy errand again", func() {
							Expect(boshClient.RunErrandCallCount()).To(Equal(1))
						})
					})

					Context("and a post deploy errand is complete", func() {
						BeforeEach(func() {
							boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskComplete, taskComplete}, nil)
							task, _ = deployRunner.GetTask(deploymentName, operationData, logger)
						})

						It("returns the complete task", func() {
							Expect(task.State).To(Equal(boshdirector.TaskDone))
						})

						It("does not run a post deploy errand again", func() {
							Expect(boshClient.RunErrandCallCount()).To(Equal(1))
						})
					})

					Context("and the post deploy errand fails", func() {
						BeforeEach(func() {
							boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{taskErrored, taskComplete}, nil)
							task, _ = deployRunner.GetTask(deploymentName, operationData, logger)
						})

						It("returns the failed task", func() {
							Expect(task.State).To(Equal(boshdirector.TaskError))
						})

						It("does not run a post deploy errand again", func() {
							Expect(boshClient.RunErrandCallCount()).To(Equal(1))
						})
					})

					Context("and when running the errand errors", func() {
						BeforeEach(func() {
							boshClient.RunErrandReturns(0, errors.New("some errand err"))
						})

						It("returns an error", func() {
							_, err := deployRunner.GetTask(deploymentName, operationData, logger)
							Expect(err).To(MatchError("some errand err"))
						})
					})

					Context("and the errand task cannot be found", func() {
						BeforeEach(func() {
							boshClient.GetTaskReturns(boshdirector.BoshTask{}, errors.New("some err"))
						})

						It("returns an error", func() {
							_, err := deployRunner.GetTask(deploymentName, operationData, logger)
							Expect(err).To(MatchError("some err"))
						})
					})
				})

				Context("and the plan cannot be found", func() {
					BeforeEach(func() {
						opData := operationData
						opData.PlanID = "non-existent-plan"
						task, _ = deployRunner.GetTask(deploymentName, opData, logger)
					})

					It("logs that it can't find plan", func() {
						Expect(logBuffer.String()).To(ContainSubstring("can't determine lifecycle errands, plan with id non-existent-plan not found"))
					})

					It("does not run a post deploy errand", func() {
						Expect(boshClient.RunErrandCallCount()).To(Equal(0))
					})

					It("returns the completed task", func() {
						Expect(task).To(Equal(taskComplete))
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
		})

		Context("when operation data has no context id", func() {
			operationDataWithoutContextID := broker.OperationData{BoshTaskID: taskProcessing.ID, OperationType: broker.OperationTypeCreate}

			BeforeEach(func() {
				boshClient.GetTaskReturns(taskProcessing, nil)
			})

			It("calls get tasks with the correct id", func() {
				deployRunner.GetTask(deploymentName, operationDataWithoutContextID, logger)

				Expect(boshClient.GetTaskCallCount()).To(Equal(1))
				actualTaskID, _ := boshClient.GetTaskArgsForCall(0)
				Expect(actualTaskID).To(Equal(taskProcessing.ID))
			})

			It("returns the processing task", func() {
				task, _ := deployRunner.GetTask(deploymentName, operationDataWithoutContextID, logger)

				Expect(task).To(Equal(taskProcessing))
			})

			It("does not error", func() {
				_, err := deployRunner.GetTask(deploymentName, operationDataWithoutContextID, logger)

				Expect(err).ToNot(HaveOccurred())
			})

			Context("and bosh client returns an error", func() {
				BeforeEach(func() {
					boshClient.GetTaskReturns(boshdirector.BoshTask{}, errors.New("error getting tasks"))
				})

				It("returns the error", func() {
					_, err := deployRunner.GetTask(deploymentName, operationDataWithoutContextID, logger)

					Expect(err).To(MatchError("error getting tasks"))
				})
			})
		})

		DescribeTable("for different operation types",
			func(operationType broker.OperationType, errandRuns bool) {
				operationData := broker.OperationData{OperationType: operationType, BoshContextID: contextID, PlanID: planID}
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
			Entry("delete does not run errand", broker.OperationTypeDelete, false),
		)
	})

	Describe("pre-delete errand", func() {
		BeforeEach(func() {
			operationData = broker.OperationData{
				BoshContextID: contextID,
				OperationType: broker.OperationTypeDelete,
				Errands:       []broker.Errand{{Name: "a-cool-errand"}},
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
				Errands:       []broker.Errand{{Name: "some-errand"}, {Name: "some-other-errand"}},
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
	})
})
