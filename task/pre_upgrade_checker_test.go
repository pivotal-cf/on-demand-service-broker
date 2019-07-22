package task_test

import (
	. "github.com/onsi/ginkgo"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/task"
	"github.com/pivotal-cf/on-demand-service-broker/task/fakes"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
	"github.com/pkg/errors"

	. "github.com/onsi/gomega"
)

var _ = Describe("PreUpgrade", func() {
	var (
		boshClient *fakes.FakeBoshClient

		generatedManifest string
		oldManifest       []byte

		preUpgrade            task.PreUpgrade
		manifestGenerator     *fakes.FakeManifestGenerator
		defaultPlanWithErrand config.Plan
	)

	BeforeEach(func() {
		boshClient = new(fakes.FakeBoshClient)
		manifestGenerator = new(fakes.FakeManifestGenerator)

		oldManifest = []byte("name: old-manifest")
		generatedManifest = "name: new-manifest"

		manifestGenerator.GenerateManifestReturns(
			serviceadapter.MarshalledGenerateManifest{Manifest: generatedManifest},
			nil,
		)

		defaultPlanWithErrand = config.Plan{
			ID: "a-plan-id",
			LifecycleErrands: &serviceadapter.LifecycleErrands{
				PostDeploy: []serviceadapter.Errand{{
					Name: "errand-name",
				}},
			},
		}

		preUpgrade = task.NewPreUpgrade(manifestGenerator, boshClient)
	})

	Describe("ShouldUpgrade", func() {
		Context("when the manifest has changed", func() {
			It("should upgrade", func() {
				shouldUpgrade := preUpgrade.ShouldUpgrade(
					task.GenerateManifestProperties{
						DeploymentName: deploymentName,
						OldManifest:    oldManifest,
					},
					defaultPlanWithErrand,
					logger)

				Expect(shouldUpgrade).To(BeTrue())
				Expect(boshClient.GetEventsCallCount()).To(BeZero())
				Expect(boshClient.GetTaskCallCount()).To(BeZero())
				Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
			})
		})

		Context("when the manifest has not changed", func() {
			Context("when all errands have run successfully in a previous BOSH update event", func() {
				const expectedDeploymentTask = 103
				const expectedContextID = "231"
				BeforeEach(func() {
					boshClient.GetEventsReturns([]boshdirector.BoshEvent{
						{TaskId: expectedDeploymentTask},
					}, nil)
					boshClient.GetTaskReturns(boshdirector.BoshTask{
						State: boshdirector.TaskDone, ID: 3232, ContextID: expectedContextID,
					}, nil)
					boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{
						{State: boshdirector.TaskDone, ID: 1, ContextID: expectedContextID},
						{State: boshdirector.TaskDone, ID: 2, ContextID: expectedContextID},
					}, nil)
				})

				It("should skip upgrade", func() {
					shouldUpgrade := preUpgrade.ShouldUpgrade(
						task.GenerateManifestProperties{
							DeploymentName: deploymentName,
							OldManifest:    []byte(generatedManifest),
						},
						defaultPlanWithErrand,
						logger)

					_, eventType, _ := boshClient.GetEventsArgsForCall(0)
					Expect(eventType).To(Equal("update"))

					By("asserting all calls to task")
					taskId, _ := boshClient.GetTaskArgsForCall(0)
					Expect(taskId).To(Equal(expectedDeploymentTask))

					By("asserting all calls to task by context id")
					actualDeploymentName, contextID, _ := boshClient.GetNormalisedTasksByContextArgsForCall(0)
					Expect(actualDeploymentName).To(Equal(deploymentName))
					Expect(contextID).To(Equal(expectedContextID))

					Expect(shouldUpgrade).To(BeFalse())
				})
			})

			Context("when all errands have run successfully in a previous BOSH create event", func() {
				const expectedDeploymentTask = 103
				const expectedContextID = "231"
				BeforeEach(func() {
					boshClient.GetEventsReturnsOnCall(0, []boshdirector.BoshEvent{}, nil)
					boshClient.GetEventsReturnsOnCall(1, []boshdirector.BoshEvent{{TaskId: expectedDeploymentTask}}, nil)
					boshClient.GetTaskReturns(boshdirector.BoshTask{
						State: boshdirector.TaskDone, ID: 3232, ContextID: expectedContextID,
					}, nil)
					boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{
						{State: boshdirector.TaskDone, ID: 1, ContextID: expectedContextID},
						{State: boshdirector.TaskDone, ID: 2, ContextID: expectedContextID},
					}, nil)
				})

				It("should skip upgrade", func() {
					shouldUpgrade := preUpgrade.ShouldUpgrade(
						task.GenerateManifestProperties{
							DeploymentName: deploymentName,
							OldManifest:    []byte(generatedManifest),
						},
						defaultPlanWithErrand,
						logger)

					_, eventType, _ := boshClient.GetEventsArgsForCall(1)
					Expect(eventType).To(Equal("create"))

					By("asserting all calls to task")
					taskId, _ := boshClient.GetTaskArgsForCall(0)
					Expect(taskId).To(Equal(expectedDeploymentTask))

					By("asserting all calls to task by context id")
					actualDeploymentName, contextID, _ := boshClient.GetNormalisedTasksByContextArgsForCall(0)
					Expect(actualDeploymentName).To(Equal(deploymentName))
					Expect(contextID).To(Equal(expectedContextID))

					Expect(shouldUpgrade).To(BeFalse())
				})
			})

			Context("when one errand", func() {
				const expectedDeploymentTask = 103
				const expectedContextID = "231"
				const expectedNewDeploymentTask = 35
				BeforeEach(func() {
					boshClient.GetEventsReturns([]boshdirector.BoshEvent{
						{TaskId: expectedDeploymentTask},
					}, nil)
					boshClient.GetTaskReturns(boshdirector.BoshTask{
						State: boshdirector.TaskDone, ID: 3232, ContextID: expectedContextID,
					}, nil)
					boshClient.DeployReturns(expectedNewDeploymentTask, nil)
				})

				It("has failed the previous run it should upgrade", func() {
					boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{
						{State: boshdirector.TaskError, ID: 3234, ContextID: expectedContextID},
						{State: boshdirector.TaskDone, ID: 3232, ContextID: expectedContextID},
					}, nil)

					shouldUpgrade := preUpgrade.ShouldUpgrade(
						task.GenerateManifestProperties{
							DeploymentName: deploymentName,
							OldManifest:    []byte(generatedManifest),
						},
						defaultPlanWithErrand,
						logger)

					Expect(shouldUpgrade).To(BeTrue(), "expected 'shouldUpgrade' to return true")
				})

				It("has not completed the previous run it should upgrade", func() {
					boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{
						{State: boshdirector.TaskProcessing, ID: 3234, ContextID: expectedContextID},
						{State: boshdirector.TaskDone, ID: 3233, ContextID: expectedContextID},
					}, nil)

					shouldUpgrade := preUpgrade.ShouldUpgrade(
						task.GenerateManifestProperties{DeploymentName: deploymentName},
						config.Plan{},
						logger)

					Expect(shouldUpgrade).To(BeTrue())
				})
			})

			Context("when there are no post-deploy errands", func() {

				It("should skip upgrade when lifecycle errands does not have post-deploy errands defined", func() {
					shouldUpgrade := preUpgrade.ShouldUpgrade(
						task.GenerateManifestProperties{
							DeploymentName: deploymentName,
							OldManifest:    []byte(generatedManifest),
						},
						config.Plan{
							ID:               "a-plan-id",
							LifecycleErrands: &serviceadapter.LifecycleErrands{},
						},
						logger)

					Expect(shouldUpgrade).To(BeFalse())
					Expect(boshClient.GetEventsCallCount()).To(BeZero())
					Expect(boshClient.GetTaskCallCount()).To(BeZero())
					Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
				})

				It("should skip upgrade when lifecycle errands is not defined", func() {
					shouldUpgrade := preUpgrade.ShouldUpgrade(
						task.GenerateManifestProperties{
							DeploymentName: deploymentName,
							OldManifest:    []byte(generatedManifest),
						},
						config.Plan{
							ID: "a-plan-id",
						},
						logger)

					Expect(shouldUpgrade).To(BeFalse())
					Expect(boshClient.GetEventsCallCount()).To(BeZero())
					Expect(boshClient.GetTaskCallCount()).To(BeZero())
					Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
				})
			})

			When("not all errands have run", func() {
				const expectedDeploymentTask = 103
				const expectedContextID = "231"
				BeforeEach(func() {
					boshClient.GetEventsReturns([]boshdirector.BoshEvent{
						{TaskId: expectedDeploymentTask},
					}, nil)
					boshClient.GetTaskReturns(boshdirector.BoshTask{
						State: boshdirector.TaskDone, ID: 3232, ContextID: expectedContextID,
					}, nil)
					boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{
						{State: boshdirector.TaskDone, ID: 3, ContextID: expectedContextID},
					}, nil)
				})

				It("should upgrade", func() {
					shouldUpgrade := preUpgrade.ShouldUpgrade(
						task.GenerateManifestProperties{
							DeploymentName: deploymentName,
							OldManifest:    []byte(generatedManifest),
						},
						defaultPlanWithErrand,
						logger)

					Expect(shouldUpgrade).To(BeTrue())
				})
			})
		})

		Context("when manifest generation fails", func() {
			It("should upgrade", func() {

				errorMessage := "can't generate manifest"
				manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{}, errors.New(errorMessage))

				shouldUpgrade := preUpgrade.ShouldUpgrade(
					task.GenerateManifestProperties{
						DeploymentName: deploymentName,
						OldManifest:    []byte(generatedManifest),
					},
					defaultPlanWithErrand,
					logger)

				Expect(shouldUpgrade).To(BeTrue())
				Expect(logBuffer.String()).To(ContainSubstring(errorMessage))

				Expect(boshClient.GetEventsCallCount()).To(BeZero())
				Expect(boshClient.GetTaskCallCount()).To(BeZero())
				Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
			})
		})

		When("the bosh client output is unexpected", func() {
			It("should upgrade when get update events fails", func() {
				errorMessage := "failed to retrieve events"
				boshClient.GetEventsReturns([]boshdirector.BoshEvent{}, errors.New(errorMessage))

				shouldUpgrade := preUpgrade.ShouldUpgrade(
					task.GenerateManifestProperties{
						DeploymentName: deploymentName,
						OldManifest:    []byte(generatedManifest),
					},
					defaultPlanWithErrand,
					logger)

				Expect(shouldUpgrade).To(BeTrue())
				Expect(logBuffer.String()).To(ContainSubstring(errorMessage))
				Expect(boshClient.GetTaskCallCount()).To(BeZero())
				Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
			})

			It("should upgrade when get create events fails", func() {
				errorMessage := "failed to retrieve events"
				boshClient.GetEventsReturnsOnCall(0, []boshdirector.BoshEvent{}, errors.New(errorMessage))
				boshClient.GetEventsReturnsOnCall(1, []boshdirector.BoshEvent{}, errors.New(errorMessage))

				shouldUpgrade := preUpgrade.ShouldUpgrade(
					task.GenerateManifestProperties{
						DeploymentName: deploymentName,
						OldManifest:    []byte(generatedManifest),
					},
					defaultPlanWithErrand,
					logger)

				Expect(shouldUpgrade).To(BeTrue())
				Expect(logBuffer.String()).To(ContainSubstring(errorMessage))
				Expect(boshClient.GetTaskCallCount()).To(BeZero())
				Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
			})

			It("should upgrade when there are no create or update events", func() {
				shouldUpgrade := preUpgrade.ShouldUpgrade(
					task.GenerateManifestProperties{
						DeploymentName: deploymentName,
						OldManifest:    []byte(generatedManifest),
					},
					defaultPlanWithErrand,
					logger)

				Expect(shouldUpgrade).To(BeTrue())
				Expect(boshClient.GetTaskCallCount()).To(BeZero())
				Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
			})

			It("should upgrade when get task returns task without contextID", func() {
				boshClient.GetEventsReturns([]boshdirector.BoshEvent{
					{TaskId: 189},
				}, nil)
				boshClient.GetTaskReturns(boshdirector.BoshTask{ContextID: "", State: boshdirector.TaskDone, ID: 3232}, nil)

				shouldUpgrade := preUpgrade.ShouldUpgrade(
					task.GenerateManifestProperties{
						DeploymentName: deploymentName,
						OldManifest:    []byte(generatedManifest),
					},
					defaultPlanWithErrand,
					logger)

				Expect(shouldUpgrade).To(BeTrue())
				Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero(), "expected to call GetNormalisedTasksByContext")
			})

			It("should upgrade when get task returns an error", func() {
				boshClient.GetEventsReturns([]boshdirector.BoshEvent{
					{TaskId: 189},
				}, nil)
				errorMessage := "get task failed"
				boshClient.GetTaskReturns(boshdirector.BoshTask{}, errors.New(errorMessage))

				shouldUpgrade := preUpgrade.ShouldUpgrade(
					task.GenerateManifestProperties{
						DeploymentName: deploymentName,
						OldManifest:    []byte(generatedManifest),
					},
					defaultPlanWithErrand,
					logger)

				Expect(shouldUpgrade).To(BeTrue())
				Expect(logBuffer.String()).To(ContainSubstring(errorMessage))
				Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
			})

			It("should upgrade when GetNormalisedTasksByContext returns an error", func() {
				boshClient.GetEventsReturns([]boshdirector.BoshEvent{
					{TaskId: 189},
				}, nil)
				boshClient.GetTaskReturns(boshdirector.BoshTask{
					State: boshdirector.TaskDone, ID: 3232, ContextID: "12",
				}, nil)
				errorMessage := "get normalised task failed"
				boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{}, errors.New(errorMessage))

				shouldUpgrade := preUpgrade.ShouldUpgrade(
					task.GenerateManifestProperties{
						DeploymentName: deploymentName,
						OldManifest:    []byte(generatedManifest),
					},
					defaultPlanWithErrand,
					logger)

				Expect(logBuffer.String()).To(ContainSubstring(errorMessage))
				Expect(shouldUpgrade).To(BeTrue())
			})

			It("should upgrade and log when get tasks by context id returns no task", func() {
				boshClient.GetEventsReturns([]boshdirector.BoshEvent{
					{TaskId: 189},
				}, nil)
				boshClient.GetTaskReturns(boshdirector.BoshTask{
					State: boshdirector.TaskDone, ID: 3232, ContextID: "12",
				}, nil)
				boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{}, nil)

				shouldUpgrade := preUpgrade.ShouldUpgrade(
					task.GenerateManifestProperties{
						DeploymentName: deploymentName,
						OldManifest:    []byte(generatedManifest),
					},
					defaultPlanWithErrand,
					logger)

				Expect(shouldUpgrade).To(BeTrue())
				Expect(logBuffer.String()).To(ContainSubstring(`No tasks for contextID "12"`))
			})
		})
	})
})
