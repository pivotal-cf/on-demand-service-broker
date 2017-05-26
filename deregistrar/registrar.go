package deregistrar

import (
	"fmt"
	"log"
)

type Deregistrar struct {
	client CloudFoundryClient
	logger *log.Logger
}

//go:generate counterfeiter -o fakes/fake_cloud_foundry_client.go . CloudFoundryClient
type CloudFoundryClient interface {
	GetServiceOfferingGUID(string, *log.Logger) (string, error)
	DeregisterBroker(string, *log.Logger) error
}

func New(client CloudFoundryClient, logger *log.Logger) *Deregistrar {
	return &Deregistrar{
		client: client,
		logger: logger,
	}
}

func (r *Deregistrar) Deregister(brokerName string) error {
	var brokerGUID string

	brokerGUID, err := r.client.GetServiceOfferingGUID(brokerName, r.logger)
	if err != nil {
		return err
	}

	err = r.client.DeregisterBroker(brokerGUID, r.logger)
	if err != nil {
		return fmt.Errorf("Failed to deregister broker with %s with guid %s, err: %s", brokerName, brokerGUID, err.Error())
	}
	return nil
}
