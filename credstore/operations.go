package credstore

import (
	"encoding/json"
	"errors"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials"
)

type Operations struct {
	getter CredhubGetter
}

//go:generate counterfeiter -o fakes/fake_credhub_getter.go . CredhubGetter

type CredhubGetter interface {
	GetLatestVersion(name string) (credentials.Credential, error)
}

func New(getter CredhubGetter) *Operations {
	return &Operations{
		getter: getter,
	}
}

func (b *Operations) BulkGet(secretsToFetch [][]byte) (map[string]string, error) {
	ret := map[string]string{}
	for _, credhubPath := range secretsToFetch {
		path := string(credhubPath)
		c, err := b.getter.GetLatestVersion(path)
		if err != nil {
			return nil, err
		}
		switch credValue := c.Value.(type) {
		case string:
			ret[path] = credValue
		default:
			credValueJSON, err := json.Marshal(credValue)
			if err != nil {
				return nil, errors.New("failed to marshal secret: " + err.Error())
			}
			ret[path] = string(credValueJSON)
		}
	}
	return ret, nil
}
