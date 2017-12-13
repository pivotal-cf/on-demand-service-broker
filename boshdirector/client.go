// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector

import (
	"log"
	"time"

	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pkg/errors"

	"github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
)

type Client struct {
	url string

	PollingInterval time.Duration
	BoshInfo        Info

	director Director
}

//go:generate counterfeiter -o fakes/fake_director.go . Director
type Director interface {
	director.Director
}

//go:generate counterfeiter -o fakes/fake_deployment.go . BOSHDeployment
type BOSHDeployment interface {
	director.Deployment
}

//go:generate counterfeiter -o fakes/fake_task.go . Task
type Task interface {
	director.Task
}

//go:generate counterfeiter -o fakes/fake_uaa.go . UAA
type UAA interface {
	boshuaa.UAA
}

//go:generate counterfeiter -o fakes/fake_director_factory.go . DirectorFactory
type DirectorFactory interface {
	New(config director.FactoryConfig, taskReporter director.TaskReporter, fileReporter director.FileReporter) (director.Director, error)
}

//go:generate counterfeiter -o fakes/fake_uaa_factory.go . UAAFactory
type UAAFactory interface {
	New(config boshuaa.Config) (boshuaa.UAA, error)
}

//go:generate counterfeiter -o fakes/fake_cert_appender.go . CertAppender
type CertAppender interface {
	AppendCertsFromPEM(pemCerts []byte) (ok bool)
}

func NewBOSHClient(director director.Director) *Client {
	return &Client{
		director: director,
	}
}

func New(url string, trustedCertPEM []byte, certAppender CertAppender, directorFactory DirectorFactory, uaaFactory UAAFactory, boshAuth config.BOSHAuthentication, logger *log.Logger) (*Client, error) {
	certAppender.AppendCertsFromPEM(trustedCertPEM)

	directorConfig, err := director.NewConfigFromURL(url)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build director config from url")
	}
	directorConfig.CACert = string(trustedCertPEM)

	unauthenticatedDirector, err := directorFactory.New(directorConfig, director.NewNoopTaskReporter(), director.NewNoopFileReporter())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build unauthenticated director client")
	}

	noAuthClient := &Client{url: url, director: unauthenticatedDirector}
	boshInfo, err := noAuthClient.GetInfo(logger)
	if err != nil {
		return nil, errors.Wrap(err, "error fetching BOSH director information")
	}

	if boshAuth.UAA.IsSet() {
		uaa, err := buildUAA(boshInfo.UserAuthentication.Options.URL, boshAuth, directorConfig.CACert, uaaFactory)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to build UAA client")
		}

		directorConfig.TokenFunc = boshuaa.NewClientTokenSession(uaa).TokenFunc
	} else {
		directorConfig.Client = boshAuth.Basic.Username
		directorConfig.ClientSecret = boshAuth.Basic.Password
	}

	authenticatedDirector, err := directorFactory.New(directorConfig, director.NewNoopTaskReporter(), director.NewNoopFileReporter())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build authenticated director client")
	}

	return &Client{
		director:        authenticatedDirector,
		PollingInterval: 5,
		url:             url,
		BoshInfo:        boshInfo,
	}, nil
}

func (c *Client) VerifyAuth(logger *log.Logger) error {
	isAuthenticated, err := c.director.IsAuthenticated()
	if err != nil {
		return errors.Wrap(err, "Failed to verify credentials")
	}
	if isAuthenticated {
		return nil
	}
	return errors.New("not authenticated")
}

func buildUAA(UAAURL string, boshAuth config.BOSHAuthentication, CACert string, factory UAAFactory) (UAA, error) {
	uaaConfig, err := boshuaa.NewConfigFromURL(UAAURL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build UAA config from url")
	}
	uaaConfig.Client = boshAuth.UAA.ID
	uaaConfig.ClientSecret = boshAuth.UAA.Secret
	uaaConfig.CACert = CACert
	return factory.New(uaaConfig)
}

type Info struct {
	Version            string
	UserAuthentication UserAuthentication `json:"user_authentication"`
}

type UserAuthentication struct {
	Options AuthenticationOptions
}

type AuthenticationOptions struct {
	URL string
}

type Deployment struct {
	Name string
}

const (
	StemcellDirectorVersionType = VersionType("stemcell")
	SemverDirectorVersionType   = VersionType("semver")
)

type VersionType string

type Version struct {
	MajorVersion int
	VersionType  VersionType
}

type DeploymentNotFoundError struct {
	error
}

type RequestError struct {
	error
}

func NewRequestError(e error) RequestError {
	return RequestError{e}
}
