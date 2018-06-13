package credstore

import (
	"encoding/json"
	"errors"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

type Operations struct {
	getter CredhubGetter
}

//go:generate counterfeiter -o fakes/fake_credhub_getter.go . CredhubGetter

type CredhubGetter interface {
	GetLatestVersion(name string) (credentials.Credential, error)
	GetById(id string) (credentials.Credential, error)
}

func New(getter CredhubGetter) *Operations {
	return &Operations{
		getter: getter,
	}
}

func (b *Operations) BulkGet(secretsToFetch map[string]boshdirector.Variable) (map[string]string, error) {
	ret := map[string]string{}
	for name, deploymentVar := range secretsToFetch {
		var c credentials.Credential
		var err error
		if deploymentVar.ID != "" {
			c, err = b.getter.GetById(deploymentVar.ID)
		} else {
			c, err = b.getter.GetLatestVersion(deploymentVar.Path)
		}
		if err != nil {
			continue
		}
		switch credValue := c.Value.(type) {
		case string:
			ret[name] = credValue
		default:
			credValueJSON, err := json.Marshal(credValue)
			if err != nil {
				return nil, errors.New("failed to marshal secret: " + err.Error())
			}
			ret[name] = string(credValueJSON)
		}
	}
	return ret, nil
}
