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

func (b *CredHubBroker) Bind(ctx context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {

	requestID := uuid.New()
	ctx = brokercontext.WithReqID(ctx, requestID)
	logger := b.loggerFactory.NewWithContext(ctx)

	binding, err := b.CombinedBroker.Bind(ctx, instanceID, bindingID, details)
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

	var actor string
	if details.AppGUID != "" {
		actor = fmt.Sprintf("mtls-app:%s", details.AppGUID)
	} else if details.BindResource != nil && details.BindResource.CredentialClientID != "" {
		actor = fmt.Sprintf("uaa-client:%s", details.BindResource.CredentialClientID)
	}

	if actor == "" {
		return brokerapi.Binding{}, errors.New("No app-guid or credential client ID were provided in the binding request, you must configure one of these")
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

func (b *CredHubBroker) Unbind(ctx context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails) error {

	requestID := uuid.New()
	ctx = brokercontext.WithReqID(ctx, requestID)
	logger := b.loggerFactory.NewWithContext(ctx)

	logger.Printf("removing credentials for instance ID: %s, with binding ID: %s\n", instanceID, bindingID)
	err := b.CombinedBroker.Unbind(ctx, instanceID, bindingID, details)
	if err != nil {
		return err
	}

	key := constructKey(details.ServiceID, instanceID, bindingID)
	chErr := b.credStore.Delete(key)
	if chErr != nil {
		logger.Printf("WARNING: failed to remove key '%s' from credential store", key)
	}

	return nil
}

func constructKey(serviceID, instanceID, bindingID string) string {
	return fmt.Sprintf("/c/%s/%s/%s/credentials", serviceID, instanceID, bindingID)
}
