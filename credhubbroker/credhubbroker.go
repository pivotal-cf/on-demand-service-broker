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

package credhubbroker

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub/permissions"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

type CredHubBroker struct {
	apiserver.CombinedBroker
	credStore     CredentialStore
	serviceName   string
	loggerFactory *loggerfactory.LoggerFactory
}

func New(broker apiserver.CombinedBroker,
	credStore CredentialStore,
	serviceName string,
	loggerFactory *loggerfactory.LoggerFactory,
) *CredHubBroker {

	return &CredHubBroker{
		CombinedBroker: broker,
		credStore:      credStore,
		serviceName:    serviceName,
		loggerFactory:  loggerFactory,
	}
}

func (b *CredHubBroker) Bind(ctx context.Context, instanceID, bindingID string, details brokerapi.BindDetails, asyncAllowed bool) (brokerapi.Binding, error) {
	var actor string
	switch {
	case details.AppGUID != "":
		actor = fmt.Sprintf("mtls-app:%s", details.AppGUID)
	case details.BindResource != nil && details.BindResource.AppGuid != "":
		actor = fmt.Sprintf("mtls-app:%s", details.BindResource.AppGuid)
	case details.BindResource != nil && details.BindResource.CredentialClientID != "":
		actor = fmt.Sprintf("uaa-client:%s", details.BindResource.CredentialClientID)
	default:
		return brokerapi.Binding{}, errors.New("No app-guid or credential client ID were provided in the binding request, you must configure one of these")
	}

	requestID := uuid.New()
	ctx = brokercontext.WithReqID(ctx, requestID)
	logger := b.loggerFactory.NewWithContext(ctx)

	binding, err := b.CombinedBroker.Bind(ctx, instanceID, bindingID, details, asyncAllowed)
	if err != nil {
		return brokerapi.Binding{}, err
	}

	key := constructKey(details.ServiceID, instanceID, bindingID)
	logger.Printf("storing credentials for instance ID: %s, with binding ID: %s", instanceID, bindingID)
	err = b.credStore.Set(key, binding.Credentials)
	if err != nil {
		ctx = brokercontext.New(ctx, string(broker.OperationTypeBind), requestID, b.serviceName, instanceID)
		setErr := broker.NewGenericError(ctx, fmt.Errorf("failed to set credentials in credential store: %v", err))
		logger.Print(setErr)
		return brokerapi.Binding{}, setErr.ErrorForCFUser()
	}

	additionalPermissions := []permissions.Permission{
		{
			Actor:      actor,
			Operations: []string{"read"},
		},
	}
	b.credStore.AddPermissions(key, additionalPermissions)

	binding.Credentials = map[string]string{"credhub-ref": key}
	return binding, nil
}

func (b *CredHubBroker) Unbind(ctx context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails, asyncAllowed bool) (brokerapi.UnbindSpec, error) {
	requestID := uuid.New()
	ctx = brokercontext.WithReqID(ctx, requestID)
	logger := b.loggerFactory.NewWithContext(ctx)

	logger.Printf("removing credentials for instance ID: %s, with binding ID: %s\n", instanceID, bindingID)
	unbind, err := b.CombinedBroker.Unbind(ctx, instanceID, bindingID, details, asyncAllowed)
	if err != nil {
		return brokerapi.UnbindSpec{}, err
	}

	key := constructKey(details.ServiceID, instanceID, bindingID)
	chErr := b.credStore.Delete(key)
	if chErr != nil {
		logger.Printf("WARNING: failed to remove key '%s' from credential store", key)
	}

	return unbind, nil
}

func constructKey(serviceID, instanceID, bindingID string) string {
	return fmt.Sprintf("/c/%s/%s/%s/credentials", serviceID, instanceID, bindingID)
}
