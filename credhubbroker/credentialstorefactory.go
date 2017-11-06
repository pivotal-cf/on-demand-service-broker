package credhubbroker

import (
	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

type CredentialStoreFactory interface {
	New() (CredentialStore, error)
}

type CredhubFactory struct {
	Conf config.Config
}

func (factory CredhubFactory) New() (CredentialStore, error) {
	return NewCredHubStore(
		factory.Conf.CredHub.APIURL,
		credhub.CaCerts(factory.Conf.CredHub.CaCert, factory.Conf.CF.Authentication.CaCert),
		credhub.Auth(auth.UaaClientCredentials(factory.Conf.CredHub.ClientID, factory.Conf.CredHub.ClientSecret)),
	)
}

type DummyCredhubFactory struct {
}

func (factory DummyCredhubFactory) New() (CredentialStore, error) {
	return &CredHubStore{}, nil
}
