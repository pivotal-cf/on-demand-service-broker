// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi/v8/domain"
	"github.com/pivotal-cf/brokerapi/v8/domain/apiresponses"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

func (b *Broker) Provision(
	ctx context.Context,
	instanceID string,
	details domain.ProvisionDetails,
	asyncAllowed bool) (domain.ProvisionedServiceSpec, error) {

	b.deploymentLock.Lock()
	defer b.deploymentLock.Unlock()

	requestID := uuid.New()
	ctx = brokercontext.New(ctx, string(OperationTypeCreate), requestID, b.serviceOffering.Name, instanceID)
	logger := b.loggerFactory.NewWithContext(ctx)

	if !asyncAllowed {
		return domain.ProvisionedServiceSpec{}, b.processError(apiresponses.ErrAsyncRequired, logger)
	}

	detailsWithRawParameters := domain.DetailsWithRawParameters(details)
	requestParams, err := convertDetailsToMap(detailsWithRawParameters)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, b.processError(err, logger)
	}

	serviceCatalog, err := b.Services(ctx)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, b.processError(err, logger)
	}

	if err := b.decider.CanProvision(serviceCatalog, details.PlanID, details.MaintenanceInfo, logger); err != nil {
		return domain.ProvisionedServiceSpec{}, b.processError(err, logger)
	}

	_, err = b.boshClient.GetInfo(logger)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, b.processError(NewBoshRequestError("create", err), logger)
	}

	requestContext := requestParams["context"]
	var instanceName string
	if requestContext != nil {
		instanceName = getInstanceNameFromContext(requestContext.(map[string]interface{}))
	}

	operationData, dashboardURL, err := b.provisionInstance(
		ctx,
		instanceID,
		details,
		requestParams,
		instanceName,
		logger,
	)

	if err != nil {
		return domain.ProvisionedServiceSpec{}, b.processError(err, logger)
	}

	operationDataJSON, err := json.Marshal(operationData)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, b.processError(err, logger)
	}

	return domain.ProvisionedServiceSpec{
		IsAsync:       true,
		DashboardURL:  dashboardURL,
		OperationData: string(operationDataJSON),
	}, nil
}

func (b *Broker) provisionInstance(ctx context.Context, instanceID string, details domain.ProvisionDetails, requestParams map[string]interface{}, instanceName string, logger *log.Logger) (OperationData, string, error) {
	planID := details.PlanID
	errs := func(err error) (OperationData, string, error) {
		return OperationData{}, "", err
	}

	plan, found := b.serviceOffering.FindPlanByID(planID)
	if !found {
		return errs(NewDisplayableError(
			fmt.Errorf("plan %s not found", planID),
			fmt.Errorf("finding plan ID %s", planID),
		))
	}

	_, found, err := b.boshClient.GetDeployment(deploymentName(instanceID), logger)
	switch err := err.(type) {
	case boshdirector.RequestError:
		return errs(NewBoshRequestError("create", fmt.Errorf("could not get manifest: %s", err)))
	case error:
		return errs(NewGenericError(ctx, fmt.Errorf("could not get manifest: %s", err)))
	}

	if found {
		return errs(NewDisplayableError(
			apiresponses.ErrInstanceAlreadyExists,
			fmt.Errorf("deploying instance %s", instanceID),
		))
	}

	cfPlanCounts, err := b.cfClient.CountInstancesOfServiceOffering(b.serviceOffering.ID, logger)
	if err != nil {
		return errs(NewGenericError(ctx, err))
	}

	quotasErrors, ok := b.checkQuotas(ctx, plan, cfPlanCounts, b.serviceOffering.ID, logger)
	if !ok {
		return errs(quotasErrors)
	}

	if err := b.checkPlanSchemas(ctx, requestParams, plan, logger); err != nil {
		return errs(err)
	}

	var boshContextID string

	if plan.LifecycleErrands != nil {
		boshContextID = uuid.New()
	}

	requestContext := requestParams["context"]
	var spaceGUID string
	if requestContext != nil {
		spaceGUID = getSpaceGUIDFromContext(requestContext.(map[string]interface{}))
	}

	// ToDo set client secret if provided
	serviceInstanceClient, err := b.uaaClient.CreateClient(instanceID, "", instanceName, spaceGUID)
	if err != nil {
		return errs(NewGenericError(ctx, err))
	}

	boshTaskID, manifest, err := b.deployer.Create(
		deploymentName(instanceID),
		plan.ID,
		requestParams,
		boshContextID,
		serviceInstanceClient,
		logger,
	)
	switch err := err.(type) {
	case boshdirector.RequestError:
		return errs(NewBoshRequestError("create", err))
	case DisplayableError:
		return errs(err)
	case serviceadapter.UnknownFailureError:
		return errs(adapterToAPIError(ctx, err))
	case error:
		return errs(NewGenericError(ctx, err))
	}

	ctx = brokercontext.WithBoshTaskID(ctx, boshTaskID)

	abridgedPlan := plan.AdapterPlan(b.serviceOffering.GlobalProperties)

	dashboardUrl, err := b.adapterClient.GenerateDashboardUrl(instanceID, abridgedPlan, manifest, logger)
	if err != nil {
		logger.Printf("generating dashboard: %v\n", err)
	}

	operationData := OperationData{
		BoshTaskID:    boshTaskID,
		OperationType: OperationTypeCreate,
		BoshContextID: boshContextID,
		Errands:       plan.PostDeployErrands(),
	}

	//Dashboard url optional
	if _, ok := err.(serviceadapter.NotImplementedError); ok {
		return operationData, dashboardUrl, nil
	}

	if err := adapterToAPIError(ctx, err); err != nil {
		return operationData, dashboardUrl, err
	}

	_, err = b.uaaClient.UpdateClient(instanceID, dashboardUrl, spaceGUID)
	if err != nil {
		return errs(NewGenericError(ctx, err))
	}

	return operationData, dashboardUrl, nil
}

func (b *Broker) checkPlanSchemas(ctx context.Context, requestParams map[string]interface{}, plan config.Plan, logger *log.Logger) error {
	if b.EnablePlanSchemas {
		var schemas domain.ServiceSchemas
		schemas, err := b.adapterClient.GeneratePlanSchema(plan.AdapterPlan(b.serviceOffering.GlobalProperties), logger)
		if err != nil {
			if _, ok := err.(serviceadapter.NotImplementedError); !ok {
				return err
			}
			logger.Println("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas")
			return fmt.Errorf("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas")
		}

		instanceProvisionSchema := schemas.Instance.Create

		validator := NewValidator(instanceProvisionSchema.Parameters)
		err = validator.ValidateSchema()
		if err != nil {
			return err
		}

		paramsToValidate, ok := requestParams["parameters"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("provision request params are malformed: %s", requestParams["parameters"])
		}

		err = validator.ValidateParams(paramsToValidate)
		if err != nil {
			failureResp := apiresponses.NewFailureResponseBuilder(err, http.StatusBadRequest, "params-validation-failed").Build()
			return failureResp
		}
	}

	return nil
}
