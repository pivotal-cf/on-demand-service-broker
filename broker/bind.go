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
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
)

func (b *Broker) Bind(
	ctx context.Context,
	instanceID,
	bindingID string,
	details brokerapi.BindDetails,
) (brokerapi.Binding, error) {
	requestID := uuid.New()
	if len(brokercontext.GetReqID(ctx)) > 0 {
		requestID = brokercontext.GetReqID(ctx)
	}

	ctx = brokercontext.New(ctx, string(OperationTypeBind), requestID, b.serviceOffering.Name, instanceID)
	logger := b.loggerFactory.NewWithContext(ctx)

	errs := func(err BrokerError) (brokerapi.Binding, error) {
		logger.Println(err)
		return brokerapi.Binding{}, err.ErrorForCFUser()
	}

	manifest, vms, deploymentErr := b.getDeploymentInfo(instanceID, ctx, "bind", logger)
	if deploymentErr != nil {
		return errs(deploymentErr)
	}

	logger.Printf("service adapter will create binding with ID %s for instance %s\n", bindingID, instanceID)
	detailsWithRawParameters := brokerapi.DetailsWithRawParameters(details)
	mappedParams, err := convertDetailsToMap(detailsWithRawParameters)
	if err != nil {
		return errs(NewGenericError(ctx, fmt.Errorf("converting to map %s", err)))
	}

	var createBindingErr error
	binding, createBindingErr := b.adapterClient.CreateBinding(bindingID, vms, manifest, mappedParams, logger)
	if createBindingErr != nil {
		logger.Printf("creating binding: %v\n", createBindingErr)
	}

	if err := adapterToAPIError(ctx, createBindingErr); err != nil {
		return brokerapi.Binding{}, err
	}

	return brokerapi.Binding{
		Credentials:     binding.Credentials,
		SyslogDrainURL:  binding.SyslogDrainURL,
		RouteServiceURL: binding.RouteServiceURL,
	}, nil
}
