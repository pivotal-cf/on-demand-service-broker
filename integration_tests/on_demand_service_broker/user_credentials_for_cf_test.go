// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
	"github.com/pivotal-cf/on-demand-service-broker/mockuaa"
)

var _ = Describe("UAA user credentials for CF", func() {
	const (
		cfClientID     = "cf"
		cfClientSecret = ""
		cfUsername     = "some-cf-admin"
		cfPassword     = "some-cf-admin-password"
	)

	var (
		runningBroker *gexec.Session
		cfAPI         *mockhttp.Server
		cfUAA         *mockuaa.UserCredentialsServer
		boshDirector  *mockbosh.MockBOSH
		boshUAA       *mockuaa.ClientCredentialsServer
	)

	BeforeEach(func() {
		boshUAA = mockuaa.NewClientCredentialsServerTLS(boshClientID, boshClientSecret, pathToSSLCerts("cert.pem"), pathToSSLCerts("key.pem"), "bosh uaa token")
		boshDirector = mockbosh.NewWithUAA(boshUAA.URL)
		cfAPI = mockcfapi.New()
		cfAPI.VerifyAndMock(
			mockcfapi.GetInfo().RespondsWithSufficientAPIVersion(),
			mockcfapi.ListServiceOfferings().RespondsWithNoServiceOfferings(),
		)
		boshDirector.VerifyAndMock(
			mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
			mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
			mockbosh.Info().RespondsWithSufficientStemcellVersionForODB(boshDirector.UAAURL),
		)

		cfUAA = mockuaa.NewUserCredentialsServer(cfClientID, cfClientSecret, cfUsername, cfPassword, "CF UAA token")

		conf := defaultBrokerConfig(boshDirector.URL, boshUAA.URL, cfAPI.URL, cfUAA.URL)
		conf.CF.UAA = config.UAAConfig{
			URL: cfUAA.URL,
			Authentication: config.UAACredentials{
				UserCredentials: config.UserCredentials{
					Username: cfUsername,
					Password: cfPassword,
				},
			},
		}

		runningBroker = startBroker(conf)
	})

	AfterEach(func() {
		if runningBroker != nil {
			Eventually(runningBroker.Terminate()).Should(gexec.Exit())
		}
		boshDirector.VerifyMocks()
		cfAPI.VerifyMocks()
		cfAPI.Close()
		cfUAA.Close()
		boshUAA.Close()
		boshDirector.Close()
	})

	It("obtains a token from the UAA", func() {
		Eventually(runningBroker.Terminate()).Should(gexec.Exit())
		Expect(cfUAA.TokensIssued).To(Equal(1))
	})
})
