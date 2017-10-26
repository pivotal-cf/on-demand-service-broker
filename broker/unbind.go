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
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
)

func (b *Broker) Unbind(
	ctx context.Context,
	instanceID,
	bindingID string,
	details brokerapi.UnbindDetails,
) error {

	requestID := uuid.New()
	if len(brokercontext.GetReqID(ctx)) > 0 {
		requestID = brokercontext.GetReqID(ctx)
	}

	ctx = brokercontext.New(ctx, string(OperationTypeUnbind), requestID, b.serviceOffering.Name, instanceID)
	logger := b.loggerFactory.NewWithContext(ctx)

	errs := func(err DisplayableError) error {
		logger.Println(err)
		return err.ErrorForCFUser()
	}

	vms, manifest, err := b.getDeploymentInfo(instanceID, logger)
	switch err.(type) {
	case boshdirector.RequestError:
		return errs(NewBoshRequestError("unbind", fmt.Errorf("could not get deployment info: %s", err)))
	case boshdirector.DeploymentNotFoundError:
		return errs(NewDisplayableError(brokerapi.ErrInstanceDoesNotExist, fmt.Errorf("error unbinding: instance %s, not found", instanceID)))
	case error:
		return errs(NewGenericError(ctx, fmt.Errorf("gathering unbinding info %s", err)))
	}

	requestParams := map[string]interface{}{
		"plan_id":    details.PlanID,
		"service_id": details.ServiceID,
	}

	logger.Printf("service adapter will delete binding with ID %s for instance %s\n", bindingID, instanceID)
	err = b.adapterClient.DeleteBinding(bindingID, vms, manifest, requestParams, logger)

	if err != nil {
		logger.Printf("delete binding: %v\n", err)
	}

	if err := adapterToAPIError(ctx, err); err != nil {
		return err
	}

	return nil
}
