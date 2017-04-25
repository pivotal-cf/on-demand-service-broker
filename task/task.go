// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package task

import (
	"errors"
	"fmt"
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/boshclient"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
)

//go:generate counterfeiter -o fakes/fake_bosh_client.go . BoshClient
type BoshClient interface {
	Deploy(manifest []byte, contextID string, logger *log.Logger) (int, error)
	GetTasks(deploymentName string, logger *log.Logger) (boshclient.BoshTasks, error)
	GetDeployment(name string, logger *log.Logger) ([]byte, bool, error)
}

// TODO SF Why is  previousPlanID a pointer to a string?
//go:generate counterfeiter -o fakes/fake_manifest_generator.go . ManifestGenerator
type ManifestGenerator interface {
	GenerateManifest(deploymentName, planID string, requestParams map[string]interface{}, oldManifest []byte, previousPlanID *string, logger *log.Logger) (BoshManifest, error)
}

//go:generate counterfeiter -o fakes/fake_feature_flags.go . FeatureFlags
type FeatureFlags interface {
	CFUserTriggeredUpgrades() bool
}

type deployer struct {
	boshClient        BoshClient
	manifestGenerator ManifestGenerator
	featureFlags      FeatureFlags
}

func NewDeployer(
	boshClient BoshClient,
	manifestGenerator ManifestGenerator,
	featureFlags FeatureFlags,
) deployer {
	return deployer{
		boshClient:        boshClient,
		manifestGenerator: manifestGenerator,
		featureFlags:      featureFlags,
	}
}

func (d deployer) Create(
	deploymentName,
	planID string,
	requestParams map[string]interface{},
	boshContextID string,
	logger *log.Logger,
) (int, []byte, error) {

	err := d.assertNoOperationsInProgress(deploymentName, logger)
	if err != nil {
		return 0, nil, err
	}

	return d.doDeploy(
		deploymentName,
		planID,
		"create",
		requestParams,
		nil,
		nil,
		boshContextID,
		logger,
	)
}

func (d deployer) Upgrade(
	deploymentName,
	planID string,
	previousPlanID *string,
	boshContextID string,
	logger *log.Logger,
) (int, []byte, error) {

	oldManifest, err := d.getDeploymentManifest(deploymentName, logger)
	if err != nil {
		return 0, nil, err
	}

	err = d.assertNoOperationsInProgressForUpgrade(deploymentName, logger)
	if err != nil {
		return 0, nil, err
	}

	return d.doDeploy(
		deploymentName,
		planID,
		"upgrade",
		nil,
		oldManifest,
		previousPlanID,
		boshContextID,
		logger,
	)
}

func (d deployer) Update(
	deploymentName,
	planID string,
	requestParams map[string]interface{},
	previousPlanID *string,
	boshContextID string,
	logger *log.Logger,
) (int, []byte, error) {

	oldManifest, err := d.getDeploymentManifest(deploymentName, logger)
	if err != nil {
		return 0, nil, err
	}

	if err := d.assertNoOperationsInProgress(deploymentName, logger); err != nil {
		return 0, nil, err
	}

	parameters := parametersFromRequest(requestParams)
	applyingChanges, err := d.validatedApplyChanges(parameters)
	if err != nil {
		return 0, nil, err
	}

	if applyingChanges {
		if err := d.assertCanApplyChanges(parameters, planID, previousPlanID); err != nil {
			return 0, nil, err
		}
	}

	if err := d.checkForPendingChanges(applyingChanges, deploymentName, previousPlanID, oldManifest, logger); err != nil {
		return 0, nil, err
	}

	return d.doDeploy(
		deploymentName,
		planID,
		"update",
		requestParams,
		oldManifest,
		previousPlanID,
		boshContextID,
		logger,
	)
}

func (d deployer) getDeploymentManifest(deploymentName string, logger *log.Logger) ([]byte, error) {
	oldManifest, found, err := d.boshClient.GetDeployment(deploymentName, logger)

	if err != nil {
		return nil, err
	}

	if !found {
		return nil, broker.NewDeploymentNotFoundError(fmt.Errorf("bosh deployment '%s' not found", deploymentName))
	}

	return oldManifest, nil
}

