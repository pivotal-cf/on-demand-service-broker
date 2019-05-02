package registrar

import (
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pkg/errors"
)

type RegisterBrokerRunner struct {
	Config   config.RegisterBrokerErrandConfig
	CFClient RegisterBrokerCFClient
	Logger   *log.Logger
}

//go:generate counterfeiter -o fakes/register_broker_cf_client.go . RegisterBrokerCFClient
type RegisterBrokerCFClient interface {
	ServiceBrokers() ([]cf.ServiceBroker, error)
	CreateServiceBroker(name, username, password, url string) error
}

func (r *RegisterBrokerRunner) Run() error {
	existingBrokers, err := r.CFClient.ServiceBrokers()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve list of service brokers")
	}

	if !r.brokerExistsIn(existingBrokers) {
		err := r.CFClient.CreateServiceBroker(r.Config.BrokerName, r.Config.BrokerUsername, r.Config.BrokerPassword, r.Config.BrokerURL)
		if err != nil {
			return errors.Wrap(err, "failed to create service broker")
		}
	}

	return nil
}

func (r *RegisterBrokerRunner) brokerExistsIn(brokers []cf.ServiceBroker) bool {
	for _, broker := range brokers {
		if broker.Name == r.Config.BrokerName {
			return true
		}
	}
	return false
}
