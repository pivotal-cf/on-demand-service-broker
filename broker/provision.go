// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"net/http"

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
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
		return brokerapi.ProvisionedServiceSpec{}, b.processError(brokerapi.ErrAsyncRequired, logger)
	}

	detailsWithRawParameters := brokerapi.DetailsWithRawParameters(details)
	requestParams, err := convertDetailsToMap(detailsWithRawParameters)
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, b.processError(err, logger)
	}

	_, err = b.boshClient.GetInfo(logger)
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, b.processError(NewBoshRequestError("create", err), logger)
	}

	operationData, dashboardURL, err := b.provisionInstance(
		ctx,
		instanceID,
		details.PlanID,
		requestParams,
		logger,
	)

	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, b.processError(err, logger)
	}

	operationDataJSON, err := json.Marshal(operationData)
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, b.processError(err, logger)
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

	cfPlanCounts, err := b.cfClient.CountInstancesOfServiceOffering(b.serviceOffering.ID, logger)
	if err != nil {
		return errs(NewGenericError(ctx, err))
	}

	planCounts := b.getAllPlanCounts(ctx, cfPlanCounts, logger)

	if b.serviceOffering.GlobalQuotas.ServiceInstanceLimit != nil {
		err = b.checkGlobalQuota(ctx, planCounts, logger)
		if err != nil {
			return errs(err)
		}
	}
	if plan.Quotas.ServiceInstanceLimit != nil {
		limit := *plan.Quotas.ServiceInstanceLimit
		count, ok := planCounts[planID]
		if ok && count >= limit {
			return errs(NewDisplayableError(
				brokerapi.ErrPlanQuotaExceeded,
				fmt.Errorf("plan quota exceeded for plan ID %s", planID),
			))
		}
	}

	var quotasErrors []error

	if err := b.checkGlobalResourceQuotaNotExceeded(ctx, plan, planCounts, logger); err != nil {
		quotasErrors = append(quotasErrors, err)
	}

	if err := b.checkPlanResourceQuotaNotExceeded(ctx, plan, planCounts, logger); err != nil {
		quotasErrors = append(quotasErrors, err)
	}

	if len(quotasErrors) > 0 {
		errorStrings := []string{}
		for _, e := range quotasErrors {
			errorStrings = append(errorStrings, e.Error())
		}
		return errs(errors.New(strings.Join(errorStrings, ", ")))
	}

	if err := b.checkPlanSchemas(ctx, requestParams, plan, logger); err != nil {
		return errs(err)
	}

	var boshContextID string

	if plan.LifecycleErrands != nil {
		boshContextID = uuid.New()
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
		Errands:       plan.PostDeployErrands(),
	}

	//Dashboard url optional
	if _, ok := err.(serviceadapter.NotImplementedError); ok {
		return operationData, dashboardUrl, nil
	}

	if err := adapterToAPIError(ctx, err); err != nil {
		return operationData, dashboardUrl, err
	}

	return operationData, dashboardUrl, nil
}

func (b *Broker) getAllPlanCounts(ctx context.Context, cfPlanCounts map[cf.ServicePlan]int, logger *log.Logger) map[string]int {
	var brokerPlanCounts = make(map[string]int)

	for plan, count := range cfPlanCounts {
		id := plan.ServicePlanEntity.UniqueID
		brokerPlanCounts[id] = count
	}

	return brokerPlanCounts
}

func (b *Broker) checkPlanSchemas(ctx context.Context, requestParams map[string]interface{}, plan config.Plan, logger *log.Logger) error {
	if b.EnablePlanSchemas {
		var schemas brokerapi.ServiceSchemas
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
			failureResp := brokerapi.NewFailureResponseBuilder(err, http.StatusBadRequest, "params-validation-failed").WithEmptyResponse().Build()
			return failureResp
		}
	}

	return nil
}

func (b *Broker) checkGlobalQuota(
	ctx context.Context,
	planCounts map[string]int,
	logger *log.Logger,
) error {
	var totalServiceInstances = 0
	for _, count := range planCounts {
		totalServiceInstances += count
	}

	if b.serviceOffering.GlobalQuotas.ServiceInstanceLimit != nil && totalServiceInstances >= *b.serviceOffering.GlobalQuotas.ServiceInstanceLimit {
		return NewDisplayableError(
			brokerapi.ErrServiceQuotaExceeded,
			fmt.Errorf("service quota exceeded for service ID %s", b.serviceOffering.ID),
		)
	}

	return nil
}

type exceededQuota struct {
	name     string
	limit    int
	usage    int
	required int
}

func (b *Broker) checkGlobalResourceQuotaNotExceeded(ctx context.Context, plan config.Plan, planCounts map[string]int, logger *log.Logger) error {
	if b.serviceOffering.GlobalQuotas.ResourceLimits == nil {
		return nil
	}

	var exceededQuotas []exceededQuota

	for kind, limit := range b.serviceOffering.GlobalQuotas.ResourceLimits {
		var currentUsage int

		for _, p := range b.serviceOffering.Plans {
			instanceCount := planCounts[plan.ID]
			cost, ok := p.ResourceCosts[kind]
			if ok {
				currentUsage += cost * instanceCount
			}
		}
		required := plan.ResourceCosts[kind]
		if (currentUsage + required) > limit {
			exceededQuotas = append(exceededQuotas, exceededQuota{kind, limit, currentUsage, required})
		}
	}

	if exceededQuotas == nil {
		return nil
	}

	errorDetails := []string{}
	for _, q := range exceededQuotas {
		errorDetails = append(errorDetails, fmt.Sprintf("%s: (limit %d, used %d, requires %d)", q.name, q.limit, q.usage, q.required))
	}

	return fmt.Errorf("global quotas [%s] would be exceeded by this deployment", strings.Join(errorDetails, ", "))
}

func (b *Broker) checkPlanResourceQuotaNotExceeded(ctx context.Context, plan config.Plan, planCounts map[string]int, logger *log.Logger) error {
	if plan.Quotas.ResourceLimits == nil {
		return nil
	}

	var exceededQuotas []exceededQuota

	for kind, limit := range plan.Quotas.ResourceLimits {
		var currentUsage int

		instanceCount := planCounts[plan.ID]
		cost, ok := plan.ResourceCosts[kind]
		if ok {
			currentUsage += cost * instanceCount
		}

		if (currentUsage + cost) > limit {
			exceededQuotas = append(exceededQuotas, exceededQuota{kind, limit, currentUsage, cost})
		}
	}

	if exceededQuotas == nil {
		return nil
	}

	errorDetails := []string{}
	for _, q := range exceededQuotas {
		errorDetails = append(errorDetails, fmt.Sprintf("%s: (limit %d, used %d, requires %d)", q.name, q.limit, q.usage, q.required))
	}

	return fmt.Errorf("plan quotas [%s] would be exceeded by this deployment", strings.Join(errorDetails, ", "))
}
