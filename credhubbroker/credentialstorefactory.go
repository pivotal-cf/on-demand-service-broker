package credhubbroker

import (
	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

//go:generate counterfeiter -o fakes/credentialstorefactory.go . CredentialStoreFactory
type CredentialStoreFactory interface {
	New() (CredentialStore, error)
}

type CredhubFactory struct {
	Conf config.Config
}

func (factory CredhubFactory) New() (CredentialStore, error) {
	return NewCredHubStore(
		factory.Conf.CredHub.APIURL,
		credhub.CaCerts(factory.Conf.CredHub.CaCert, factory.Conf.CredHub.InternalUAACaCert),
		credhub.Auth(auth.UaaClientCredentials(factory.Conf.CredHub.ClientID, factory.Conf.CredHub.ClientSecret)),
	)
}
