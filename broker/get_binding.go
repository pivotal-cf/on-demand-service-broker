package broker

import (
	"context"
	"errors"

	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/pivotal-cf/brokerapi/domain/apiresponses"
)

func (b *Broker) GetBinding(ctx context.Context, instanceID, bindingID string) (domain.GetBindingSpec, error) {
	err := apiresponses.NewFailureResponse(errors.New("GetBinding Not Implemented"), 404, "")
	return domain.GetBindingSpec{}, err
}
