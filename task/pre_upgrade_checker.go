package task

import (
	"bytes"
	"log"
	"strconv"

	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

type PreUpgrade struct {
	manifestGenerator ManifestGenerator
	boshClient        BoshClient
}

const (
	ShouldUpgrade = true
)

func NewPreUpgrade(generator ManifestGenerator, client BoshClient) PreUpgrade {
	return PreUpgrade{manifestGenerator: generator, boshClient: client}
}

func (p PreUpgrade) ShouldUpgrade(generateManifestProp GenerateManifestProperties, logger *log.Logger) bool {
	generateManifestOutput, err := p.manifestGenerator.GenerateManifest(
		generateManifestProp,
		logger,
	)
	if err != nil {
		logger.Print(err.Error())
		return ShouldUpgrade
	}

	if manifestAreTheSame(generateManifestOutput, generateManifestProp.OldManifest) {

		events, err := p.getUpdateDeploymentEvents(generateManifestProp.DeploymentName, logger)
		if err != nil {
			logger.Printf("failed to get update deployment events for deploymentName %q with cause %q", generateManifestProp.DeploymentName, err.Error())
			return ShouldUpgrade
		}
		if p.noPreviousUpgrade(events) {
			return ShouldUpgrade
		}

		taskID := events[0].TaskId
		taskIDint, err := strconv.Atoi(taskID)

		task, err := p.boshClient.GetTask(taskIDint, logger)
		if err != nil {
			logger.Printf("failed to get task for id %d with cause %q", taskIDint, err.Error())
			return ShouldUpgrade
		}
		if (boshdirector.BoshTask{}) == task {
			logger.Printf("no task found for taskID /%q", taskIDint)
			return ShouldUpgrade
		}
		if p.noPostDeployErrands(task) {
			logger.Printf("manifest is unchanged and there are no post-deploy errand for %q, skipping upgrade", generateManifestProp.DeploymentName)
			return !ShouldUpgrade
		}

		tasksForContextId, err := p.boshClient.GetNormalisedTasksByContext(generateManifestProp.DeploymentName, task.ContextID, logger)
		if err != nil {
			logger.Printf("failed to get task by context id %q with cause %q", task.ContextID, err.Error())
			return ShouldUpgrade
		}
		if len(tasksForContextId) == 0 {
			logger.Printf("no task for contexId %q, upgrading deployment %q", task.ContextID, generateManifestProp.DeploymentName)
			return ShouldUpgrade
		}

		logger.Printf("no task for contexId %q, upgrading deployment %q", task.ContextID, generateManifestProp.DeploymentName)
		return !tasksForContextId.AreAllTaskDone()
	}

	logger.Printf("manifest is unchanged and all post-deploy errand were run successfuly for, skipping upgrade")
	return ShouldUpgrade
}

func (p PreUpgrade) noPostDeployErrands(task boshdirector.BoshTask) bool {
	return task.ContextID == ""
}

func (p PreUpgrade) noPreviousUpgrade(events []boshdirector.BoshEvent) bool {
	return len(events) == 0
}

func (p PreUpgrade) getUpdateDeploymentEvents(deploymentName string, logger *log.Logger) ([]boshdirector.BoshEvent, error) {
	events, err := p.boshClient.GetEvents(director.EventsFilter{Deployment: deploymentName, Action: "update", ObjectType: "deployment"}, logger)
	return events, err
}

func manifestAreTheSame(generateManifestOutput serviceadapter.MarshalledGenerateManifest, oldManifest []byte) bool {
	return bytes.Compare([]byte(generateManifestOutput.Manifest), oldManifest) == 0
}
