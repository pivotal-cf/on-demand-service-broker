// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

func (b *Broker) Provision(ctx context.Context, instanceID string, details brokerapi.ProvisionDetails,
	asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, error) {

	b.deploymentLock.Lock()
	defer b.deploymentLock.Unlock()

	requestID := uuid.New()
	ctx = brokercontext.New(ctx, string(OperationTypeCreate), requestID, b.serviceOffering.Name, instanceID)
	logger := b.loggerFactory.NewWithContext(ctx)

	if !asyncAllowed {
		return brokerapi.ProvisionedServiceSpec{}, brokerapi.ErrAsyncRequired
	}

	detailsWithRawParameters := brokerapi.DetailsWithRawParameters(details)
	requestParams, err := convertDetailsToMap(detailsWithRawParameters)
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	operationData, dashboardURL, err := b.provisionInstance(
		ctx,
		instanceID,
		details.PlanID,
		requestParams,
		logger,
	)

	switch anotherErr := err.(type) {
	case DisplayableError:
		if anotherErr.Occurred() {
			logger.Println(anotherErr)
			return brokerapi.ProvisionedServiceSpec{}, anotherErr.ErrorForCFUser()
		}
	case error:
		return brokerapi.ProvisionedServiceSpec{}, anotherErr
	}

	operationDataJSON, err := json.Marshal(operationData)
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	return brokerapi.ProvisionedServiceSpec{
		IsAsync:       true,
		DashboardURL:  dashboardURL,
		OperationData: string(operationDataJSON),
	}, nil
}

func (b *Broker) provisionInstance(ctx context.Context, instanceID string, planID string,
	requestParams map[string]interface{}, logger *log.Logger) (OperationData, string, error) {

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
			brokerapi.ErrInstanceAlreadyExists,
			fmt.Errorf("deploying instance %s", instanceID),
		))
	}

	var planCounts map[string]int
	if b.serviceOffering.GlobalQuotas.ServiceInstanceLimit != nil {
		var displayableError DisplayableError
		planCounts, displayableError = b.checkGlobalQuota(ctx, b.serviceOffering.ID, logger)
		if displayableError.Occurred() {
			return errs(displayableError)
		}
	}

	if plan.Quotas.ServiceInstanceLimit != nil {
		limit := *plan.Quotas.ServiceInstanceLimit
		planCount, displayableError := b.getPlanCount(ctx, planID, planCounts, logger)
		if displayableError.Occurred() {
			return errs(displayableError)
		}

		if planCount >= limit {
			return errs(NewDisplayableError(
				brokerapi.ErrPlanQuotaExceeded,
				fmt.Errorf("plan quota exceeded for plan ID %s", planID),
			))
		}
	}

	var boshContextID string
	var operationPostDeployErrand string
	var operationPostDeployErrandInstances []string

	if plan.LifecycleErrands != nil {
		boshContextID = uuid.New()
	}

	if plan.PostDeployErrand() != "" {
		operationPostDeployErrand = plan.PostDeployErrand()
		operationPostDeployErrandInstances = plan.PostDeployErrandInstances()
	}

	boshTaskID, manifest, err := b.deployer.Create(deploymentName(instanceID), plan.ID, requestParams, boshContextID, logger)
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
		PostDeployErrand: PostDeployErrand{
			Name:      operationPostDeployErrand,
			Instances: operationPostDeployErrandInstances,
		},
		PreDeleteErrand: PreDeleteErrand{
			Name:      plan.PreDeleteErrand(),
			Instances: plan.PreDeleteErrandInstances(),
		},
	}

	//Dashboard url optional
	if _, ok := err.(serviceadapter.NotImplementedError); ok {
		return operationData, dashboardUrl, nil
	}

	if err := adapterToAPIError(ctx, err); err != nil {
		return operationData, dashboardUrl, err
	}

	return operationData, dashboardUrl, DisplayableError{}
}

func (b *Broker) getPlanCount(ctx context.Context, planID string, planCounts map[string]int, logger *log.Logger) (int, DisplayableError) {
	var planCount int

	if planCounts != nil {
		planCount = planCounts[planID]
	} else {
		var countErr error
		planCount, countErr = b.cfClient.CountInstancesOfPlan(b.serviceOffering.ID, planID, logger)
		if countErr != nil {
			return 0, NewGenericError(ctx, fmt.Errorf("could not count instances of plan: %s", countErr))
		}
	}

	return planCount, DisplayableError{}
}

func (b *Broker) checkGlobalQuota(
	ctx context.Context,
	serviceOfferingID string,
	logger *log.Logger,
) (map[string]int, DisplayableError) {

	planCounts, err := b.cfClient.CountInstancesOfServiceOffering(serviceOfferingID, logger)
	if err != nil {
		return nil, NewGenericError(ctx, err)
	}

	var totalServiceInstances = 0
	for _, count := range planCounts {
		totalServiceInstances += count
	}

	if b.serviceOffering.GlobalQuotas.ServiceInstanceLimit != nil && totalServiceInstances >= *b.serviceOffering.GlobalQuotas.ServiceInstanceLimit {
		return nil, NewDisplayableError(
			brokerapi.ErrServiceQuotaExceeded,
			fmt.Errorf("service quota exceeded for service ID %s", b.serviceOffering.ID),
		)
	}

	return nil, DisplayableError{}
}
