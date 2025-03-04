package broker

import (
	"context"
	"errors"

	"code.cloudfoundry.org/brokerapi/v13/domain"
	"code.cloudfoundry.org/brokerapi/v13/domain/apiresponses"
)

func (b *Broker) LastBindingOperation(ctx context.Context, instanceID, bindingID string, details domain.PollDetails) (domain.LastOperation, error) {
	err := apiresponses.NewFailureResponse(errors.New("LastBindingOperation Not Implemented"), 404, "")
	return domain.LastOperation{}, err
}
