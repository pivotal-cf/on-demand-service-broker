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
	UpdateServiceBroker(guid, name, username, password, url string) error
}

const executionError = "failed to execute register-broker"

func (r *RegisterBrokerRunner) Run() error {
	existingBrokers, err := r.CFClient.ServiceBrokers()
	if err != nil {
		return errors.Wrap(err, executionError)
	}

	if err := r.createOrUpdateBroker(existingBrokers); err != nil {
		return errors.Wrap(err, executionError)
	}

	return nil
}

func (r *RegisterBrokerRunner) createOrUpdateBroker(existingBrokers []cf.ServiceBroker) error {
	if brokerGUID, found := r.findBroker(existingBrokers); found {
		return r.CFClient.UpdateServiceBroker(brokerGUID, r.Config.BrokerName, r.Config.BrokerUsername, r.Config.BrokerPassword, r.Config.BrokerURL)
	}
	return r.CFClient.CreateServiceBroker(r.Config.BrokerName, r.Config.BrokerUsername, r.Config.BrokerPassword, r.Config.BrokerURL)
}

func (r *RegisterBrokerRunner) findBroker(brokers []cf.ServiceBroker) (string, bool) {
	for _, broker := range brokers {
		if broker.Name == r.Config.BrokerName {
			return broker.GUID, true
		}
	}
	return "", false
}
