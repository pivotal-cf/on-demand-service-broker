package broker

import (
	"context"
	"errors"

	"code.cloudfoundry.org/brokerapi/v13/domain"
	"code.cloudfoundry.org/brokerapi/v13/domain/apiresponses"
)

func (b *Broker) GetInstance(ctx context.Context, instanceID string, instanceDetails domain.FetchInstanceDetails) (domain.GetInstanceDetailsSpec, error) {
	err := apiresponses.NewFailureResponse(errors.New("GetInstance Not Implemented"), 404, "")
	return domain.GetInstanceDetailsSpec{}, err
}
