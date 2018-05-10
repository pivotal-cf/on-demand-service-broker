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
