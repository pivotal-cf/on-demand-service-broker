package credhubbroker

import (
	"errors"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials/values"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/permissions"
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

func (c *CredHubStore) Set(key string, value interface{}) error {
	var err error
	switch credValue := value.(type) {
	case map[string]interface{}:
		_, err = c.credhubClient.SetJSON(key, values.JSON(credValue), credhub.Mode("no-overwrite"))
	case string:
		_, err = c.credhubClient.SetValue(key, values.Value(credValue), credhub.Mode("no-overwrite"))
	default:
		return errors.New("Unknown credential type")
	}
	return err
}

func (c *CredHubStore) AddPermissions(name string, permissions []permissions.Permission) ([]permissions.Permission, error) {
	return c.credhubClient.AddPermissions(name, permissions)
}

func (c *CredHubStore) Delete(key string) error {
	return c.credhubClient.Delete(key)
}

func (c *CredHubStore) Authenticate() error {
	oauth, ok := c.credhubClient.Auth.(*auth.OAuthStrategy)
	if !ok {
		return errors.New("Invalid UAA configuration")
	}

	return oauth.Login()
}
