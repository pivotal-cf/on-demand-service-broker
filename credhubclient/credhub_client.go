// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package credhubclient

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pivotal-cf/credhub-cli/actions"
	"github.com/pivotal-cf/credhub-cli/client"
	"github.com/pivotal-cf/credhub-cli/commands"
	"github.com/pivotal-cf/credhub-cli/config"
	"github.com/pivotal-cf/credhub-cli/repositories"
)

type Client struct {
	Url                        string
	Id                         string
	Secret                     string
	DisableSSLCertVerification bool
}

var Noop = new(noop)

type noop struct{}

func (n *noop) PutCredentials(id string, creds map[string]interface{}) error { return nil }

func NewCredhubClient(url, id, secret string, disableSSLCertVertification bool) *Client {
	return &Client{
		Url:    url,
		Id:     id,
		Secret: secret,
		DisableSSLCertVerification: disableSSLCertVertification,
	}
}

func (c *Client) PutCredentials(identifier string, credentialsMap map[string]interface{}) error {
	rawCredentials, err := json.Marshal(credentialsMap)
	if err != nil {
		return fmt.Errorf("error marshalling credentials")
	}

	cfg := config.Config{}
	cfg.InsecureSkipVerify = c.DisableSSLCertVerification
	commands.GetApiInfo(&cfg, c.Url, false)

	httpClient := client.NewHttpClient(cfg)
	repository := repositories.NewSecretRepository(httpClient)

	cfg.AccessToken, cfg.RefreshToken, err = c.getCredhubTokens(cfg, httpClient)
	if err != nil {
		return fmt.Errorf("error getting credhub auth token")
	}

	req := client.NewPutPasswordRequest(cfg, identifier, string(rawCredentials), false)

	action := actions.NewAction(repository, cfg)
	_, err = action.DoAction(req, identifier)
	if err != nil {
		return fmt.Errorf("error putting password into credhub: %s", err)
	}

	return nil
}

func (c *Client) getCredhubTokens(cfg config.Config, httpClient *http.Client) (string, string, error) {
	token, err := actions.NewAuthToken(httpClient, cfg).GetAuthToken(c.Id, c.Secret)
	if err != nil {
		return "", "", err
	}
	return token.AccessToken, token.RefreshToken, nil
}
