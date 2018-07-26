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
	"github.com/coreos/go-semver/semver"
)

type Client struct {
	url string

	PollingInterval time.Duration
	BoshInfo        Info

	trustedCertPEM  []byte
	boshAuth        config.Authentication
	uaaFactory      UAAFactory
	directorFactory DirectorFactory
	dnsRetriever    DNSRetriever
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

//go:generate counterfeiter -o fakes/fake_boshhttp.go . HTTP

type HTTP interface {
	RawGet(path string) (string, error)
	RawPost(path, data, contentType string) (string, error)
	RawDelete(path string) (string, error)
}

//go:generate counterfeiter -o fakes/fake_dns_retriever.go . DNSRetriever

type DNSRetriever interface {
	LinkProviderID(deploymentName, instanceGroupName, providerName string) (string, error)
	CreateLinkConsumer(providerID string) (string, error)
	GetLinkAddress(consumerLinkID string, azs []string, status string) (string, error)
	DeleteLinkConsumer(consumerID string) error
}

//go:generate counterfeiter -o fakes/fake_dns_retriever_factory.go . DNSRetrieverFactory

type DNSRetrieverFactory func(HTTP) DNSRetriever

func New(url string,
	trustedCertPEM []byte,
	certAppender CertAppender,
	directorFactory DirectorFactory,
	uaaFactory UAAFactory,
	boshAuth config.Authentication,
	dnsRetrieverFactory DNSRetrieverFactory,
	boshHTTPFactory HTTPFactory,
	logger *log.Logger) (*Client, error) {

	certAppender.AppendCertsFromPEM(trustedCertPEM)

	noAuthClient := &Client{url: url, trustedCertPEM: trustedCertPEM, directorFactory: directorFactory}
	boshInfo, err := noAuthClient.GetInfo(logger)
	if err != nil {
		return nil, errors.Wrap(err, "error fetching BOSH director information")
	}

	client := &Client{
		trustedCertPEM:  trustedCertPEM,
		boshAuth:        boshAuth,
		uaaFactory:      uaaFactory,
		directorFactory: directorFactory,
		PollingInterval: 5,
		url:             url,
		BoshInfo:        boshInfo,
	}
	client.dnsRetriever = dnsRetrieverFactory(boshHTTPFactory(client))
	return client, nil
}

func (c *Client) Director(taskReporter director.TaskReporter) (director.Director, error) {
	directorConfig, err := c.directorConfig()
	if err != nil {
		return nil, err
	}
	return c.directorFactory.New(directorConfig, taskReporter, director.NewNoopFileReporter())
}

func (c *Client) directorConfig() (director.FactoryConfig, error) {
	directorConfig, err := director.NewConfigFromURL(c.url)
	if err != nil {
		return director.FactoryConfig{}, errors.Wrap(err, "Failed to build director config from url")
	}
	directorConfig.CACert = string(c.trustedCertPEM)

	if c.boshAuth.UAA.IsSet() {
		uaa, err := buildUAA(c.BoshInfo.UserAuthentication.Options.URL, c.boshAuth, directorConfig.CACert, c.uaaFactory)
		if err != nil {
			return director.FactoryConfig{}, errors.Wrap(err, "Failed to build UAA client")
		}

		directorConfig.TokenFunc = boshuaa.NewClientTokenSession(uaa).TokenFunc
	} else {
		directorConfig.Client = c.boshAuth.Basic.Username
		directorConfig.ClientSecret = c.boshAuth.Basic.Password
	}
	return directorConfig, nil
}

func (c *Client) VerifyAuth(logger *log.Logger) error {
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return errors.Wrap(err, " to verify credentials")
	}
	isAuthenticated, err := d.IsAuthenticated()
	if err != nil {
		return errors.Wrap(err, "Failed to verify credentials")
	}
	if isAuthenticated {
		return nil
	}
	return errors.New("not authenticated")
}

func buildUAA(UAAURL string, boshAuth config.Authentication, CACert string, factory UAAFactory) (UAA, error) {
	uaaConfig, err := boshuaa.NewConfigFromURL(UAAURL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build UAA config from url")
	}
	uaaConfig.Client = boshAuth.UAA.ClientCredentials.ID
	uaaConfig.ClientSecret = boshAuth.UAA.ClientCredentials.Secret
	uaaConfig.CACert = CACert
	return factory.New(uaaConfig)
}

type Info struct {
	Version            string
	UserAuthentication UserAuthentication `json:"user_authentication"`
}

type Variable struct {
	Path string `json:"name"`
	ID   string `json:"id"`
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
	Version semver.Version
	Type    VersionType
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
