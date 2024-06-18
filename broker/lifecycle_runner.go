// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

type LifeCycleRunner struct {
	boshClient BoshClient
	plans      config.Plans
}

func NewLifeCycleRunner(
	boshClient BoshClient,
	plans config.Plans,
) LifeCycleRunner {
	return LifeCycleRunner{
		boshClient,
		plans,
	}
}

func (l LifeCycleRunner) GetTask(deploymentName string, operationData OperationData, logger *log.Logger,
) (boshdirector.BoshTask, error) {
	switch {
	case operationData.BoshContextID == "":
		return l.boshClient.GetTask(operationData.BoshTaskID, logger)
	case validPostDeployOpType(operationData.OperationType):
		return l.processPostDeployment(deploymentName, operationData, logger)
	case validPreDeleteOpType(operationData.OperationType):
		return l.processPreDelete(deploymentName, operationData, logger)
	default:
		return l.boshClient.GetTask(operationData.BoshTaskID, logger)
	}
}

func validPostDeployOpType(op OperationType) bool {
	return op == OperationTypeCreate ||
		op == OperationTypeUpdate ||
		op == OperationTypeRecreate ||
		op == OperationTypeUpgrade
}

func validPreDeleteOpType(op OperationType) bool {
	return op == OperationTypeDelete || op == OperationTypeForceDelete
}

func (l LifeCycleRunner) processPostDeployment(
	deploymentName string,
	operationData OperationData,
	logger *log.Logger,
) (boshdirector.BoshTask, error) {
	boshTasks, err := l.boshClient.GetNormalisedTasksByContext(deploymentName, operationData.BoshContextID, logger)
	if err != nil {
		return boshdirector.BoshTask{}, err
	}

	if len(boshTasks) == 0 {
		return boshdirector.BoshTask{}, fmt.Errorf("no tasks found for context id: %s", operationData.BoshContextID)
	}

	task := boshTasks[0]

	if task.StateType() != boshdirector.TaskComplete {
		return task, nil
	}

	if isOldStylePostDeployOperationData(boshTasks, operationData) {
		return l.runErrand(deploymentName, config.Errand{Name: operationData.PostDeployErrand.Name, Instances: operationData.PostDeployErrand.Instances}, operationData.BoshContextID, logger)
	}

	nextErrandIndex := len(boshTasks) - 1
	if nextErrandIndex < len(operationData.Errands) {
		return l.runErrand(deploymentName, operationData.Errands[nextErrandIndex], operationData.BoshContextID, logger)
	}

	if len(operationData.Errands) == 0 && operationData.PostDeployErrand.Name == "" {
		logger.Println("can't determine lifecycle errands, neither PlanID nor PostDeployErrand.Name is present")
	}
	return task, nil
}

func (l LifeCycleRunner) processPreDelete(
	deploymentName string,
	operationData OperationData,
	logger *log.Logger,
) (boshdirector.BoshTask, error) {
	boshTasks, err := l.getTasks(deploymentName, operationData, logger)
	if err != nil {
		return boshdirector.BoshTask{}, err
	}

	currentTask := boshTasks[0]
	if taskIsNotDone(currentTask, operationData.OperationType, logger) {
		return currentTask, nil
	}

	if shouldRunDeprovision(boshTasks, operationData) {
		return l.deprovisionAfterAllErrand(deploymentName, boshTasks, logger, operationData)
	}

	if hasRunAllErrands(boshTasks, operationData) {
		return currentTask, nil
	}

	errand := operationData.Errands[len(boshTasks)]
	return l.runErrand(deploymentName, errand, operationData.BoshContextID, logger)
}

func shouldRunDeprovision(boshTasks []boshdirector.BoshTask, operationData OperationData) bool {
	return len(boshTasks) == len(operationData.Errands) || isOldStylePreDeleteOperationData(boshTasks, operationData)
}

func (l LifeCycleRunner) deprovisionAfterAllErrand(deploymentName string, boshTasks []boshdirector.BoshTask, logger *log.Logger, operationData OperationData) (boshdirector.BoshTask, error) {
	taskID, err := l.boshClient.DeleteDeployment(
		deploymentName,
		operationData.BoshContextID,
		shouldForceDelete(operationData),
		boshdirector.NewAsyncTaskReporter(),
		logger)
	if err != nil {
		return boshdirector.BoshTask{}, err
	}
	return l.boshClient.GetTask(taskID, logger)
}

func (l LifeCycleRunner) getTasks(deploymentName string, operationData OperationData, logger *log.Logger) ([]boshdirector.BoshTask, error) {
	boshTasks, err := l.boshClient.GetNormalisedTasksByContext(deploymentName, operationData.BoshContextID, logger)
	if err != nil {
		return []boshdirector.BoshTask{}, err
	}

	if len(boshTasks) == 0 {
		return []boshdirector.BoshTask{}, fmt.Errorf("no tasks found for context id: %s", operationData.BoshContextID)
	}

	return boshTasks, nil
}

func taskIsNotDone(task boshdirector.BoshTask, operationType OperationType, logger *log.Logger) bool {
	currentTaskState := task.StateType()
	return !shouldSkipError(currentTaskState, operationType, logger) && isNotCompleted(currentTaskState)
}

func isNotCompleted(taskState boshdirector.TaskStateType) bool {
	return taskState != boshdirector.TaskComplete
}

func shouldSkipError(taskState boshdirector.TaskStateType, operationType OperationType, logger *log.Logger) bool {
	shouldSkipError := taskState == boshdirector.TaskFailed && operationType == OperationTypeForceDelete

	if shouldSkipError {
		logger.Printf("pre-delete errand failed during %q, continuing to next operation", operationType)
	}

	return shouldSkipError
}

func hasRunAllErrands(boshTasks boshdirector.BoshTasks, operationData OperationData) bool {
	return len(boshTasks) > len(operationData.Errands)
}

func shouldForceDelete(operationData OperationData) bool {
	forceDelete := false
	if operationData.OperationType == OperationTypeForceDelete {
		forceDelete = true
	}
	return forceDelete
}

func isOldStylePreDeleteOperationData(boshTasks boshdirector.BoshTasks, operationData OperationData) bool {
	return len(boshTasks) == 1 && operationData.PreDeleteErrand.Name != ""
}

func isOldStylePostDeployOperationData(boshTasks boshdirector.BoshTasks, operationData OperationData) bool {
	return len(boshTasks) == 1 && operationData.PostDeployErrand.Name != ""
}

func (l LifeCycleRunner) runErrand(deploymentName string, errand config.Errand, contextID string, log *log.Logger) (boshdirector.BoshTask, error) {
	taskID, err := l.boshClient.RunErrand(deploymentName, errand.Name, errand.Instances, contextID, log, boshdirector.NewAsyncTaskReporter())
	if err != nil {
		return boshdirector.BoshTask{}, err
	}

	task, err := l.boshClient.GetTask(taskID, log)
	if err != nil {
		return boshdirector.BoshTask{}, err
	}

	return task, nil
}
