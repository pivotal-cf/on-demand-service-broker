package broker

import (
	"context"
	"errors"

	"code.cloudfoundry.org/brokerapi/v13/domain"
	"code.cloudfoundry.org/brokerapi/v13/domain/apiresponses"
)

func (b *Broker) GetBinding(ctx context.Context, instanceID, bindingID string, bindingsDetails domain.FetchBindingDetails) (domain.GetBindingSpec, error) {
	err := apiresponses.NewFailureResponse(errors.New("GetBinding Not Implemented"), 404, "")
	return domain.GetBindingSpec{}, err
}
