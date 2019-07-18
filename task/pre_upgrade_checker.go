package task

import (
	"bytes"
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/config"
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

func (p PreUpgrade) ShouldUpgrade(generateManifestProp GenerateManifestProperties, plan config.Plan, logger *log.Logger) bool {
	generateManifestOutput, err := p.manifestGenerator.GenerateManifest(
		generateManifestProp,
		logger,
	)
	if err != nil {
		logger.Print(err.Error())
		return ShouldUpgrade
	}

	if !manifestAreTheSame(generateManifestOutput, generateManifestProp.OldManifest) {
		return ShouldUpgrade
	}

	if p.hasNoPostDeployErrands(plan) {
		logger.Printf("manifest is unchanged and there are no post-deploy errand for %q, skipping upgrade", generateManifestProp.DeploymentName)
		return !ShouldUpgrade
	}

	events, err := p.getUpdateDeploymentEvents(generateManifestProp.DeploymentName, logger)
	if err != nil {
		logger.Printf("failed to get update deployment events for deployment %q with cause %q", generateManifestProp.DeploymentName, err.Error())
		return ShouldUpgrade
	}

	if p.hasNoPreviousUpgrade(events) {
		events, err = p.getCreateDeploymentEvents(generateManifestProp.DeploymentName, logger)
		if err != nil {
			logger.Printf("failed to get create deployment events for deployment %q with cause %q", generateManifestProp.DeploymentName, err.Error())
			return ShouldUpgrade
		}

		if len(events) == 0 {
			return ShouldUpgrade
		}
	}

	mostRecentBOSHUpdateEvent := events[0]
	taskID := mostRecentBOSHUpdateEvent.TaskId

	task, err := p.boshClient.GetTask(taskID, logger)
	if err != nil {
		logger.Printf("failed to get task for id %d with cause %q for deployment %q", taskID, err.Error(), generateManifestProp.DeploymentName)
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
		logger.Printf("no tasks for contextID %q, upgrading deployment %q", task.ContextID, generateManifestProp.DeploymentName)
		return ShouldUpgrade
	}

	if p.allErrandsHaveRun(plan, tasksForContextId) && tasksForContextId.AllTasksAreDone() {
		logger.Printf("manifest is unchanged and all post-deploy errands were run successfuly. Skipping upgrade for deployment %q", generateManifestProp.DeploymentName)
		return !ShouldUpgrade
	}

	return ShouldUpgrade
}

func (p PreUpgrade) hasNoPostDeployErrands(plan config.Plan) bool {
	errands := plan.LifecycleErrands
	if errands == nil {
		return true
	}
	return len(errands.PostDeploy) == 0
}

func (p PreUpgrade) hasNoPreviousUpgrade(events []boshdirector.BoshEvent) bool {
	return len(events) == 0
}

func (p PreUpgrade) getUpdateDeploymentEvents(deploymentName string, logger *log.Logger) ([]boshdirector.BoshEvent, error) {
	events, err := p.boshClient.GetEvents(deploymentName, "update", logger)
	return events, err
}

func (p PreUpgrade) getCreateDeploymentEvents(deploymentName string, logger *log.Logger) ([]boshdirector.BoshEvent, error) {
	events, err := p.boshClient.GetEvents(deploymentName, "create", logger)
	return events, err
}

func (p PreUpgrade) allErrandsHaveRun(plan config.Plan, tasks boshdirector.BoshTasks) bool {
	numTasksForDeploy := 1
	numPostDeployErrands := len(plan.LifecycleErrands.PostDeploy)
	numTasksBoshRan := len(tasks)

	return numPostDeployErrands+numTasksForDeploy == numTasksBoshRan
}

func manifestAreTheSame(generateManifestOutput serviceadapter.MarshalledGenerateManifest, oldManifest []byte) bool {
	return bytes.Compare([]byte(generateManifestOutput.Manifest), oldManifest) == 0
}
