package deregistrar

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
)

type Deregistrar struct {
	client CloudFoundryClient
	logger *log.Logger
}

//go:generate counterfeiter -o fakes/fake_cloud_foundry_client.go . CloudFoundryClient
type CloudFoundryClient interface {
	ListServiceBrokers(*log.Logger) ([]cf.ServiceBroker, error)
	DeregisterBroker(string, 	*log.Logger) error
}

func New(client CloudFoundryClient, logger *log.Logger) *Deregistrar {
	return &Deregistrar{
		client: client,
		logger: logger,
	}
}

func (r *Deregistrar) Deregister(brokerName string) error {
	var brokerGUID string

	brokers, err := r.client.ListServiceBrokers(r.logger)
	if err != nil {
		return err
	}

	for _, broker := range brokers {
		if broker.Name == brokerName {
			brokerGUID = broker.GUID
		}
	}

	if brokerGUID == "" {
		return fmt.Errorf("Failed to find broker with name: %s", brokerName)
	}

	err = r.client.DeregisterBroker(brokerGUID, r.logger)
	if err != nil {
		return fmt.Errorf("Failed to deregister broker with %s with guid %s, err: %s", brokerName, brokerGUID, err.Error())
	}
	return nil
}
