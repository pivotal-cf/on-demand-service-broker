package credhubbroker

import (
	"context"
	"fmt"

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

type CredHubBroker struct {
	apiserver.CombinedBroker
	credStore     CredentialStore
	loggerFactory *loggerfactory.LoggerFactory
}

func New(broker apiserver.CombinedBroker, credStore CredentialStore, loggerFactory *loggerfactory.LoggerFactory) *CredHubBroker {
	return &CredHubBroker{
		CombinedBroker: broker,
		credStore:      credStore,
		loggerFactory:  loggerFactory,
	}
}

func (b *CredHubBroker) Bind(ctx context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	ctx = brokercontext.WithReqID(ctx, uuid.New())
	logger := b.loggerFactory.NewWithContext(ctx)

	binding, err := b.CombinedBroker.Bind(ctx, instanceID, bindingID, details)
	if err != nil {
		return brokerapi.Binding{}, err
	}

	key := fmt.Sprintf("/c/%s/%s/%s/credentials", details.ServiceID, instanceID, bindingID)
	logger.Printf("storing credentials for instance ID: %s, with binding ID: %s", instanceID, bindingID)
	err = b.credStore.Set(key, binding.Credentials)
	if err != nil {
		logger.Printf("failed to set credentials in credential store for instance ID: %s, with binding ID: %s", instanceID, bindingID)
		return brokerapi.Binding{}, err
	}

	return binding, nil
}
