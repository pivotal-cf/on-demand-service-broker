package task_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
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

		preUpgrade        task.PreUpgrade
		manifestGenerator *fakes.FakeManifestGenerator
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

		preUpgrade = task.NewPreUpgrade(manifestGenerator, boshClient)
	})

	Context("when the manifest has changed", func() {
		It("should return true", func() {
			shouldUpgrade := preUpgrade.ShouldUpgrade(
				task.GenerateManifestProperties{
					DeploymentName: deploymentName,
					OldManifest:    oldManifest,
				},
				logger)

			Expect(shouldUpgrade).To(BeTrue())
			Expect(boshClient.GetEventsCallCount()).To(BeZero())
			Expect(boshClient.GetTaskCallCount()).To(BeZero())
			Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
		})
	})

	Context("when manifest generation fails", func() {
		It("returns true", func() {

			errorMessage := "can't generate manifest"
			manifestGenerator.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{}, errors.New(errorMessage))

			shouldUpgrade := preUpgrade.ShouldUpgrade(
				task.GenerateManifestProperties{
					DeploymentName: deploymentName,
					OldManifest:    []byte(generatedManifest),
				},
				logger)

			Expect(shouldUpgrade).To(BeTrue())
			Expect(logBuffer.String()).To(ContainSubstring(errorMessage))

			Expect(boshClient.GetEventsCallCount()).To(BeZero())
			Expect(boshClient.GetTaskCallCount()).To(BeZero())
			Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
		})
	})

	Context("when the bosh client fails", func() {
		It("should return true when get events fail", func() {
			errorMessage := "failed to retrieve events"
			boshClient.GetEventsReturns([]boshdirector.BoshEvent{}, errors.New(errorMessage))

			shouldUpgrade := preUpgrade.ShouldUpgrade(
				task.GenerateManifestProperties{
					DeploymentName: deploymentName,
					OldManifest:    []byte(generatedManifest),
				},
				logger)

			Expect(shouldUpgrade).To(BeTrue())
			Expect(logBuffer.String()).To(ContainSubstring(errorMessage))
			Expect(boshClient.GetTaskCallCount()).To(BeZero())
			Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
		})

		It("should return true when get events returns no events", func() {
			boshClient.GetEventsReturns([]boshdirector.BoshEvent{}, nil)

			shouldUpgrade := preUpgrade.ShouldUpgrade(
				task.GenerateManifestProperties{
					DeploymentName: deploymentName,
					OldManifest:    []byte(generatedManifest),
				},
				logger)

			Expect(shouldUpgrade).To(BeTrue())
			Expect(boshClient.GetTaskCallCount()).To(BeZero())
			Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
		})

		It("should return true when get tasks returns no tasks", func() {
			boshClient.GetEventsReturns([]boshdirector.BoshEvent{
				{TaskId: "189"},
			}, nil)
			boshClient.GetTaskReturns(boshdirector.BoshTask{}, nil)

			shouldUpgrade := preUpgrade.ShouldUpgrade(
				task.GenerateManifestProperties{
					DeploymentName: deploymentName,
					OldManifest:    []byte(generatedManifest),
				},
				logger)

			Expect(shouldUpgrade).To(BeTrue())
			Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
		})

		It("should return true when get tasks returns task without contextID", func() {
			boshClient.GetEventsReturns([]boshdirector.BoshEvent{
				{TaskId: "189"},
			}, nil)
			boshClient.GetTaskReturns(boshdirector.BoshTask{ContextID: "", State: boshdirector.TaskDone, ID: 3232}, nil)

			shouldUpgrade := preUpgrade.ShouldUpgrade(
				task.GenerateManifestProperties{
					DeploymentName: deploymentName,
					OldManifest:    []byte(generatedManifest),
				},
				logger)

			Expect(shouldUpgrade).To(BeFalse())
			Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
		})

		It("should return true when get tasks returns an error", func() {
			boshClient.GetEventsReturns([]boshdirector.BoshEvent{
				{TaskId: "189"},
			}, nil)
			errorMessage := "get task failed"
			boshClient.GetTaskReturns(boshdirector.BoshTask{}, errors.New(errorMessage))

			shouldUpgrade := preUpgrade.ShouldUpgrade(
				task.GenerateManifestProperties{
					DeploymentName: deploymentName,
					OldManifest:    []byte(generatedManifest),
				},
				logger)

			Expect(shouldUpgrade).To(BeTrue())
			Expect(logBuffer.String()).To(ContainSubstring(errorMessage))
			Expect(boshClient.GetNormalisedTasksByContextCallCount()).To(BeZero())
		})

		It("should return true when get tasks by context id returns an error", func() {
			boshClient.GetEventsReturns([]boshdirector.BoshEvent{
				{TaskId: "189"},
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
				logger)

			Expect(logBuffer.String()).To(ContainSubstring(errorMessage))
			Expect(shouldUpgrade).To(BeTrue())
		})

		It("should return true when get tasks by context id returns no task", func() {
			boshClient.GetEventsReturns([]boshdirector.BoshEvent{
				{TaskId: "189"},
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
				logger)

			Expect(shouldUpgrade).To(BeTrue())
		})
	})

	Context("when the deployment is already updated", func() {
		Context("when all errands have run successfully in the previous run", func() {
			const expectedDeploymentTask = 103
			const expectedContextID = "231"
			BeforeEach(func() {
				boshClient.GetEventsReturns([]boshdirector.BoshEvent{
					{TaskId: fmt.Sprintf("%d", expectedDeploymentTask)},
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

			It("returns OperationAlreadyCompletedError error", func() {
				shouldUpgrade := preUpgrade.ShouldUpgrade(
					task.GenerateManifestProperties{
						DeploymentName: deploymentName,
						OldManifest:    []byte(generatedManifest),
					},
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

		Context("when one errand ", func() {
			const expectedDeploymentTask = "103"
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

			It("has failed the previous run then starts upgrading successfully", func() {
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
					logger)

				Expect(shouldUpgrade).To(BeTrue())
			})

			It("has not completed the previous run then starts upgrading successfully", func() {
				boshClient.GetNormalisedTasksByContextReturns(boshdirector.BoshTasks{
					{State: boshdirector.TaskProcessing, ID: 3234, ContextID: expectedContextID},
					{State: boshdirector.TaskDone, ID: 3233, ContextID: expectedContextID},
					{State: boshdirector.TaskDone, ID: 3232, ContextID: expectedContextID},
				}, nil)

				shouldUpgrade := preUpgrade.ShouldUpgrade(
					task.GenerateManifestProperties{DeploymentName: deploymentName},
					logger)

				Expect(shouldUpgrade).To(BeTrue())
			})
		})
	})
})
