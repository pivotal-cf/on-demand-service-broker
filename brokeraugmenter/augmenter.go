package brokeraugmenter

import (
	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
)

func New(conf config.Config, baseBroker apiserver.CombinedBroker, loggerFactory *loggerfactory.LoggerFactory) (apiserver.CombinedBroker, error) {
	if !conf.HasCredHub() {
		return baseBroker, nil
	}

	credhubStore, err := credhubbroker.NewCredHubStore(
		conf.CredHub.APIURL,
		credhub.AuthURL(conf.CF.Authentication.URL),
		credhub.Auth(auth.UaaClientCredentials(conf.CredHub.ClientID, conf.CredHub.ClientSecret)),
		credhub.SkipTLSValidation(true),
	)
	if err != nil {
		return nil, err
	}

	return credhubbroker.New(baseBroker, credhubStore, loggerFactory), nil
}
