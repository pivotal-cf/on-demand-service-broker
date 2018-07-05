// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	"log"
	"testing"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

var (
	c *boshdirector.Client

	fakeCertAppender    *fakes.FakeCertAppender
	fakeDirector        *fakes.FakeDirector
	fakeDirectorFactory *fakes.FakeDirectorFactory
	fakeUAAFactory      *fakes.FakeUAAFactory
	fakeUAA             *fakes.FakeUAA
	fakeBoshHTTPFactory *fakes.FakeHTTPFactory
	logger              *log.Logger
	boshAuthConfig      config.Authentication
)

var _ = BeforeEach(func() {
	fakeCertAppender = new(fakes.FakeCertAppender)
	fakeDirectorFactory = new(fakes.FakeDirectorFactory)
	fakeUAAFactory = new(fakes.FakeUAAFactory)
	fakeUAA = new(fakes.FakeUAA)
	fakeDirector = new(fakes.FakeDirector)
	fakeBoshHTTPFactory = new(fakes.FakeHTTPFactory)
	boshAuthConfig = config.Authentication{
		UAA: config.UAAAuthentication{
			ClientCredentials: config.ClientCredentials{
				ID:     "bosh-user",
				Secret: "bosh-secret",
			},
		},
	}
	logger = log.New(GinkgoWriter, "[boshdirector unit test]", log.LstdFlags)

	fakeDirectorFactory.NewReturns(fakeDirector, nil)
	fakeUAAFactory.NewReturns(fakeUAA, nil)
	fakeDirector.InfoReturns(boshdir.Info{
		Version: "1.3262.0.0 (00000000)",
		Auth: boshdir.UserAuthentication{
			Type: "uaa",
			Options: map[string]interface{}{
				"url": "https://this-is-the-uaa-url.example.com",
			},
		},
	}, nil)
})

var _ = JustBeforeEach(func() {
	var certPEM []byte

	var err error

	c, err = boshdirector.New(
		"https://director.example.com",
		certPEM,
		fakeCertAppender,
		fakeDirectorFactory,
		fakeUAAFactory,
		boshAuthConfig,
		fakeBoshHTTPFactory.Spy,
		logger,
	)
	Expect(err).NotTo(HaveOccurred())
	c.PollingInterval = 0
})

func TestBoshDirector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bosh Director Suite")
}
