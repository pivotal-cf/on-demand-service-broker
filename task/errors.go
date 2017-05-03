// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package task

import "fmt"

type PlanNotFoundError struct {
	PlanGUID string
}

func (e PlanNotFoundError) Error() string {
	return fmt.Sprintf("plan %s does not exist", e.PlanGUID)
}

type DeploymentNotFoundError struct {
	error
}

func NewDeploymentNotFoundError(e error) error {
	return DeploymentNotFoundError{e}
}

type TaskInProgressError struct {
	Message string
}

func (e TaskInProgressError) Error() string {
	return e.Message
}

type ServiceError struct {
	error
}

func NewServiceError(e error) error {
	return ServiceError{error: e}
}

type TaskErrorType int // horrible interim solution until we can get the logic in the right place

const (
	ApplyChangesInvalid TaskErrorType = iota
	ApplyChangesWithPlanChange
	ApplyChangesWithParams
	ApplyChangesWithPendingChanges
)

type TaskError struct {
	error
	TaskErrorType TaskErrorType
}

func NewTaskError(e error, taskErrorType TaskErrorType) error {
	return TaskError{
		error:         e,
		TaskErrorType: taskErrorType,
	}
}
