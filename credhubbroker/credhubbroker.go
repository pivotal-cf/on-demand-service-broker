package credhubbroker

import (
	"context"
	"fmt"

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

	key := fmt.Sprintf("/c/%s/%s/%s/credentials", details.ServiceID, instanceID, bindingID)
	logger.Printf("storing credentials for instance ID: %s, with binding ID: %s", instanceID, bindingID)
	err = b.credStore.Set(key, binding.Credentials)
	if err != nil {
		ctx = brokercontext.New(ctx, string(broker.OperationTypeBind), requestID, b.serviceName, instanceID)
		err = (broker.NewGenericError(ctx, fmt.Errorf("failed to set credentials in credential store: %v", err)))
		logger.Print(err)
		return brokerapi.Binding{}, err
	}

	return binding, nil
}
