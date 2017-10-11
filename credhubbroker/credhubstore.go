package credhubbroker

import "github.com/cloudfoundry-incubator/credhub-cli/credhub"

type CredHubStore struct {
	APIURL       string
	ClientID     string
	ClientSecret string
}

func (credHubStore *CredHubStore) Set(key string, value interface{}) error {
	credhub, err := credhub.New(credHubStore.APIURL)
	if err != nil {
		return err
	}
	_, err = credhub.SetCredential(key, "json", value, false)
	return err
}
