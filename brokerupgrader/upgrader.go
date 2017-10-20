package brokerupgrader

import (
	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/pivotal-cf/on-demand-service-broker/apiserver"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/credhubbroker"
)

func New(conf config.Config, baseBroker apiserver.CombinedBrokers) (apiserver.CombinedBrokers, error) {
	if !conf.HasCredHub() {
		return baseBroker, nil
	}

	credhubStore, err := credhubbroker.NewCredHubStore(
		conf.CredHub.APIURL,
		credhub.AuthURL(conf.CF.Authentication.URL),
		credhub.Auth(auth.UaaClientCredentials(conf.CredHub.ClientID, conf.CredHub.ClientSecret)),
	)
	if err != nil {
		return nil, err
	}

	return credhubbroker.New(baseBroker, credhubStore), nil
}
