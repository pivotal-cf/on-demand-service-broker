package broker

import (
	"context"
	"errors"

	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/pivotal-cf/brokerapi/domain/apiresponses"
)

func (b *Broker) GetInstance(ctx context.Context, instanceID string) (domain.GetInstanceDetailsSpec, error) {
	err := apiresponses.NewFailureResponse(errors.New("GetInstance Not Implemented"), 404, "")
	return domain.GetInstanceDetailsSpec{}, err
}
