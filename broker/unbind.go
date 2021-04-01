// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"context"
	"fmt"

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi/v8/domain"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
)

func (b *Broker) Unbind(
	ctx context.Context,
	instanceID,
	bindingID string,
	details domain.UnbindDetails,
	asyncAllowed bool,
) (domain.UnbindSpec, error) {

	b.bindLock.Lock()
	defer b.bindLock.Unlock()

	emptyUnbindSpec := domain.UnbindSpec{}
	requestID := uuid.New()
	if len(brokercontext.GetReqID(ctx)) > 0 {
		requestID = brokercontext.GetReqID(ctx)
	}

	ctx = brokercontext.New(ctx, string(OperationTypeUnbind), requestID, b.serviceOffering.Name, instanceID)
	logger := b.loggerFactory.NewWithContext(ctx)

	manifest, vms, deploymentErr := b.getDeploymentInfo(instanceID, ctx, "unbind", logger)
	if deploymentErr != nil {
		return emptyUnbindSpec, b.processError(deploymentErr, logger)
	}

	requestParams := map[string]interface{}{
		"plan_id":    details.PlanID,
		"service_id": details.ServiceID,
	}

	deploymentVariables, err := b.boshClient.Variables(deploymentName(instanceID), logger)
	if err != nil {
		logger.Printf("failed to retrieve deployment variables for deployment '%s': %s", deploymentName(instanceID), err)
	}

	secretsMap, err := b.secretManager.ResolveManifestSecrets(manifest, deploymentVariables, logger)
	if err != nil {
		logger.Printf("failed to resolve manifest secrets: %s", err.Error())
	}

	plan, found := b.serviceOffering.FindPlanByID(details.PlanID)
	if !found {
		return emptyUnbindSpec, b.processError(NewDisplayableError(
			fmt.Errorf("plan %q not found", details.PlanID),
			fmt.Errorf("finding plan ID %q", details.PlanID),
		), logger)
	}

	dnsAddresses, err := b.boshClient.GetDNSAddresses(deploymentName(instanceID), plan.BindingWithDNS)
	if err != nil {
		return emptyUnbindSpec, b.processError(NewGenericError(ctx, fmt.Errorf("failed to get required DNS info: %s", err)), logger)
	}

	logger.Printf("service adapter will delete binding with ID %s for instance %s\n", bindingID, instanceID)
	err = b.adapterClient.DeleteBinding(bindingID, vms, manifest, requestParams, secretsMap, dnsAddresses, logger)

	if err != nil {
		logger.Printf("delete binding: %v\n", err)
	}

	if err := adapterToAPIError(ctx, err); err != nil {
		return emptyUnbindSpec, b.processError(err, logger)
	}

	return emptyUnbindSpec, nil
}
