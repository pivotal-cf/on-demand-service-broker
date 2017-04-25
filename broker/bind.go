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
	"github.com/pivotal-cf/on-demand-service-broker/boshclient"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
)

func (b *Broker) Bind(
	ctx context.Context,
	instanceID,
	bindingID string,
	details brokerapi.BindDetails,
) (brokerapi.Binding, error) {
	requestID := uuid.New()
	ctx = brokercontext.New(ctx, string(OperationTypeBind), requestID, b.serviceOffering.Name, instanceID)
	logger := b.loggerFactory.NewWithContext(ctx)

	errs := func(err DisplayableError) (brokerapi.Binding, error) {
		logger.Println(err)
		return brokerapi.Binding{}, err.ErrorForCFUser()
	}

	vms, manifest, err := b.getDeploymentInfo(instanceID, logger)
	switch err.(type) {
	case boshclient.RequestError:
		return errs(NewBoshRequestError("bind", fmt.Errorf("could not get deployment info: %s", err)))
	case boshclient.DeploymentNotFoundError:
		return errs(NewDisplayableError(brokerapi.ErrInstanceDoesNotExist, fmt.Errorf("error binding: instance %s, not found", instanceID)))
	case error:
		return errs(
			NewGenericError(ctx, fmt.Errorf("gathering binding info %s", err)),
		)
	}

	logger.Printf("service adapter will create binding with ID %s for instance %s\n", bindingID, instanceID)
	detailsWithRawParameters := brokerapi.DetailsWithRawParameters(details)
	mappedParams, err := convertDetailsToMap(detailsWithRawParameters)
	if err != nil {
		return errs(NewGenericError(ctx, fmt.Errorf("converting to map %s", err)))
	}

	binding, err := b.adapterClient.CreateBinding(bindingID, vms, manifest, mappedParams, logger)
	if err != nil {
		logger.Printf("creating binding: %v\n", err)
	}

	if err := adapterToAPIError(ctx, err); err != nil {
		return brokerapi.Binding{}, err
	}

	credentialId := fmt.Sprintf("%s/%s", instanceID, bindingID)
	if err := b.credentialStore.PutCredentials(credentialId, binding.Credentials); err != nil {
		logger.Printf("Unable to put %s in credhub: %s\n", credentialId, err)
	}

	return brokerapi.Binding{
		Credentials:     binding.Credentials,
		SyslogDrainURL:  binding.SyslogDrainURL,
		RouteServiceURL: binding.RouteServiceURL,
	}, nil
}
