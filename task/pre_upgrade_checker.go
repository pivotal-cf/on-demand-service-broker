package task

import (
	"bytes"
	"log"

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
		errandEvents, err := p.boshClient.GetErrandEvents(generateManifestProp.DeploymentName, logger)
		if err != nil {
			logger.Printf("failed to get errand events for deployment %q with cause %q", generateManifestProp.DeploymentName, err.Error())
			return ShouldUpgrade
		}
		if p.noPostDeployErrands(errandEvents) {
			logger.Printf("manifest is unchanged and there are no post-deploy errand for %q, skipping upgrade", generateManifestProp.DeploymentName)
			return !ShouldUpgrade
		}

		events, err := p.getUpdateDeploymentEvents(generateManifestProp.DeploymentName, logger)
		if err != nil {
			logger.Printf("failed to get update deployment events for deployment %q with cause %q", generateManifestProp.DeploymentName, err.Error())
			return ShouldUpgrade
		}
		if p.noPreviousUpgrade(events) {
			return ShouldUpgrade
		}

		taskID := events[0].TaskId

		task, err := p.boshClient.GetTask(taskID, logger)
		if err != nil {
			logger.Printf("failed to get task for id %d with cause %q for deployment %q", taskID, err.Error(), generateManifestProp.DeploymentName)
			return ShouldUpgrade
		}
		if (boshdirector.BoshTask{}) == task {
			logger.Printf("no task found for taskID %q for deployment %q", taskID, generateManifestProp.DeploymentName)
			return ShouldUpgrade
		}
		if task.ContextID == "" {
			logger.Printf("failed to get contextID for deployment %q", generateManifestProp.DeploymentName)
			return ShouldUpgrade
		}

		tasksForContextId, err := p.boshClient.GetNormalisedTasksByContext(generateManifestProp.DeploymentName, task.ContextID, logger)
		if err != nil {
			logger.Printf("failed to get task by context id %q with cause %q for deployment %q", task.ContextID, err.Error(), generateManifestProp.DeploymentName)
			return ShouldUpgrade
		}
		if len(tasksForContextId) == 0 {
			logger.Printf("no task for contexId %q, upgrading deployment %q", task.ContextID, generateManifestProp.DeploymentName)
			return ShouldUpgrade
		}

		if tasksForContextId.AreAllTaskDone() {
			logger.Printf("manifest is unchanged and all post-deploy errand were run successfuly for, skipping upgrade for deployment %q", generateManifestProp.DeploymentName)
			return !ShouldUpgrade
		}
	}

	return ShouldUpgrade
}

func (p PreUpgrade) noPostDeployErrands(errandEvents []boshdirector.BoshEvent) bool {
	return len(errandEvents) == 0
}

func (p PreUpgrade) noPreviousUpgrade(events []boshdirector.BoshEvent) bool {
	return len(events) == 0
}

func (p PreUpgrade) getUpdateDeploymentEvents(deploymentName string, logger *log.Logger) ([]boshdirector.BoshEvent, error) {
	events, err := p.boshClient.GetUpdatesEvents(deploymentName, logger)
	return events, err
}

func manifestAreTheSame(generateManifestOutput serviceadapter.MarshalledGenerateManifest, oldManifest []byte) bool {
	return bytes.Compare([]byte(generateManifestOutput.Manifest), oldManifest) == 0
}
