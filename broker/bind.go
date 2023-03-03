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
	"net/http"

	"github.com/pivotal-cf/brokerapi/v9/domain/apiresponses"

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi/v9/domain"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

func (b *Broker) Bind(
	ctx context.Context,
	instanceID,
	bindingID string,
	details domain.BindDetails,
	asyncAllowed bool,
) (domain.Binding, error) {
	b.bindLock.Lock()
	defer b.bindLock.Unlock()

	requestID := uuid.New()
	if len(brokercontext.GetReqID(ctx)) > 0 {
		requestID = brokercontext.GetReqID(ctx)
	}

	ctx = brokercontext.New(ctx, string(OperationTypeBind), requestID, b.serviceOffering.Name, instanceID)
	logger := b.loggerFactory.NewWithContext(ctx)

	if details.BindResource.BackupAgent && !b.SupportBackupAgentBinding {
		return domain.Binding{}, b.processError(apiresponses.NewFailureResponse(
			errors.New("service does not support backup agent binding"),
			http.StatusUnprocessableEntity,
			"unsupported-binding-type",
		), logger)
	}

	manifest, vms, deploymentErr := b.getDeploymentInfo(instanceID, ctx, "bind", logger)
	if deploymentErr != nil {
		return domain.Binding{}, b.processError(deploymentErr, logger)
	}

	deploymentVariables, err := b.boshClient.Variables(deploymentName(instanceID), logger)
	if err != nil {
		logger.Printf("failed to retrieve deployment variables for deployment '%s': %s", deploymentName(instanceID), err)
	}

	secretsMap, err := b.secretManager.ResolveManifestSecrets(manifest, deploymentVariables, logger)
	if err != nil {
		logger.Printf("failed to resolve manifest secrets: %s", err.Error())
	}

	logger.Printf("service adapter will create binding with ID %s for instance %s\n", bindingID, instanceID)

	detailsWithRawParameters := domain.DetailsWithRawParameters(details)

	mappedParams, err := convertDetailsToMap(detailsWithRawParameters)
	if err != nil {
		return domain.Binding{}, b.processError(NewGenericError(ctx, fmt.Errorf("converting to map %s", err)), logger)
	}

	plan, planFound := b.serviceOffering.FindPlanByID(details.PlanID)

	if b.EnablePlanSchemas {
		if !planFound {
			return domain.Binding{}, b.processError(NewDisplayableError(
				fmt.Errorf("plan %s not found", details.PlanID),
				fmt.Errorf("finding plan ID %s", details.PlanID),
			), logger)
		}

		schemas, err := b.adapterClient.GeneratePlanSchema(plan.AdapterPlan(b.serviceOffering.GlobalProperties), logger)
		if err != nil {
			if _, ok := err.(serviceadapter.NotImplementedError); !ok {
				return domain.Binding{}, b.processError(err, logger)
			}

			logger.Println("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas")
			return domain.Binding{}, b.processError(fmt.Errorf("enable_plan_schemas is set to true, but the service adapter does not implement generate-plan-schemas"), logger)
		}

		bindingCreateSchema := schemas.Binding.Create
		validator := NewValidator(bindingCreateSchema.Parameters)

		params, ok := mappedParams["parameters"].(map[string]interface{})
		if !ok {
			return domain.Binding{}, b.processError(NewGenericError(ctx, errors.New("converting parameters to map failed")), logger)
		}

		err = validator.ValidateParams(params)
		if err != nil {
			failureResp := apiresponses.NewFailureResponseBuilder(err, http.StatusBadRequest, "params-validation-failed").Build()
			return domain.Binding{}, b.processError(failureResp, logger)
		}
	}

	dnsAddresses, err := b.boshClient.GetDNSAddresses(deploymentName(instanceID), plan.BindingWithDNS)
	if err != nil {
		return domain.Binding{}, b.processError(NewGenericError(ctx, fmt.Errorf("failed to get required DNS info: %s", err)), logger)
	}

	binding, createBindingErr := b.adapterClient.CreateBinding(bindingID, vms, manifest, mappedParams, secretsMap, dnsAddresses, logger)
	if createBindingErr != nil {
		if !b.EnableSecureManifests {
			logger.Printf("broker.resolve_secrets_at_bind was: false ")
		}

		logger.Printf("creating binding: %v\n", createBindingErr)
	}

	if err := adapterToAPIError(ctx, createBindingErr); err != nil {
		return domain.Binding{}, b.processError(err, logger)
	}

	return domain.Binding{
		Credentials:     binding.Credentials,
		SyslogDrainURL:  binding.SyslogDrainURL,
		RouteServiceURL: binding.RouteServiceURL,
		BackupAgentURL:  binding.BackupAgentURL,
	}, nil
}
