// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"context"
	"errors"
	"fmt"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/adapterclient"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
)

const (
	GenericErrorPrefix              = "There was a problem completing your request. Please contact your operations team providing the following information:"
	PendingChangesErrorMessage      = `There is a pending change to your service instance, you must first run cf update-service <service_name> -c '{"apply-changes": true}', no other arbitrary parameters are allowed`
	ApplyChangesDisabledMessage     = "Service cannot be updated at this time, please try again later or contact your operator for more information"
	ApplyChangesNotPermittedMessage = `'apply-changes' is not permitted. Contact your operator for more information`
	OperationInProgressMessage      = "An operation is in progress for your service instance. Please try again later."
)

type InstanceNotFoundError struct {
	error
}

func NewInstanceNotFoundError() InstanceNotFoundError {
	return InstanceNotFoundError{error: errors.New("service instance not found")}
}

type DeploymentNotFoundError struct {
	error
}

func NewDeploymentNotFoundError(e error) error {
	return DeploymentNotFoundError{e}
}

type OperationInProgressError struct {
	error
}

func applyChangesNotABooleanError(value interface{}) error {
	return fmt.Errorf("apply-changes value '%v' is not true or false", value)
}

func NewOperationInProgressError(e error) error {
	return OperationInProgressError{e}
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
	taskErrorType TaskErrorType
}

func NewTaskError(e error, taskErrorType TaskErrorType) error {
	return TaskError{
		error:         e,
		taskErrorType: taskErrorType,
	}
}

type TaskInProgressError struct {
	Message string
}

func (e TaskInProgressError) Error() string {
	return e.Message
}

type PlanNotFoundError struct {
	PlanGUID string
}

func (e PlanNotFoundError) Error() string {
	return fmt.Sprintf("plan %s does not exist", e.PlanGUID)
}

type ServiceError struct {
	error
}

func NewServiceError(e error) error {
	return ServiceError{error: e}
}

var NilError = DisplayableError{nil, nil}

// TODO SF Remove by logging operator messages when raising the error?
type DisplayableError struct {
	errorForCFUser   error
	errorForOperator error
}

func (e DisplayableError) ErrorForCFUser() error {
	return e.errorForCFUser
}

func (e DisplayableError) Error() string {
	return fmt.Sprintf("error: %s. error for user: %s.", e.errorForOperator, e.errorForCFUser)
}

func (e DisplayableError) Occurred() bool {
	return e.errorForCFUser != nil && e.errorForOperator != nil
}

func NewDisplayableError(errorForCFUser, errForOperator error) DisplayableError {
	return DisplayableError{
		errorForCFUser,
		errForOperator,
	}
}

func NewBoshRequestError(action string, requestError error) DisplayableError {
	return DisplayableError{
		fmt.Errorf("Currently unable to %s service instance, please try again later", action),
		requestError,
	}
}

func NewPendingChangesError(errForOperator error) DisplayableError {
	return DisplayableError{
		errors.New(PendingChangesErrorMessage),
		errForOperator,
	}
}

func NewApplyChangesDisabledError(errForOperator error) DisplayableError {
	return DisplayableError{
		errors.New(ApplyChangesDisabledMessage),
		errForOperator,
	}
}

func NewApplyChangesNotPermittedError(errForOperator error) DisplayableError {
	return DisplayableError{
		errors.New(ApplyChangesNotPermittedMessage),
		errForOperator,
	}
}

func NewGenericError(ctx context.Context, err error) DisplayableError {
	serviceName := brokercontext.GetServiceName(ctx)
	instanceID := brokercontext.GetInstanceID(ctx)
	reqID := brokercontext.GetReqID(ctx)
	operation := brokercontext.GetOperation(ctx)
	boshTaskID := brokercontext.GetBoshTaskID(ctx)

	message := fmt.Sprintf(
		"%s service: %s, service-instance-guid: %s, broker-request-id: %s",
		GenericErrorPrefix,
		serviceName,
		instanceID,
		reqID,
	)

	if boshTaskID != 0 {
		message += fmt.Sprintf(", task-id: %d", boshTaskID)
	}

	if operation != "" {
		message += fmt.Sprintf(", operation: %s", operation)
	}

	return DisplayableError{
		errorForCFUser:   errors.New(message),
		errorForOperator: err,
	}
}

// TODO test this logic
func adapterToAPIError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	switch err.(type) {
	case adapterclient.BindingAlreadyExistsError:
		return brokerapi.ErrBindingAlreadyExists
	case adapterclient.BindingNotFoundError:
		return brokerapi.ErrBindingDoesNotExist
	case adapterclient.AppGuidNotProvidedError:
		return brokerapi.ErrAppGuidNotProvided
	case adapterclient.UnknownFailureError:
		if err.Error() == "" {
			//Adapter returns an unknown error with no message
			err = NewGenericError(ctx, err).ErrorForCFUser()
		}

		return err
	default:
		return NewGenericError(ctx, err).ErrorForCFUser()
	}
}
