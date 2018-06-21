// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package credhub

import (
	"errors"
	"log"

	"encoding/json"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials/values"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/permissions"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/task"
)

type Store struct {
	credhubClient CredhubClient
}

//go:generate counterfeiter -o fakes/fake_credhub_client.go . CredhubClient
type CredhubClient interface {
	GetById(id string) (credentials.Credential, error)
	GetLatestVersion(name string) (credentials.Credential, error)
	SetJSON(name string, value values.JSON, overwrite credhub.Mode) (credentials.JSON, error)
	SetValue(name string, value values.Value, overwrite credhub.Mode) (credentials.Value, error)
	AddPermissions(credName string, perms []permissions.Permission) ([]permissions.Permission, error)
	Delete(name string) error
}

func Build(APIURL string, options ...credhub.Option) (*Store, error) {
	credhubClient, err := credhub.New(APIURL, options...)

	if err != nil {
		return &Store{}, err
	}

	credhubStore := New(credhubClient)
	return credhubStore, nil
}

func New(credhubClient CredhubClient) *Store {
	return &Store{credhubClient: credhubClient}
}

var noOverwriteMode = credhub.Mode("no-overwrite")

func (c *Store) Set(key string, value interface{}) error {
	var err error
	switch credValue := value.(type) {
	case map[string]interface{}:
		_, err = c.credhubClient.SetJSON(key, values.JSON(credValue), noOverwriteMode)
	case string:
		_, err = c.credhubClient.SetValue(key, values.Value(credValue), noOverwriteMode)
	default:
		return errors.New("Unknown credential type")
	}
	return err
}

func (c *Store) AddPermissions(name string, permissions []permissions.Permission) ([]permissions.Permission, error) {
	return c.credhubClient.AddPermissions(name, permissions)
}

func (c *Store) Delete(key string) error {
	return c.credhubClient.Delete(key)
}

func (c *Store) BulkGet(secretsToFetch map[string]boshdirector.Variable, logger *log.Logger) (map[string]string, error) {
	ret := map[string]string{}
	for name, deploymentVar := range secretsToFetch {
		var cred credentials.Credential
		var err error
		if deploymentVar.ID != "" {
			cred, err = c.credhubClient.GetById(deploymentVar.ID)
		} else {
			cred, err = c.credhubClient.GetLatestVersion(deploymentVar.Path)
		}
		if err != nil {
			logger.Printf("Could not resolve %s: %s", name, err)
			continue
		}
		switch credValue := cred.Value.(type) {
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

func (c *Store) BulkSet(secretsToSet []task.ManifestSecret) error {
	for _, secret := range secretsToSet {
		if err := c.Set(secret.Path, secret.Value); err != nil {
			return err
		}
	}
	return nil
}
