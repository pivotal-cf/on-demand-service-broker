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
	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

//go:generate counterfeiter -o fakes/credentialstorefactory.go . CredentialStoreFactory
type CredentialStoreFactory interface {
	New() (CredentialStore, error)
}

type CredhubFactory struct {
	Conf config.Config
}

func (factory CredhubFactory) New() (CredentialStore, error) {
	return NewCredHubStore(
		factory.Conf.CredHub.APIURL,
		credhub.CaCerts(factory.Conf.CredHub.CaCert, factory.Conf.CredHub.InternalUAACaCert),
		credhub.Auth(auth.UaaClientCredentials(factory.Conf.CredHub.ClientID, factory.Conf.CredHub.ClientSecret)),
	)
}
