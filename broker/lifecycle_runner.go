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
		op == OperationTypeUpgrade
}

func validPreDeleteOpType(op OperationType) bool {
	return op == OperationTypeDelete
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
		return l.runErrand(deploymentName, operationData.PostDeployErrand.Name, operationData.PostDeployErrand.Instances, operationData.BoshContextID, logger)
	}

	nextErrandIndex := len(boshTasks) - 1
	if nextErrandIndex < len(operationData.Errands) {
		errand := operationData.Errands[nextErrandIndex].Name
		instances := operationData.Errands[nextErrandIndex].Instances
		return l.runErrand(deploymentName, errand, instances, operationData.BoshContextID, logger)
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

	if len(boshTasks) == len(operationData.Errands) || isOldStylePreDeleteOperationData(boshTasks, operationData) {
		taskID, err := l.boshClient.DeleteDeployment(deploymentName, operationData.BoshContextID, logger, boshdirector.NewAsyncTaskReporter())
		if err != nil {
			return boshdirector.BoshTask{}, err
		}
		return l.boshClient.GetTask(taskID, logger)
	}

	if len(boshTasks) > len(operationData.Errands) {
		return task, nil
	}

	errand := operationData.Errands[len(boshTasks)]
	return l.runErrand(deploymentName, errand.Name, errand.Instances, operationData.BoshContextID, logger)
}

func isOldStylePreDeleteOperationData(boshTasks boshdirector.BoshTasks, operationData OperationData) bool {
	return len(boshTasks) == 1 && operationData.PreDeleteErrand.Name != ""
}

func isOldStylePostDeployOperationData(boshTasks boshdirector.BoshTasks, operationData OperationData) bool {
	return len(boshTasks) == 1 && operationData.PostDeployErrand.Name != ""
}

func (l LifeCycleRunner) runErrand(deploymentName, errand string, errandInstances []string, contextID string, log *log.Logger) (boshdirector.BoshTask, error) {
	taskID, err := l.boshClient.RunErrand(deploymentName, errand, errandInstances, contextID, log, boshdirector.NewAsyncTaskReporter())
	if err != nil {
		return boshdirector.BoshTask{}, err
	}

	task, err := l.boshClient.GetTask(taskID, log)
	if err != nil {
		return boshdirector.BoshTask{}, err
	}

	return task, nil
}
