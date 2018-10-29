package broker

import (
	"context"
	"errors"

	"github.com/pivotal-cf/brokerapi"
)

func (b *Broker) GetInstance(ctx context.Context, instanceID string) (brokerapi.GetInstanceDetailsSpec, error) {
	err := brokerapi.NewFailureResponse(errors.New("GetInstance Not Implemented"), 404, "")
	return brokerapi.GetInstanceDetailsSpec{}, err
}
