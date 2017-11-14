package brokeraugmenter

import (
	"strings"
	"time"

	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

func New(conf config.Config,
	baseBroker apiserver.CombinedBroker,
	credhubFactory credhubbroker.CredentialStoreFactory,
	loggerFactory *loggerfactory.LoggerFactory) (apiserver.CombinedBroker, error) {

	if !conf.HasCredHub() {
		return baseBroker, nil
	}

	var credhubStore credhubbroker.CredentialStore
	var err error

	waitMillis := 16
	retryLimit := 10

	// if consul hasn't started yet, wait until internal DNS begins to work
	for retries := 0; retries < retryLimit; retries++ {
		credhubStore, err = credhubFactory.New()
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "no such host") {
			time.Sleep(time.Duration(waitMillis) * time.Millisecond)
			waitMillis *= 2
		} else {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	return credhubbroker.New(baseBroker, credhubStore, conf.ServiceCatalog.Name, loggerFactory), nil
}
