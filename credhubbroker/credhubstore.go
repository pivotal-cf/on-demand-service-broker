package credhubbroker

import (
	"errors"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials/values"
)

type CredHubStore struct {
	credhubClient *credhub.CredHub
}

func NewCredHubStore(APIURL string, options ...credhub.Option) (*CredHubStore, error) {
	credhubClient, err := credhub.New(APIURL, options...)

	if err != nil {
		return &CredHubStore{}, err
	}

	credhubStore := &CredHubStore{credhubClient}
	return credhubStore, nil
}

func (credHubStore *CredHubStore) Set(key string, value interface{}) error {
	var err error
	switch credValue := value.(type) {
	case map[string]interface{}:
		_, err = credHubStore.credhubClient.SetJSON(key, values.JSON(credValue), credhub.Mode("no-overwrite"))
	case string:
		_, err = credHubStore.credhubClient.SetValue(key, values.Value(credValue), credhub.Mode("no-overwrite"))
	default:
		return errors.New("Unknown credential type")
	}
	return err
}

func (credHubStore *CredHubStore) Delete(key string) error {
	return credHubStore.credhubClient.Delete(key)
}

func (credhubStore *CredHubStore) Authenticate() error {
	oauth, ok := credhubStore.credhubClient.Auth.(*auth.OAuthStrategy)
	if !ok {
		return errors.New("Invalid UAA configuration")
	}

	return oauth.Login()
}
