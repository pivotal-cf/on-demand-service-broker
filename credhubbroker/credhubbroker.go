package credhubbroker

import (
	"context"
	"fmt"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
)

type CredHubBroker struct {
	apiserver.CombinedBroker
	credStore CredentialStore
	// loggerFactory *loggerfactory.LoggerFactory
}

func New(broker apiserver.CombinedBroker, credStore CredentialStore, loggerFactory *loggerfactory.Loggerfactory) *CredHubBroker {
	return &CredHubBroker{
		CombinedBroker: broker,
		credStore:      credStore,
		// loggerFactory:  loggerFactory,
	}
}

func (b *CredHubBroker) Bind(ctx context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	// requestID := uuid.New()
	// ctx = brokercontext.New(ctx, string(OperationTypeBind), requestID, b.serviceOffering.Name, instanceID)
	// logger := b.loggerFactory.NewWithContext(ctx)

	binding, err := b.CombinedBroker.Bind(ctx, instanceID, bindingID, details)
	if err != nil {
		return brokerapi.Binding{}, err
	}
	key := fmt.Sprintf("/c/%s/%s/%s/credentials", details.ServiceID, instanceID, bindingID)
	err = b.credStore.Set(key, binding.Credentials)
	if err != nil {
		return brokerapi.Binding{}, err
	}
	return binding, nil
}
