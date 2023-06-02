package broker

import (
	"context"
	"errors"

	"github.com/pivotal-cf/brokerapi/v10/domain"
	"github.com/pivotal-cf/brokerapi/v10/domain/apiresponses"
)

func (b *Broker) LastBindingOperation(ctx context.Context, instanceID, bindingID string, details domain.PollDetails) (domain.LastOperation, error) {
	err := apiresponses.NewFailureResponse(errors.New("LastBindingOperation Not Implemented"), 404, "")
	return domain.LastOperation{}, err
}
