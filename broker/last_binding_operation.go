package broker

import (
	"context"
	"errors"

	"github.com/pivotal-cf/brokerapi"
)

func (b *Broker) LastBindingOperation(ctx context.Context, instanceID, bindingID string, details brokerapi.PollDetails) (brokerapi.LastOperation, error) {
	err := brokerapi.NewFailureResponse(errors.New("LastBindingOperation Not Implemented"), 404, "")
	return brokerapi.LastOperation{}, err
}
