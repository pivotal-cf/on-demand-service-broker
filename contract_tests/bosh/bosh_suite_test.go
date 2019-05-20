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

package bosh_test

import (
	"crypto/x509"
	"fmt"
	"log"
	"os"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshlinks"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"

	"testing"
)

func TestBosh(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bosh Suite")
}

func envMustHave(key string) string {
	value := os.Getenv(key)
	Expect(value).ToNot(BeEmpty(), fmt.Sprintf("must set %s", key))
	return value
}

var (
	c              *boshdirector.Client
	logger         *log.Logger
	stdout         *gbytes.Buffer
	boshAuthConfig config.Authentication
)

var _ = BeforeSuite(func() {
	c = NewBOSHClient()
	uploadDummyRelease(c)
})

func NewBOSHClient() *boshdirector.Client {
	certPEM := []byte(envMustHave("BOSH_CA_CERT"))
	var err error

	stdout = gbytes.NewBuffer()

	factory := boshdir.NewFactory(boshlog.NewLogger(boshlog.LevelError))
	uaaFactory := boshuaa.NewFactory(boshlog.NewLogger(boshlog.LevelError))

	certPool, err := x509.SystemCertPool()
	Expect(err).NotTo(HaveOccurred())

	boshAuthConfig = config.Authentication{
		UAA: config.UAAAuthentication{
			ClientCredentials: config.ClientCredentials{
				ID:     envMustHave("BOSH_CLIENT"),
				Secret: envMustHave("BOSH_CLIENT_SECRET"),
			},
		},
	}

	loggerFactory := loggerfactory.New(stdout, "", loggerfactory.Flags)
	logger = loggerFactory.New()

	c, err = boshdirector.New(
		envMustHave("BOSH_ENVIRONMENT"),
		certPEM,
		certPool,
		factory,
		uaaFactory,
		boshAuthConfig,
		boshlinks.NewDNSRetriever,
		boshdirector.NewBoshHTTP,
		logger,
	)
	Expect(err).NotTo(HaveOccurred())
	c.PollingInterval = 0
	return c
}

func NewBOSHClientWithBadCredentials() *boshdirector.Client {
	certPEM := []byte(envMustHave("BOSH_CA_CERT"))
	var err error

	stdout = gbytes.NewBuffer()

	factory := boshdir.NewFactory(boshlog.NewLogger(boshlog.LevelError))
	uaaFactory := boshuaa.NewFactory(boshlog.NewLogger(boshlog.LevelError))

	certPool, err := x509.SystemCertPool()
	Expect(err).NotTo(HaveOccurred())

	boshAuthConfig = config.Authentication{
		UAA: config.UAAAuthentication{
			ClientCredentials: config.ClientCredentials{
				ID:     "foo",
				Secret: "bar",
			},
		},
	}

	loggerFactory := loggerfactory.New(stdout, "", loggerfactory.Flags)
	logger = loggerFactory.New()

	c, err = boshdirector.New(
		envMustHave("BOSH_ENVIRONMENT"),
		certPEM,
		certPool,
		factory,
		uaaFactory,
		boshAuthConfig,
		boshlinks.NewDNSRetriever,
		boshdirector.NewBoshHTTP,
		logger,
	)
	Expect(err).NotTo(HaveOccurred())
	c.PollingInterval = 0
	return c
}

func uploadDummyRelease(c *boshdirector.Client) {
	taskReporter := boshdirector.NewAsyncTaskReporter()
	d, err := c.Director(taskReporter)
	Expect(err).ToNot(HaveOccurred())
	err = d.UploadReleaseURL(
		envMustHave("DUMMY_RELEASE_URL"),
		envMustHave("DUMMY_RELEASE_SHA"),
		true,
		false,
	)
	Expect(err).ToNot(HaveOccurred())
	Eventually(taskReporter.Finished).Should(Receive(), "Timed out uploading dummy-release")
}
