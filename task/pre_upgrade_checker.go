package task

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

type PreUpgrade struct {
	manifestGenerator       ManifestGenerator
	boshClient              BoshClient
	enableOptimisedUpgrades bool
}

const (
	ShouldUpgrade = true
)

func NewPreUpgrade(generator ManifestGenerator, client BoshClient, enableOptimisedUpgrades bool) PreUpgrade {
	return PreUpgrade{
		manifestGenerator:       generator,
		boshClient:              client,
		enableOptimisedUpgrades: enableOptimisedUpgrades,
	}
}

func (p PreUpgrade) ShouldUpgrade(generateManifestProp GenerateManifestProperties, plan config.Plan, logger *log.Logger) bool {
	if !p.enableOptimisedUpgrades {
		return ShouldUpgrade
	}

	upgradeLogger := LoggerWithContext{
		logger:  logger,
		context: fmt.Sprintf("[ShouldUpgrade] Upgrading deployment %q", generateManifestProp.DeploymentName),
	}

	generateManifestOutput, err := p.manifestGenerator.GenerateManifest(
		generateManifestProp,
		logger,
	)

	if err != nil {
		upgradeLogger.log("Failed to get manifest from adapter with cause %q", err.Error())
		return ShouldUpgrade
	}

	if !manifestAreTheSame([]byte(generateManifestOutput.Manifest), generateManifestProp.OldManifest) {
		upgradeLogger.log("Manifest has changed")
		return ShouldUpgrade
	}

	if p.hasNoPostDeployErrands(plan) {
		upgradeLogger.log("Manifest is unchanged and there are no post-deploy errand. Skipping upgrade")
		return !ShouldUpgrade
	}

	events, err := p.getUpdateDeploymentEvents(generateManifestProp.DeploymentName, logger)
	if err != nil {
		upgradeLogger.log("Failed to get update deployment events with cause %q", err.Error())
		return ShouldUpgrade
	}

	if p.hasNoPreviousUpgrade(events) {
		events, err = p.getCreateDeploymentEvents(generateManifestProp.DeploymentName, logger)
		if err != nil {
			upgradeLogger.log("Failed to get create deployment events with cause %q", err.Error())
			return ShouldUpgrade
		}

		if len(events) == 0 {
			upgradeLogger.log("No create deployment events found")
			return ShouldUpgrade
		}
	}

	mostRecentBOSHUpdateEvent := events[0]
	taskID := mostRecentBOSHUpdateEvent.TaskId

	task, err := p.boshClient.GetTask(taskID, logger)
	if err != nil {
		upgradeLogger.log("Failed to get task for id %d with cause %q", taskID, err.Error())
		return ShouldUpgrade
	}

	if task.ContextID == "" {
		upgradeLogger.log("Failed to get contextID")
		return ShouldUpgrade
	}

	tasksForContextId, err := p.boshClient.GetNormalisedTasksByContext(generateManifestProp.DeploymentName, task.ContextID, logger)
	if err != nil {
		upgradeLogger.log("Failed to get task by context id %q with cause %q", task.ContextID, err.Error())
		return ShouldUpgrade
	}

	if len(tasksForContextId) == 0 {
		upgradeLogger.log("No tasks for contextID %q", task.ContextID)
		return ShouldUpgrade
	}

	if p.allErrandsHaveRun(plan, tasksForContextId) && tasksForContextId.AllTasksAreDone() {
		upgradeLogger.log("Manifest is unchanged and all post-deploy errands were run successfully. Skipping upgrade")
		return !ShouldUpgrade
	}

	upgradeLogger.log("No apparent reasons to skip upgrade.")
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

func manifestAreTheSame(generateManifest, oldManifest []byte) bool {
	areTheSame, _ := ManifestsAreTheSame(generateManifest, oldManifest)

	return areTheSame
}

type LoggerWithContext struct {
	logger  *log.Logger
	context string
}

func (l LoggerWithContext) log(format string, values ...interface{}) {
	l.logger.Printf(l.context+" "+format, values...)
}
