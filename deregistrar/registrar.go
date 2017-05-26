package deregistrar

import (
	"fmt"
	"log"
)

type Deregistrar struct {
	cfClient CloudFoundryClient
	logger   *log.Logger
}

//go:generate counterfeiter -o fakes/fake_cloud_foundry_client.go . CloudFoundryClient
type CloudFoundryClient interface {
	GetServiceOfferingGUID(string, *log.Logger) (string, error)
	DeregisterBroker(string, *log.Logger) error
}

func New(client CloudFoundryClient, logger *log.Logger) *Deregistrar {
	return &Deregistrar{
		cfClient: client,
		logger:   logger,
	}
}

func (r *Deregistrar) Deregister(brokerName string) error {
	var brokerGUID string

	brokerGUID, err := r.cfClient.GetServiceOfferingGUID(brokerName, r.logger)
	if err != nil {
		return err
	}

	err = r.cfClient.DeregisterBroker(brokerGUID, r.logger)
	if err != nil {
		return fmt.Errorf("Failed to deregister broker with %s with guid %s, err: %s", brokerName, brokerGUID, err.Error())
	}
	return nil
}
