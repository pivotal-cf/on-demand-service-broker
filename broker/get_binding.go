package broker

import (
	"context"
	"errors"

	"github.com/pivotal-cf/brokerapi"
)

func (b *Broker) GetBinding(ctx context.Context, instanceID, bindingID string) (brokerapi.GetBindingSpec, error) {
	err := brokerapi.NewFailureResponse(errors.New("GetBinding Not Implemented"), 404, "")
	return brokerapi.GetBindingSpec{}, err
}
