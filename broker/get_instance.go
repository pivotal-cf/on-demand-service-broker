package broker

import (
	"context"
	"errors"

	"github.com/pivotal-cf/brokerapi/v12/domain"
	"github.com/pivotal-cf/brokerapi/v12/domain/apiresponses"
)

func (b *Broker) GetInstance(ctx context.Context, instanceID string, instanceDetails domain.FetchInstanceDetails) (domain.GetInstanceDetailsSpec, error) {
	err := apiresponses.NewFailureResponse(errors.New("GetInstance Not Implemented"), 404, "")
	return domain.GetInstanceDetailsSpec{}, err
}