// TODO SF Why are these two methods different?
func (d deployer) assertNoOperationsInProgress(deploymentName string, logger *log.Logger) error {
	clientTasks, err := d.boshClient.GetTasks(deploymentName, logger)
	if err != nil {
		return fmt.Errorf("error getting tasks for deployment %s: %s\n", deploymentName, err)
	}

	if incompleteTasks := clientTasks.IncompleteTasks(); len(incompleteTasks) != 0 {
		userError := errors.New("An operation is in progress for your service instance. Please try again later.")
		operatorError := broker.NewOperationInProgressError(
			fmt.Errorf("deployment %s is still in progress: tasks %s\n",
				deploymentName,
				incompleteTasks.ToLog()),
		)
		return broker.NewDisplayableError(userError, operatorError)
	}

	return nil
}

func (d deployer) assertNoOperationsInProgressForUpgrade(deploymentName string, logger *log.Logger) error {
	clientTasks, err := d.boshClient.GetTasks(deploymentName, logger)
	if err != nil {
		return fmt.Errorf("error getting tasks for deployment %s: %s\n", deploymentName, err)
	}

	if incompleteTasks := clientTasks.IncompleteTasks(); len(incompleteTasks) != 0 {
		logger.Printf("deployment %s is still in progress: tasks %s\n", deploymentName, incompleteTasks.ToLog())
		return broker.NewOperationInProgressError(errors.New("An operation is in progress for your service instance. Please try again later."))
	}

	return nil
}

func parametersFromRequest(requestParams map[string]interface{}) map[string]interface{} {
	parameters, ok := requestParams["parameters"].(map[string]interface{})
	if !ok {
		return nil
	}

	return parameters
}

func (d deployer) validatedApplyChanges(parameters map[string]interface{}) (bool, error) {
    const applyChangesKey = "apply-changes"

    value := parameters[applyChangesKey]
	if value == nil {
		return false, nil
	}

	applyChanges, ok := value.(bool)
	if !ok {
		return false, broker.NewTaskError(errors.New("update called with apply-changes set to non-boolean"))
	}

	delete(parameters, applyChangesKey)

	return applyChanges, nil
}

func (d deployer) assertCanApplyChanges(parameters map[string]interface{}, planID string, previousPlanID *string) error {
	if !d.featureFlags.CFUserTriggeredUpgrades() {
		return broker.NewApplyChangesNotPermittedError(errors.New("'cf_user_triggered_upgrades' feature is disabled"))
	}

	if previousPlanID != nil && planID != *previousPlanID {
		return broker.NewTaskError(errors.New("update called with apply-changes and a plan change"))
	}

	if len(parameters) > 0 {
		return broker.NewTaskError(errors.New("update called with apply-changes and arbitrary parameters set"))
	}

	return nil
}

func (d deployer) checkForPendingChanges(
	applyingChanges bool,
	deploymentName string,
	previousPlanID *string,
	oldManifest BoshManifest,
	logger *log.Logger,
) error {
	regeneratedManifest, err := d.manifestGenerator.GenerateManifest(deploymentName, *previousPlanID, map[string]interface{}{}, oldManifest, previousPlanID, logger)
	if err != nil {
		return err
	}

	manifestsUnchanged, err := regeneratedManifest.Equals(oldManifest)
	if err != nil {
		return fmt.Errorf("error detecting change in manifest: %s", err)
	}

	if !manifestsUnchanged && !applyingChanges {
		return broker.NewTaskError(errors.New("pending changes detected"))
	}

	return nil
}

func (d deployer) doDeploy(
	deploymentName,
	planID string,
	operationType string,
	requestParams map[string]interface{},
	oldManifest []byte,
	previousPlanID *string,
	boshContextID string,
	logger *log.Logger,
) (int, []byte, error) {
	manifest, err := d.manifestGenerator.GenerateManifest(
		deploymentName,
		planID,
		requestParams,
		oldManifest,
		previousPlanID,
		logger,
	)
	if err != nil {
		return 0, nil, err
	}

	boshTaskID, err := d.boshClient.Deploy(manifest, boshContextID, logger)
	if err != nil {
		return 0, nil, fmt.Errorf("error deploying instance: %s\n", err)
	}
	logger.Printf("Bosh task ID for %s deployment %s is %d\n", operationType, deploymentName, boshTaskID)

	return boshTaskID, manifest, nil
}
