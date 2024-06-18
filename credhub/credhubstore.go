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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials/values"
	"code.cloudfoundry.org/credhub-cli/credhub/permissions"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
)

type Store struct {
	credhubClient CredhubClient
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate -o fakes/fake_credhub_client.go . CredhubClient
type CredhubClient interface {
	GetById(id string) (credentials.Credential, error)
	GetLatestVersion(name string) (credentials.Credential, error)
	FindByPartialName(partialName string) (credentials.FindResults, error)
	SetJSON(name string, value values.JSON, options ...credhub.SetOption) (credentials.JSON, error)
	SetValue(name string, value values.Value, options ...credhub.SetOption) (credentials.Value, error)
	AddPermission(credName, actor string, ops []string) (*permissions.Permission, error)
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

func (c *Store) Set(key string, value interface{}) error {
	var err error
	switch credValue := value.(type) {
	case map[string]interface{}:
		_, err = c.credhubClient.SetJSON(key, values.JSON(credValue))
	case string:
		_, err = c.credhubClient.SetValue(key, values.Value(credValue))
	default:
		return errors.New("Unknown credential type")
	}
	return err
}

func (c *Store) AddPermission(credName, actor string, ops []string) (*permissions.Permission, error) {
	return c.credhubClient.AddPermission(credName, actor, ops)
}

func (c *Store) BulkDelete(paths []string, logger *log.Logger) error {
	for _, path := range paths {
		if err := c.Delete(path); err != nil {
			logger.Printf("could not delete secret '%s': %s", path, err.Error())
			return err
		}
	}
	return nil
}

func (c *Store) FindNameLike(name string, logger *log.Logger) ([]string, error) {
	results, err := c.credhubClient.FindByPartialName(name)
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, result := range results.Credentials {
		paths = append(paths, result.Name)
	}

	return paths, nil
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

		var keyValue string

		namePieces := strings.Split(strings.Trim(name, "()"), ".")
		if len(namePieces) == 2 {
			keyValue, err = getSubKey(cred.Value, namePieces[1])
		} else {
			keyValue, err = getKey(cred.Value)
		}

		if err != nil {
			logger.Println(err.Error())
			continue
		}
		ret[name] = keyValue

	}
	return ret, nil
}

func getKey(requestedValue interface{}) (string, error) {
	switch credValue := requestedValue.(type) {
	case string:
		return credValue, nil
	case map[string]interface{}:
		// this will catch structured types: certificate, user, rsa, ssh
		credValueJSON, err := json.Marshal(credValue)
		if err != nil {
			return "", errors.New("failed to marshal secret: " + err.Error())
		}
		return string(credValueJSON), nil

	default:
		return "", fmt.Errorf("unexpected datatype received from credhub %T", credValue)
	}
}

func getSubKey(credValue interface{}, subkey string) (string, error) {
	var requestedValue string

	switch credValue := credValue.(type) {
	case map[string]interface{}:
		var jsonPart interface{}
		var ok bool
		if jsonPart, ok = credValue[subkey]; !ok {
			return "", fmt.Errorf("credential does not contain key '%s'", subkey)
		}

		if jsonStr, ok := jsonPart.(string); ok {
			requestedValue = jsonStr
			break
		}

		partBytes, err := json.Marshal(jsonPart)
		if err != nil {
			return "", err
		}

		requestedValue = string(partBytes)

	case string:
		return "", fmt.Errorf("string type credential cannot have key '%s'", subkey)

	default:
		return "", fmt.Errorf("unknown credential type")
	}

	return requestedValue, nil
}

func (c *Store) BulkSet(secretsToSet []broker.ManifestSecret) error {
	for _, secret := range secretsToSet {
		if err := c.Set(secret.Path, secret.Value); err != nil {
			return err
		}
	}
	return nil
}
