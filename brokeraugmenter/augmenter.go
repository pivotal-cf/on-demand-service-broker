package brokeraugmenter

import (
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

func New(conf config.Config, baseBroker apiserver.CombinedBroker, credhubFactory credhubbroker.CredentialStoreFactory, loggerFactory *loggerfactory.LoggerFactory) (apiserver.CombinedBroker, error) {
	if !conf.HasCredHub() {
		return baseBroker, nil
	}

	credhubStore, err := credhubFactory.New()
	if err != nil {
		return nil, err
	}

	return credhubbroker.New(baseBroker, credhubStore, conf.ServiceCatalog.Name, loggerFactory), nil
}
