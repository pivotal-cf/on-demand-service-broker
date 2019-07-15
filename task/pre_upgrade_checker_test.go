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

		Context("when the manifest has not changed", func() {
			Context("when all errands have run successfully in the previous run", func() {
				const expectedDeploymentTask = 103
				const expectedContextID = "231"
				BeforeEach(func() {
					boshClient.GetErrandEventsReturns([]boshdirector.BoshEvent{
						{TaskId: 9999},
					}, nil)
					boshClient.GetErrandEventsReturns([]boshdirector.BoshEvent{
						{TaskId: 9999},
					}, nil)
					boshClient.GetUpdatesEventsReturns([]boshdirector.BoshEvent{
						{TaskId: expectedDeploymentTask},
					}, nil)
					boshClient.GetTaskReturns(boshdirector.BoshTask{
						State: boshdirector.TaskDone, ID: 3232, ContextID: expectedContextID,
					}, nil)
					boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{
						{State: boshdirector.TaskDone, ID: 1, ContextID: expectedContextID},
						{State: boshdirector.TaskDone, ID: 2, ContextID: expectedContextID},
						{State: boshdirector.TaskDone, ID: 3, ContextID: expectedContextID},
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
					boshClient.GetUpdatesEventsReturns([]boshdirector.BoshEvent{
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
						{State: boshdirector.TaskDone, ID: 3233, ContextID: expectedContextID},
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
						{State: boshdirector.TaskDone, ID: 3232, ContextID: expectedContextID},
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
					Expect(boshClient.GetUpdatesEventsCallCount()).To(BeZero())
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
					Expect(boshClient.GetUpdatesEventsCallCount()).To(BeZero())
					Expect(boshClient.GetTaskCallCount()).To(BeZero())
					Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
				})
			})
		})

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
				Expect(boshClient.GetUpdatesEventsCallCount()).To(BeZero())
				Expect(boshClient.GetTaskCallCount()).To(BeZero())
				Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
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

				Expect(boshClient.GetUpdatesEventsCallCount()).To(BeZero())
				Expect(boshClient.GetTaskCallCount()).To(BeZero())
				Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
			})
		})

		Context("when the bosh client fails", func() {
			It("should upgrade when get update events fail", func() {
				errorMessage := "failed to retrieve events"
				boshClient.GetErrandEventsReturns([]boshdirector.BoshEvent{
					{TaskId: 9999},
				}, nil)
				boshClient.GetUpdatesEventsReturns([]boshdirector.BoshEvent{}, errors.New(errorMessage))

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

			It("should upgrade when get update events returns no events", func() {
				boshClient.GetErrandEventsReturns([]boshdirector.BoshEvent{
					{TaskId: 9999},
				}, nil)
				boshClient.GetUpdatesEventsReturns([]boshdirector.BoshEvent{}, nil)

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

			It("should upgrade when get tasks returns no tasks", func() {
				boshClient.GetErrandEventsReturns([]boshdirector.BoshEvent{
					{TaskId: 9999},
				}, nil)
				boshClient.GetUpdatesEventsReturns([]boshdirector.BoshEvent{
					{TaskId: 189},
				}, nil)
				boshClient.GetTaskReturns(boshdirector.BoshTask{}, nil)

				shouldUpgrade := preUpgrade.ShouldUpgrade(
					task.GenerateManifestProperties{
						DeploymentName: deploymentName,
						OldManifest:    []byte(generatedManifest),
					},
					defaultPlanWithErrand,
					logger)

				Expect(shouldUpgrade).To(BeTrue())
				Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
			})

			It("should upgrade when get tasks returns task without contextID", func() {
				boshClient.GetErrandEventsReturns([]boshdirector.BoshEvent{
					{TaskId: 9999},
				}, nil)
				boshClient.GetUpdatesEventsReturns([]boshdirector.BoshEvent{
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
				Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
			})

			It("should upgrade when get tasks returns an error", func() {
				boshClient.GetErrandEventsReturns([]boshdirector.BoshEvent{
					{TaskId: 9999},
				}, nil)
				boshClient.GetUpdatesEventsReturns([]boshdirector.BoshEvent{
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

			It("should upgrade when get tasks by context id returns an error", func() {
				boshClient.GetErrandEventsReturns([]boshdirector.BoshEvent{
					{TaskId: 9999},
				}, nil)
				boshClient.GetUpdatesEventsReturns([]boshdirector.BoshEvent{
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

			It("should upgrade when get tasks by context id returns no task", func() {
				boshClient.GetErrandEventsReturns([]boshdirector.BoshEvent{
					{TaskId: 9999},
				}, nil)
				boshClient.GetUpdatesEventsReturns([]boshdirector.BoshEvent{
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
			})
		})
	})
})
